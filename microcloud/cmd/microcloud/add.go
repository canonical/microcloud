package main

import (
	"context"
	"fmt"
	"strings"

	cephTypes "github.com/canonical/microceph/microceph/api/types"
	"github.com/canonical/microcluster/microcluster"
	lxdAPI "github.com/lxc/lxd/shared/api"
	cli "github.com/lxc/lxd/shared/cmd"
	"github.com/lxc/lxd/shared/logger"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/mdns"
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

	missingServices := []string{}
	for serviceType, stateDir := range optionalServices {
		if service.ServiceExists(serviceType, stateDir) {
			services = append(services, serviceType)
		} else {
			missingServices = append(missingServices, string(serviceType))
		}
	}

	if len(missingServices) > 0 {
		serviceStr := strings.Join(missingServices, ",")
		if !c.flagAuto {
			skip, err := cli.AskBool(fmt.Sprintf("%s not found. Continue anyway? (yes/no) [default=yes]: ", serviceStr), "yes")
			if err != nil {
				return err
			}

			if !skip {
				return nil
			}
		}

		logger.Infof("Skipping %s (could not detect service state directory)", serviceStr)
	}

	s, err := service.NewServiceHandler(status.Name, addr, c.common.FlagMicroCloudDir, c.common.FlagLogDebug, c.common.FlagLogVerbose, services...)
	if err != nil {
		return err
	}

	peers, err := lookupPeers(s, c.flagAuto)
	if err != nil {
		return err
	}

	allResources := make(map[string]*lxdAPI.Resources, len(peers)+1)
	for peer, info := range peers {
		allResources[peer], err = s.Services[types.LXD].(*service.LXDService).GetResources(true, peer, info.Address, info.AuthSecret)
		if err != nil {
			return fmt.Errorf("Failed to get system resources of peer %q: %w", peer, err)
		}
	}

	validDisks := make(map[string][]lxdAPI.ResourcesStorageDisk, len(allResources))
	for peer, r := range allResources {
		validDisks[peer] = make([]lxdAPI.ResourcesStorageDisk, 0, len(r.Storage.Disks))
		for _, disk := range r.Storage.Disks {
			if len(disk.Partitions) == 0 {
				validDisks[peer] = append(validDisks[peer], disk)
			}
		}
	}

	var diskConfig map[string][]lxdAPI.ClusterMemberConfigKey
	var reservedDisks map[string]string
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
			diskConfig, reservedDisks, err = askLocalPool(peers, validDisks, c.flagAuto, c.flagWipe, *lxd)

			return err
		})
	}

	var cephDisks map[string][]cephTypes.DisksPost
	if s.Services[types.MicroCeph] != nil {
		ceph, ok := s.Services[types.MicroCeph].(*service.CephService)
		if !ok {
			return fmt.Errorf("Invalid MicroCeph service")
		}

		wantsDisks = true
		if !c.flagAuto {
			wantsDisks, err = cli.AskBool("Would you like to add additional disks to MicroCeph? (yes/no) [default=yes]: ", "yes")
			if err != nil {
				return err
			}
		}

		if wantsDisks {
			askRetry("Retry selecting disks?", c.flagAuto, func() error {
				peers[status.Name] = mdns.ServerInfo{Name: status.Name, Address: addr}
				defer delete(peers, status.Name)

				cephDisks, err = askRemotePool(peers, validDisks, c.flagAuto, c.flagWipe, *ceph, reservedDisks, false)

				return err
			})
		}
	}

	err = AddPeers(s, peers, diskConfig)
	if err != nil {
		return err
	}

	if len(cephDisks) > 0 && s.Services[types.MicroCeph] != nil {
		_, err = finalizeRemoteDisks(*s.Services[types.MicroCeph].(*service.CephService), cephDisks, peers)
		if err != nil {
			return err
		}
	}

	return nil
}
