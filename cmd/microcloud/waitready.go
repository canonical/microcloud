package main

import (
	"context"

	"github.com/canonical/microcluster/microcluster"
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

	options := microcluster.Args{StateDir: c.common.FlagMicroCloudDir, Verbose: c.common.FlagLogVerbose, Debug: c.common.FlagLogDebug}
	m, err := microcluster.App(context.Background(), options)
	if err != nil {
		return err
	}

	return m.Ready(c.flagTimeout)
}
