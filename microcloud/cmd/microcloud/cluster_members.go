package main

import (
	"context"
	"sort"

	"github.com/canonical/microcluster/client"
	"github.com/canonical/microcluster/microcluster"
	"github.com/lxc/lxd/lxc/utils"
	"github.com/lxc/lxd/shared"
	"github.com/spf13/cobra"
)

type cmdClusterMembers struct {
	common *CmdControl
}

func (c *cmdClusterMembers) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "manage cluster members.",
		RunE:  c.Run,
	}

	var cmdRemove = cmdClusterMemberRemove{common: c.common}
	cmd.AddCommand(cmdRemove.Command())

	var cmdList = cmdClusterMembersList{common: c.common}
	cmd.AddCommand(cmdList.Command())

	return cmd
}

func (c *cmdClusterMembers) Run(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

type cmdClusterMembersList struct {
	common *CmdControl
}

func (c *cmdClusterMembersList) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <address>",
		Short: "List cluster members locally, or remotely if an address is specified.",
		RunE:  c.Run,
	}

	return cmd
}

func (c *cmdClusterMembersList) Run(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return cmd.Help()
	}

	// Get all state information for MicroCluster.
	options := microcluster.Args{StateDir: c.common.FlagMicroCloudDir, Verbose: c.common.FlagLogVerbose, Debug: c.common.FlagLogDebug}
	m, err := microcluster.App(context.Background(), options)
	if err != nil {
		return err
	}

	var client *client.Client

	// Get a local client connected to the unix socket if no address is specified.
	if len(args) == 1 {
		client, err = m.RemoteClient(args[0])
		if err != nil {
			return err
		}
	} else {
		client, err = m.LocalClient()
		if err != nil {
			return err
		}
	}

	clusterMembers, err := client.GetClusterMembers(context.Background())
	if err != nil {
		return err
	}

	data := make([][]string, len(clusterMembers))
	for i, clusterMember := range clusterMembers {
		fingerprint, err := shared.CertFingerprintStr(clusterMember.Certificate.String())
		if err != nil {
			continue
		}

		data[i] = []string{clusterMember.Name, clusterMember.Address.String(), clusterMember.Role, fingerprint, string(clusterMember.Status)}
	}

	header := []string{"NAME", "ADDRESS", "ROLE", "FINGERPRINT", "STATUS"}
	sort.Sort(utils.ByName(data))

	return utils.RenderTable(utils.TableFormatTable, header, data, clusterMembers)
}

type cmdClusterMemberRemove struct {
	common *CmdControl

	flagForce bool
}

func (c *cmdClusterMemberRemove) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove the cluster member with the given name.",
		RunE:  c.Run,
	}

	cmd.Flags().BoolVarP(&c.flagForce, "force", "f", false, "Forcibly remove the cluster member")

	return cmd
}

func (c *cmdClusterMemberRemove) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return cmd.Help()
	}

	options := microcluster.Args{StateDir: c.common.FlagMicroCloudDir, Verbose: c.common.FlagLogVerbose, Debug: c.common.FlagLogDebug}
	m, err := microcluster.App(context.Background(), options)
	if err != nil {
		return err
	}

	client, err := m.LocalClient()
	if err != nil {
		return err
	}

	err = client.DeleteClusterMember(context.Background(), args[0], c.flagForce)
	if err != nil {
		return err
	}

	return nil
}
