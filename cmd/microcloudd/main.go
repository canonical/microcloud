// Package microcloudd provides the daemon.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/microcluster/v2/rest"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/service"
	"github.com/canonical/microcloud/microcloud/version"
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

type cmdDaemon struct {
	global *cmdGlobal

	flagMicroCloudDir string
}

func (c *cmdDaemon) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "microcloudd",
		Short:   "MicroCloud daemon",
		Version: version.Version,
	}

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdDaemon) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	addr := util.NetworkInterfaceAddress()
	name, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Failed to retrieve system hostname: %w", err)
	}

	services := []types.ServiceType{types.MicroCloud, types.LXD}
	optionalServices := map[types.ServiceType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	for serviceType, stateDir := range optionalServices {
		if service.Exists(serviceType, stateDir) {
			services = append(services, serviceType)
		} else {
			logger.Infof("Skipping %s service, could not detect state directory", serviceType)
		}
	}

	s, err := service.NewHandler(name, addr, c.flagMicroCloudDir, services...)
	if err != nil {
		return err
	}

	// Periodically check if new services have been installed.
	go func() {
		for {
			for serviceName, stateDir := range optionalServices {
				if service.Exists(serviceName, stateDir) {
					if s.Services[serviceName] != nil {
						continue
					}

					newService, err := service.NewHandler(name, addr, c.flagMicroCloudDir, serviceName)
					if err != nil {
						logger.Error("Failed to create servie handler for service", logger.Ctx{"service": serviceName, "error": err})
						break
					}

					s.Services[serviceName] = newService.Services[serviceName]
				} else if s.Services[serviceName] != nil {
					delete(s.Services, serviceName)
				}
			}

			time.Sleep(1 * time.Second)
		}
	}()

	endpoints := []rest.Endpoint{
		api.ServicesCmd(s),
		api.ServiceTokensCmd(s),
		api.ServicesClusterCmd(s),
		api.SessionJoinCmd(s),
		api.SessionInitiatingCmd(s),
		api.SessionJoiningCmd(s),
		api.LXDProxy(s),
		api.CephProxy(s),
		api.OVNProxy(s),
	}

	return s.Services[types.MicroCloud].(*service.CloudService).StartCloud(context.Background(), s, endpoints, c.global.flagLogVerbose, c.global.flagLogDebug)
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

	app.SetVersionTemplate("{{.Version}}\n")

	err := app.Execute()
	if err != nil {
		os.Exit(1)
	}
}
