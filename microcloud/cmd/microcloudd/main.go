// Package microcloudd provides the daemon.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/lxc/lxd/lxd/util"
	"github.com/lxc/lxd/shared/logger"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/version"
	"github.com/canonical/microcluster/config"
	"github.com/canonical/microcluster/microcluster"
	"github.com/canonical/microcluster/state"

	cloudMDNS "github.com/canonical/microcloud/microcloud/mdns"
)

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

// forwardingWriter forwards the mdns log message to LXD's logger package.
type forwardingWriter struct {
	w io.Writer
}

func (f forwardingWriter) Write(p []byte) (int, error) {
	logMsg := string(p)

	if strings.Contains(logMsg, "[INFO]") {
		_, after, _ := strings.Cut(logMsg, "[INFO]")
		logger.Infof(after)
	} else if strings.Contains(logMsg, "[ERR]") {
		_, after, _ := strings.Cut(logMsg, "[ERR]")
		logger.Errorf(after)
	} else {
		return 0, fmt.Errorf("Invalid log %q", logMsg)
	}

	return len(logMsg), nil
}

func (c *cmdGlobal) Run(cmd *cobra.Command, args []string) {
	log.SetOutput(forwardingWriter{})
}

type cmdDaemon struct {
	global *cmdGlobal

	flagStateDir string
}

func (c *cmdDaemon) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "microcloudd",
		Short:   "MicroCloud daemon",
		Version: version.Version,
	}

	cmd.PersistentPreRun = c.global.Run
	cmd.RunE = c.Run

	return cmd
}

func (c *cmdDaemon) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	m, err := microcluster.App(context.Background(), c.flagStateDir, c.global.flagLogVerbose, c.global.flagLogDebug)
	if err != nil {
		return err
	}

	addr := util.NetworkInterfaceAddress()
	port := 7001
	name, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Failed to retrieve system hostname: %w", err)
	}

	var tokenCancel context.CancelFunc
	var server *mdns.Server
	defer func() {
		if server != nil {
			server.Shutdown()
		}
	}()

	hooks := &config.Hooks{
		// Collect all mDNS records, and create join tokens for each one.
		OnBootstrap: func(s *state.State) error {
			tokenCancel()

			peers, err := cloudMDNS.LookupPeers(s)
			if err != nil {
				return err
			}

			tokensByName := make(map[string]string, len(peers))
			for _, peer := range peers {
				token, err := m.NewJoinToken(peer)
				if err != nil {
					return err
				}

				tokensByName[peer] = token
			}

			err = server.Shutdown()
			if err != nil {
				return fmt.Errorf("Failed to shut down %q server: %w", cloudMDNS.ClusterService, err)
			}

			bytes, err := json.Marshal(tokensByName)
			if err != nil {
				return fmt.Errorf("Failed to marshal list of tokens: %w", err)
			}

			server, err = cloudMDNS.NewBroadcast(cloudMDNS.TokenService, name, addr, port, bytes)
			if err != nil {
				return fmt.Errorf("Failed to begin join token broadcast: %w", err)
			}

			// Shutdown the server after 30 seconds.
			timeAfter := time.After(time.Second * 30)
			go func() {
				for {
					select {
					case <-timeAfter:
						logger.Info("Shutting down broadcast")
						err := server.Shutdown()
						if err != nil {
							logger.Error("Failed to shutdown mDNS server after timeout")
						}

						return
					case <-s.Context.Done():
						return
					default:
						// Sleep a bit so the loop doesn't push the CPU as hard.
						time.Sleep(100 * time.Millisecond)
					}
				}
			}()

			return nil
		},

		// Start a broadcast informing other cluster members that we are willing to join.
		// Simultaneously lookup any join tokens being issued by a server that as initiated the cluster.
		OnStart: func(s *state.State) error {
			var ctx context.Context
			ctx, tokenCancel = context.WithCancel(s.Context)

			var err error
			server, err = cloudMDNS.NewBroadcast(cloudMDNS.ClusterService, name, addr, port, nil)
			if err != nil {
				return err
			}

			cloudMDNS.LookupJoinToken(ctx, name, func(token string) error {
				return m.JoinCluster(name, net.JoinHostPort(addr, strconv.Itoa(port)), token, time.Second*30)
			})

			return nil
		},

		// OnJoin is run after the daemon is initialized and joins a cluster.
		OnJoin: func(s *state.State) error {
			err = server.Shutdown()
			if err != nil {
				return fmt.Errorf("Failed to shutdown mdns server after joining the cluster: %w", err)
			}

			return nil
		},
	}

	return m.Start(api.Endpoints, nil, hooks)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	daemonCmd := cmdDaemon{global: &cmdGlobal{}}
	app := daemonCmd.Command()
	app.SilenceUsage = true
	app.CompletionOptions = cobra.CompletionOptions{DisableDefaultCmd: true}

	app.PersistentFlags().BoolVarP(&daemonCmd.global.flagHelp, "help", "h", false, "Print help")
	app.PersistentFlags().BoolVar(&daemonCmd.global.flagVersion, "version", false, "Print version number")
	app.PersistentFlags().BoolVarP(&daemonCmd.global.flagLogDebug, "debug", "d", false, "Show all debug messages")
	app.PersistentFlags().BoolVarP(&daemonCmd.global.flagLogVerbose, "verbose", "v", false, "Show all information messages")

	app.PersistentFlags().StringVar(&daemonCmd.flagStateDir, "state-dir", "", "Path to store state information"+"``")

	app.SetVersionTemplate("{{.Version}}\n")

	err := app.Execute()
	if err != nil {
		os.Exit(1)
	}
}
