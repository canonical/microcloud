package main

import (
	"context"
	"fmt"

	"github.com/canonical/microcluster/microcluster"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/service"
)

type cmdAdd struct {
	common *CmdControl

	flagAutoSetup bool
	flagWipe      bool
	flagPreseed   bool
}

func (c *cmdAdd) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Scan for new cluster members to add",
		RunE:  c.Run,
	}

	cmd.Flags().BoolVar(&c.flagAutoSetup, "auto", false, "Automatic setup with default configuration")
	cmd.Flags().BoolVar(&c.flagWipe, "wipe", false, "Wipe disks to add to MicroCeph")
	cmd.Flags().BoolVar(&c.flagPreseed, "preseed", false, "Expect Preseed YAML for configuring MicroCloud in stdin")

	return cmd
}

func (c *cmdAdd) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	if c.flagPreseed {
		return c.common.RunPreseed(cmd, false)
	}

	cloudApp, err := microcluster.App(microcluster.Args{StateDir: c.common.FlagMicroCloudDir})
	if err != nil {
		return err
	}

	status, err := cloudApp.Status(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to get MicroCloud status: %w", err)
	}

	if !status.Ready {
		return fmt.Errorf("MicroCloud is uninitialized, run 'microcloud init' first")
	}

	addr, iface, subnet, err := c.common.askAddress(c.flagAutoSetup, status.Address.Addr().String())
	if err != nil {
		return err
	}

	services := []types.ServiceType{types.MicroCloud, types.LXD}
	optionalServices := map[types.ServiceType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	services, err = c.common.askMissingServices(services, optionalServices, c.flagAutoSetup)
	if err != nil {
		return err
	}

	s, err := service.NewHandler(status.Name, addr, c.common.FlagMicroCloudDir, c.common.FlagLogDebug, c.common.FlagLogVerbose, services...)
	if err != nil {
		return err
	}

	systems := map[string]InitSystem{}
	err = lookupPeers(s, c.flagAutoSetup, iface, subnet, nil, systems)
	if err != nil {
		return err
	}

	err = c.common.askDisks(s, systems, c.flagAutoSetup, c.flagWipe, false)
	if err != nil {
		return err
	}

	err = c.common.askNetwork(s, systems, subnet, c.flagAutoSetup, false)
	if err != nil {
		return err
	}

	return setupCluster(s, false, systems)
}
