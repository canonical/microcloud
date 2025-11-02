package main

import (
	"context"
	"fmt"

	"github.com/canonical/microcluster/v3/microcluster"
	"github.com/spf13/cobra"
)

type cmdShutdown struct {
	common *CmdControl
}

// command returns the subcommand for shutting down the MicroCloud daemon.
func (c *cmdShutdown) command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shutdown",
		Short: "Shut down the MicroCloud daemon",
		RunE:  c.Run,
	}

	return cmd
}

// Run runs the subcommand for shutting down the MicroCloud daemon.
func (c *cmdShutdown) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	options := microcluster.Args{StateDir: c.common.FlagMicroCloudDir}
	m, err := microcluster.App(options)
	if err != nil {
		return err
	}

	err = m.Ready(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to wait for MicroCloud to get ready: %w", err)
	}

	client, err := m.LocalClient()
	if err != nil {
		return err
	}

	chResult := make(chan error, 1)
	go func() {
		defer close(chResult)

		err := client.ShutdownDaemon(context.Background())
		if err != nil {
			chResult <- err
			return
		}
	}()

	return <-chResult
}
