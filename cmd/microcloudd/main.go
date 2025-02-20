// Package microcloudd provides the daemon.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/canonical/lxd/lxd/util"
	lxdAPI "github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/microcluster/v2/microcluster"
	"github.com/canonical/microcluster/v2/rest"
	microTypes "github.com/canonical/microcluster/v2/rest/types"
	"github.com/canonical/microcluster/v2/state"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/component"
	"github.com/canonical/microcloud/microcloud/version"
)

// MinimumHeartbeatInterval is used to prevent a user from setting the MicroCluster
// heartbeat to 0.
const MinimumHeartbeatInterval = time.Millisecond * 200

// Debug indicates whether to log debug messages or not.
var Debug bool

// Verbose indicates verbosity.
var Verbose bool

type cmdGlobal struct {
	cmd *cobra.Command //nolint:structcheck,unused // FIXME: Remove the nolint flag when this is in use.

	flagHelp    bool
	flagVersion bool

	flagLogDebug   bool
	flagLogVerbose bool
}

type cmdDaemon struct {
	global *cmdGlobal

	flagMicroCloudDir     string
	flagHeartbeatInterval time.Duration
}

// Command returns the main microcloudd command.
func (c *cmdDaemon) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "microcloudd",
		Short:   "MicroCloud daemon",
		Version: version.Version(),
	}

	cmd.RunE = c.Run

	return cmd
}

// Run runs the main microcloudd command.
func (c *cmdDaemon) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	if c.flagHeartbeatInterval < MinimumHeartbeatInterval {
		return fmt.Errorf("Invalid heartbeat interval: Must be >%s", MinimumHeartbeatInterval)
	}

	addr := util.NetworkInterfaceAddress()
	name, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Failed to retrieve system hostname: %w", err)
	}

	components := []types.ComponentType{types.MicroCloud, types.LXD}
	optionalComponents := map[types.ComponentType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	for componentType, stateDir := range optionalComponents {
		if component.Exists(componentType, stateDir) {
			components = append(components, componentType)
		} else {
			logger.Infof("Skipping %s component, could not detect state directory", componentType)
		}
	}

	s, err := component.NewHandler(name, addr, c.flagMicroCloudDir, components...)
	if err != nil {
		return err
	}

	// Periodically check if new components have been installed.
	go func() {
		for {
			for componentName, stateDir := range optionalComponents {
				if component.Exists(componentName, stateDir) {
					if s.Components[componentName] != nil {
						continue
					}

					newComponent, err := component.NewHandler(name, addr, c.flagMicroCloudDir, componentName)
					if err != nil {
						logger.Error("Failed to create servie handler for component", logger.Ctx{"component": componentName, "error": err})
						break
					}

					s.Components[componentName] = newComponent.Components[componentName]
				} else if s.Components[componentName] != nil {
					delete(s.Components, componentName)
				}
			}

			time.Sleep(1 * time.Second)
		}
	}()

	endpoints := []rest.Endpoint{
		api.StatusCmd(s),
		api.ComponentCmd(s),
		api.ComponentsTokensCmd(s),
		api.ComponentClusterCmd(s),
		api.SessionJoinCmd(s),
		api.SessionInitiatingCmd(s),
		api.SessionJoiningCmd(s),
		api.SessionStopCmd(s),
		api.LXDProxy(s),
		api.CephProxy(s),
		api.OVNProxy(s),
	}

	setHandlerAddress := func(url string) error {
		addrPort, err := microTypes.ParseAddrPort(url)
		if err != nil {
			return err
		}

		if addrPort != (microTypes.AddrPort{}) {
			s.SetAddress(addrPort.Addr().String())
		}

		return nil
	}

	dargs := microcluster.DaemonArgs{
		Verbose:           c.global.flagLogVerbose,
		Debug:             c.global.flagLogDebug,
		Version:           version.RawVersion,
		HeartbeatInterval: c.flagHeartbeatInterval,

		PreInitListenAddress: "[::]:" + strconv.FormatInt(component.CloudPort, 10),
		Hooks: &state.Hooks{
			PostBootstrap: func(ctx context.Context, state state.State, initConfig map[string]string) error {
				return setHandlerAddress(state.Address().URL.Host)
			},
			PostJoin: func(ctx context.Context, state state.State, cfg map[string]string) error {
				// If the node has joined close the session.
				// This will signal to the client to exit out gracefully
				// and ultimately lead to the closing of the websocket connection.
				// Prevent blocking of the hook by also watching the outer context.
				select {
				case s.Session.ExitCh() <- true:
				case <-ctx.Done():
				}

				return setHandlerAddress(state.Address().URL.Host)
			},
			OnStart: func(ctx context.Context, state state.State) error {
				// If we are already initialized, there's nothing to do.
				err := state.Database().IsOpen(ctx)
				// If we encounter a non-503 error, that means the database failed for some reason.
				if err != nil && !lxdAPI.StatusErrorCheck(err, http.StatusServiceUnavailable) {
					return nil
				}

				databaseReady := err == nil

				// With a 503 error or no error, we can be sure there is an address trying to connect to dqlite, so we can proceed with the handler address update.
				err = setHandlerAddress(state.Address().URL.Host)
				if err != nil {
					return err
				}

				if !databaseReady {
					return nil
				}

				initialized, err := s.Components[types.LXD].IsInitialized(context.Background())
				if err != nil {
					return err
				}

				if !initialized {
					return nil
				}

				// If the MicroCloud database is online, and LXD is initialized, try to set user.microcloud.
				c, err := s.Components[types.LXD].(*component.LXDComponent).Client(context.Background())
				if err != nil {
					return err
				}

				// Don't error out in case there's an issue with LXD and we need to manage it with MicroCloud.
				currentServer, etag, err := c.GetServer()
				if err != nil {
					logger.Error("Failed to retrieve LXD configuration on start", logger.Ctx{"error": err})

					return nil
				}

				newServer := currentServer.Writable()
				val, ok := newServer.Config["user.microcloud"]
				if ok && val == version.RawVersion {
					return nil
				}

				newServer.Config["user.microcloud"] = version.RawVersion

				// Don't error out in case there's an issue with LXD and we need to manage it with MicroCloud.
				err = c.UpdateServer(newServer, etag)
				if err != nil {
					logger.Error("Failed to update LXD configuration on start", logger.Ctx{"error": err})
				}

				return nil
			},
		},

		ExtensionServers: map[string]rest.Server{
			"microcloud": {
				CoreAPI:   true,
				PreInit:   true,
				ServeUnix: true,
				Resources: []rest.Resources{
					{
						PathPrefix: types.APIVersion,
						Endpoints:  endpoints,
					},
				},
			},
		},
	}

	return s.Components[types.MicroCloud].(*component.CloudComponent).StartCloud(context.Background(), dargs)
}

func main() {
	// Only root should run this
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "This must be run as root")
		os.Exit(1)
	}

	daemonCmd := cmdDaemon{global: &cmdGlobal{}}
	app := daemonCmd.Command()
	app.SilenceUsage = true
	app.CompletionOptions = cobra.CompletionOptions{DisableDefaultCmd: true}

	app.PersistentFlags().BoolVarP(&daemonCmd.global.flagHelp, "help", "h", false, "Print help")
	app.PersistentFlags().BoolVar(&daemonCmd.global.flagVersion, "version", false, "Print version number")
	app.PersistentFlags().BoolVarP(&daemonCmd.global.flagLogDebug, "debug", "d", false, "Show all debug messages")
	app.PersistentFlags().BoolVarP(&daemonCmd.global.flagLogVerbose, "verbose", "v", false, "Show all information messages")

	app.PersistentFlags().StringVar(&daemonCmd.flagMicroCloudDir, "state-dir", "", "Path to store state information for MicroCloud"+"``")
	app.PersistentFlags().DurationVar(&daemonCmd.flagHeartbeatInterval, "heartbeat", time.Second*10, "Time between attempted heartbeats")

	app.SetVersionTemplate("{{.Version}}\n")

	err := app.Execute()
	if err != nil {
		os.Exit(1)
	}
}
