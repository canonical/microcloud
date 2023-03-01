// Package microcloudd provides the daemon.
package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/lxc/lxd/lxd/util"
	"github.com/lxc/lxd/shared/logger"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/service"
	"github.com/canonical/microcloud/microcloud/version"
	"github.com/canonical/microcluster/microcluster"
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

	cloud, err := service.NewCloudService(context.Background(), name, addr, c.flagMicroCloudDir, c.global.flagLogVerbose, c.global.flagLogDebug)
	if err != nil {
		return err
	}

	lxd, err := service.NewLXDService(context.Background(), name, addr, c.flagMicroCloudDir)
	if err != nil {
		return err
	}

	services := []service.Service{*cloud, *lxd}
	app, err := microcluster.App(context.Background(), microcluster.Args{StateDir: api.MicroCephDir})
	if err != nil {
		return err
	}

	_, err = os.Stat(app.FileSystem.ControlSocket().URL.Host)
	if err == nil {
		ceph, err := service.NewCephService(context.Background(), name, addr, c.flagMicroCloudDir)
		if err != nil {
			return err
		}

		services = append(services, *ceph)
	} else {
		logger.Info("Skipping MicroCeph service, could not detect state directory")
	}

	service := service.NewServiceHandler(name, addr, services...)
	return cloud.StartCloud(service)
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
