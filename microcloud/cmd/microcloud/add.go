package main

import (
	"context"
	"fmt"
	"os"

	"github.com/canonical/microcluster/microcluster"
	lxdAPI "github.com/lxc/lxd/shared/api"
	cli "github.com/lxc/lxd/shared/cmd"
	"github.com/lxc/lxd/shared/logger"
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
	app, err := microcluster.App(context.Background(), microcluster.Args{StateDir: api.MicroCephDir})
	if err != nil {
		return err
	}

	_, err = os.Stat(app.FileSystem.ControlSocket().URL.Host)
	if err == nil {
		services = append(services, types.MicroCeph)
	} else {
		logger.Info("Skipping MicroCeph service, could not detect state directory")
	}

	s, err := service.NewServiceHandler(status.Name, addr, c.common.FlagMicroCloudDir, c.common.FlagLogDebug, c.common.FlagLogVerbose, services...)
	if err != nil {
		return err
	}

	peers, err := lookupPeers(s, c.flagAuto)
	if err != nil {
		return err
	}

	if len(peers) == 0 {
		return fmt.Errorf("Found no available systems")
	}

	var localDisks map[string][]lxdAPI.ClusterMemberConfigKey
	wantsDisks := true
	if !c.flagAuto {
		wantsDisks, err = cli.AskBool("Would you like to add a local LXD storage pool? (yes/no) [default=yes]: ", "yes")
		if err != nil {
			return err
		}
	}

	if wantsDisks {
		askRetry("Retry selecting disks?", c.flagAuto, func() error {
			lxd := s.Services[types.LXD].(*service.LXDService)
			localDisks, err = askLocalPool(peers, c.flagAuto, c.flagWipe, *lxd)

			return err
		})
	}

	err = AddPeers(s, peers, localDisks)
	if err != nil {
		return err
	}

	if s.Services[types.MicroCeph] != nil {
		ceph, ok := s.Services[types.MicroCeph].(service.CephService)
		if !ok {
			return fmt.Errorf("Invalid MicroCeph service")
		}

		wantsDisks = true
		if !c.flagAuto {
			wantsDisks, err = cli.AskBool("Would you like to add additional local disks to MicroCeph? (yes/no) [default=yes]: ", "yes")
			if err != nil {
				return err
			}
		}

		if wantsDisks {
			peerNames := make([]string, 0, len(peers))
			for peer := range peers {
				peerNames = append(peerNames, peer)
			}

			reservedDisks := map[string]string{}
			for peer, config := range localDisks {
				for _, entry := range config {
					if entry.Key == "source" {
						reservedDisks[peer] = entry.Value
						break
					}
				}
			}

			askRetry("Retry selecting disks?", c.flagAuto, func() error {
				lxd := s.Services[types.LXD].(*service.LXDService)
				_, err = askRemotePool(peerNames, c.flagAuto, c.flagWipe, ceph, *lxd, reservedDisks, false)

				return err
			})
		}
	}

	return nil
}
