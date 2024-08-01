package main

import (
	"context"
	"time"

	"github.com/canonical/microcluster/v2/microcluster"
	"github.com/spf13/cobra"
)

type cmdWaitready struct {
	common *CmdControl

	flagTimeout int
}

func (c *cmdWaitready) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "waitready",
		Short: "Wait for the daemon to be ready to process requests",
		RunE:  c.Run,
	}

	cmd.Flags().IntVarP(&c.flagTimeout, "timeout", "t", 0, "Number of seconds to wait before giving up"+"``")

	return cmd
}

func (c *cmdWaitready) Run(cmd *cobra.Command, args []string) error {
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

	return m.Ready(ctx)
}
