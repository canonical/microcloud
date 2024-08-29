package main

import (
	"context"

	"github.com/canonical/microcluster/v2/microcluster"
	"github.com/spf13/cobra"

	cloudClient "github.com/canonical/microcloud/microcloud/client"
)

type cmdRemove struct {
	common *CmdControl

	flagForce bool
}

func (c *cmdRemove) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Remove the specified member from all MicroCloud services",
		RunE:    c.Run,
	}

	cmd.Flags().BoolVarP(&c.flagForce, "force", "f", false, "Forcibly remove the cluster member")

	return cmd
}

func (c *cmdRemove) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return cmd.Help()
	}

	options := microcluster.Args{StateDir: c.common.FlagMicroCloudDir}
	m, err := microcluster.App(options)
	if err != nil {
		return err
	}

	client, err := m.LocalClient()
	if err != nil {
		return err
	}

	return cloudClient.DeleteClusterMember(context.Background(), client, args[0], c.flagForce)
}
