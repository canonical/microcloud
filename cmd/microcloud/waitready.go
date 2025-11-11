package main

import (
	"context"
	"fmt"
	"time"

	"github.com/canonical/microcluster/v3/microcluster"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/service"
)

type cmdWaitready struct {
	common *CmdControl

	flagTimeout int
}

// command returns the subcommand for waiting on the daemon to be ready.
func (c *cmdWaitready) command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "waitready",
		Short: "Wait for MicroCloud to be ready to process requests",
		RunE:  c.run,
	}

	cmd.Flags().IntVarP(&c.flagTimeout, "timeout", "t", 0, "Number of seconds to wait before giving up"+"``")

	return cmd
}

// run runs the subcommand for waiting on the daemon to be ready.
func (c *cmdWaitready) run(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return cmd.Help()
	}

	options := microcluster.Args{StateDir: c.common.FlagMicroCloudDir}
	m, err := microcluster.App(options)
	if err != nil {
		return err
	}

	ctx := context.Background()
	if c.flagTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Second*time.Duration(c.flagTimeout))
		defer cancel()
	}

	// First wait for the local MicroCloud daemon.
	err = m.Ready(ctx)
	if err != nil {
		return fmt.Errorf("Failed waiting for the MicroCloud daemon: %w", err)
	}

	// Only MicroCloud's state dir is required as we use the proxy to reach out to LXD's unix socket.
	lxdService, err := service.NewLXDService("", "", c.common.FlagMicroCloudDir)
	if err != nil {
		return fmt.Errorf("Failed to create LXD service: %w", err)
	}

	initialized, _ := lxdService.IsInitialized(ctx)
	if initialized {
		client, err := lxdService.Client(ctx)
		if err != nil {
			return err
		}

		// Wait for all networks and storage pools to be ready in LXD.
		// If no remote storage was configured, wait for the local pool.
		// If no distributed networking was configured, wait for the FAN network.
		err = lxdService.WaitReady(ctx, client, true, true)
		if err != nil {
			return fmt.Errorf("Failed waiting for the LXD daemon: %w", err)
		}
	}

	return nil
}
