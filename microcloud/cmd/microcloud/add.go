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

	flagAuto bool
	flagWipe bool
}

func (c *cmdAdd) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Begins scanning for new cluster members",
		RunE:  c.Run,
	}

	cmd.Flags().BoolVar(&c.flagAuto, "auto", false, "Automatic setup with default configuration")
	cmd.Flags().BoolVar(&c.flagWipe, "wipe", false, "Wipe disks to add to MicroCeph")

	return cmd
}

func (c *cmdAdd) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	cloudApp, err := microcluster.App(context.Background(), microcluster.Args{StateDir: c.common.FlagMicroCloudDir})
	if err != nil {
		return err
	}

	status, err := cloudApp.Status()
	if err != nil {
		return fmt.Errorf("Failed to get MicroCloud status: %w", err)
	}

	if !status.Ready {
		return fmt.Errorf("MicroCloud is uninitialized, run 'microcloud init' first")
	}

	addr := status.Address.Addr().String()
	services := []types.ServiceType{types.MicroCloud, types.LXD}
	optionalServices := map[types.ServiceType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	services, err = askMissingServices(services, optionalServices, c.flagAuto)
	if err != nil {
		return err
	}

	s, err := service.NewServiceHandler(status.Name, addr, c.common.FlagMicroCloudDir, c.common.FlagLogDebug, c.common.FlagLogVerbose, services...)
	if err != nil {
		return err
	}

	peers, err := lookupPeers(s, c.flagAuto)
	if err != nil {
		return err
	}

	lxdDisks, cephDisks, err := askDisks(s, peers, false, c.flagAuto, c.flagWipe)
	if err != nil {
		return err
	}

	err = AddPeers(s, peers, lxdDisks)
	if err != nil {
		return err
	}

	return postClusterSetup(s, peers, lxdDisks, cephDisks)
}
