package main

import (
	"context"
	"fmt"
	"time"

	"github.com/canonical/microcluster/microcluster"
	"github.com/spf13/cobra"
)

type cmdInit struct {
	common *CmdControl

	flagBootstrap bool
	flagToken     string
}

func (c *cmdInit) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <name> <address>",
		Short: "Initialize the network endpoint and create or join a new cluster",
		RunE:  c.Run,
		Example: `  microctl init member1 127.0.0.1:8443 --bootstrap
    microctl init member1 127.0.0.1:8443 --token <token>`,
	}

	cmd.Flags().BoolVar(&c.flagBootstrap, "bootstrap", false, "Configure a new cluster with this daemon")
	cmd.Flags().StringVar(&c.flagToken, "token", "", "Join a cluster with a join token")
	return cmd
}

func (c *cmdInit) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 2 {
		return cmd.Help()
	}

	m, err := microcluster.App(context.Background(), c.common.FlagStateDir, c.common.FlagLogVerbose, c.common.FlagLogDebug)
	if err != nil {
		return fmt.Errorf("Unable to configure MicroCluster: %w", err)
	}

	if c.flagBootstrap && c.flagToken != "" {
		return fmt.Errorf("Option must be one of bootstrap or token")
	}

	if c.flagBootstrap {
		return m.NewCluster(args[0], args[1], time.Second*30)
	}

	if c.flagToken != "" {
		return m.JoinCluster(args[0], args[1], c.flagToken, time.Second*30)
	}

	return fmt.Errorf("Option must be one of bootstrap or token")
}
