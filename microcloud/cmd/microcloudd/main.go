// Package microcloudd provides the daemon.
package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/canonical/microcluster/rest"
	"github.com/lxc/lxd/lxd/util"
	"github.com/lxc/lxd/shared/logger"
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
		if service.ServiceExists(serviceType, stateDir) {
			services = append(services, serviceType)
		} else {
			logger.Infof("Skipping %s service, could not detect state directory", serviceType)
		}
	}

	s, err := service.NewServiceHandler(name, addr, c.flagMicroCloudDir, c.global.flagLogDebug, c.global.flagLogVerbose, services...)
	if err != nil {
		return err
	}

	endpoints := []rest.Endpoint{
		api.ServicesCmd(s),
		api.LXDProxy(s),
		api.CephProxy(s),
		api.OVNProxy(s),
	}

	return s.Services[types.MicroCloud].(*service.CloudService).StartCloud(s, endpoints)
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

	app.PersistentFlags().StringVar(&daemonCmd.flagMicroCloudDir, "state-dir", "", "Path to store state information for MicroCloud"+"``")

	app.SetVersionTemplate("{{.Version}}\n")

	err := app.Execute()
	if err != nil {
		os.Exit(1)
	}
}
