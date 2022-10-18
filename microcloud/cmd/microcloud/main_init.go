package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/lxc/lxd/lxc/utils"
	"github.com/lxc/lxd/lxd/util"
	cli "github.com/lxc/lxd/shared/cmd"
	"github.com/lxc/lxd/shared/logger"
	"github.com/lxc/lxd/shared/units"
	"github.com/spf13/cobra"

	"github.com/canonical/microceph/microceph/api/types"
	cephClient "github.com/canonical/microceph/microceph/client"
	"github.com/canonical/microcloud/microcloud/mdns"
	"github.com/canonical/microcloud/microcloud/service"
	"github.com/canonical/microcluster/client"
)

type cmdInit struct {
	common *CmdControl

	flagAuto bool

	flagMicroCephDir string
	flagLXDDir       string
}

func (c *cmdInit) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Initialize the network endpoint and create or join a new cluster",
		RunE:  c.Run,
	}

	cmd.Flags().BoolVar(&c.flagAuto, "auto", false, "Automatic setup with default configuration")
	cmd.PersistentFlags().StringVar(&c.flagMicroCephDir, "ceph-dir", "", "Path to store state information for MicroCeph"+"``")
	cmd.PersistentFlags().StringVar(&c.flagLXDDir, "lxd-dir", "", "Path to store state information for LXD"+"``")

	return cmd
}

func (c *cmdInit) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	addr := util.NetworkInterfaceAddress()
	name, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Failed to retrieve system honame: %w", err)
	}

	if !c.flagAuto {
		addr, err = cli.AskString(fmt.Sprintf("Please choose the address MicroCloud will be listening on [default=%s]: ", addr), addr, nil)
		if err != nil {
			return err
		}

		// FIXME: MicroCeph does not currently support non-hostname cluster names.
		// name, err = cli.AskString(fmt.Sprintf("Please choose a name for this system [default=%s]: ", name), name, nil)
		// if err != nil {
		// 	return err
		// }
	}

	cloud, err := service.NewCloudService(context.Background(), name, addr, c.common.FlagMicroCloudDir, c.common.FlagLogVerbose, c.common.FlagLogDebug)
	if err != nil {
		return err
	}

	ceph, err := service.NewCephService(context.Background(), name, addr, c.flagMicroCephDir, c.common.FlagLogVerbose, c.common.FlagLogDebug)
	if err != nil {
		return err
	}

	lxd, err := service.NewLXDService(name, addr, c.flagLXDDir)
	if err != nil {
		return err
	}

	s := service.NewServiceHandler(name, addr, *cloud, *ceph, *lxd)
	peers, err := lookupPeers(s, c.flagAuto)
	if err != nil {
		return err
	}

	err = Bootstrap(s, peers)
	if err != nil {
		return err
	}

	if !c.flagAuto {
		// FIXME: Add disks to LXD.
		return askDisks(s.Name, *ceph)
	}

	return nil

}

func lookupPeers(s *service.ServiceHandler, auto bool) (map[string]string, error) {
	stdin := bufio.NewReader(os.Stdin)
	totalPeers := map[string]string{}

	fmt.Println("Scanning for eligible servers...")
	if !auto {
		fmt.Println("Press enter to end scanning for servers")
	}

	// Wait for input to stop scanning.
	var doneCh chan error
	if !auto {
		doneCh = make(chan error)
		go func() {
			_, err := stdin.ReadByte()
			if err != nil {
				doneCh <- err
			} else {
				close(doneCh)
			}
		}()
	}

	for {
		select {
		case err := <-doneCh:
			if err != nil {
				return nil, err
			}

			return totalPeers, nil
		default:
			peers, err := mdns.LookupPeers(context.Background(), mdns.ClusterService, s.Name)
			if err != nil {
				return nil, err
			}

			for peer, addr := range peers {
				_, ok := totalPeers[peer]
				if !ok {
					fmt.Printf(" Found %q at %q\n", peer, addr)
					totalPeers[peer] = addr
				}
			}

			if auto {
				return totalPeers, nil
			}

			// Sleep for a few seconds before retrying.
			time.Sleep(5 * time.Second)
		}
	}

	return totalPeers, nil
}

func Bootstrap(sh *service.ServiceHandler, peers map[string]string) error {
	fmt.Println("Initializing a new cluster")
	for serviceType, service := range sh.Services {
		err := service.Bootstrap()
		if err != nil {
			return fmt.Errorf("Failed to bootstrap local %s: %w", serviceType, err)
		}

		fmt.Printf(" Local %s has been bootstrapped\n", serviceType)
	}

	tokensByName := make(map[string]map[string]string, len(peers))
	for serviceType, s := range sh.Services {
		for peer := range peers {
			token, err := s.IssueToken(peer)
			if err != nil {
				return fmt.Errorf("Failed to issue %s token for peer %q: %w", serviceType, peer, err)
			}

			_, ok := tokensByName[peer]
			if !ok {
				tokensByName[peer] = make(map[string]string, len(sh.Services))
			}

			tokensByName[peer][string(serviceType)] = token
		}
	}

	bytes, err := json.Marshal(tokensByName)
	if err != nil {
		return fmt.Errorf("Failed to marshal list of tokens: %w", err)
	}

	fmt.Println("Awaiting cluster formation...")
	server, err := mdns.NewBroadcast(mdns.TokenService, sh.Name, sh.Address, sh.Port, bytes)
	if err != nil {
		return fmt.Errorf("Failed to begin join token broadcast: %w", err)
	}

	// Shutdown the server after 30 seconds.
	timeAfter := time.After(time.Second * 30)
	bootstrapDoneCh := make(chan struct{})
	var bootstrapDone bool
	for {
		select {
		case <-bootstrapDoneCh:
			fmt.Println("Cluster initialization is complete")
			logger.Info("Shutting down broadcast")
			err := server.Shutdown()
			if err != nil {
				return fmt.Errorf("Failed to shutdown mDNS server after timeout: %w", err)
			}

			bootstrapDone = true
		case <-timeAfter:
			logger.Info("Shutting down broadcast")
			err := server.Shutdown()
			if err != nil {
				return fmt.Errorf("Failed to shutdown mDNS server after timeout: %w", err)
			}

			bootstrapDone = true
		default:
			// Sleep a bit so the loop doesn't push the CPU as hard.
			time.Sleep(100 * time.Millisecond)

			peers, err := mdns.LookupPeers(context.Background(), mdns.JoinedService, sh.Name)
			if err != nil {
				return fmt.Errorf("Failed to lookup records from new cluster members: %w", err)
			}

			for peer := range peers {
				_, ok := tokensByName[peer]
				if ok {
					fmt.Printf(" Peer %q has joined the cluster\n", peer)
				}

				delete(tokensByName, peer)
			}

			if len(tokensByName) == 0 {
				close(bootstrapDoneCh)
			}
		}

		if bootstrapDone {
			break
		}
	}

	return nil
}

func askDisks(localName string, s service.CephService) error {
	// Add some disks.
	wantsDisks, err := cli.AskBool("Would you like to add additional local disks to MicroCeph? (yes/no) [default=yes]: ", "yes")
	if err != nil {
		return err
	}

	if !wantsDisks {
		return nil
	}

	lc, err := s.Client.LocalClient()
	if err != nil {
		return err
	}

	peers, err := lc.GetClusterMembers(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to get list of current peers: %w", err)
	}

	peerAddrs := make(map[string]string, len(peers))
	for _, peer := range peers {
		peerAddrs[peer.Name] = peer.Address.String()
	}

	validator := func(input string) error {
		_, ok := peerAddrs[input]
		if !ok {
			return fmt.Errorf("Invalid peer name %q", input)
		}

		return nil
	}

	for {
		target, err := cli.AskString(fmt.Sprintf("Which peer would like to add additional local disks on? [default=%s]: ", localName), localName, validator)
		if err != nil {
			return err
		}

		if target != localName {
			lc, err = s.Client.RemoteClient(peerAddrs[target])
			if err != nil {
				return err
			}
		}

		err = listLocalDisks(lc)
		if err != nil {
			return err
		}

		for {
			diskPath, err := cli.AskString("What's the disk path? (empty to exit): ", "", func(input string) error { return nil })
			if err != nil {
				return err
			}

			if diskPath == "" {
				break
			}

			diskWipe, err := cli.AskBool("Would you like the disk to be wiped? [default=no]: ", "no")
			if err != nil {
				return err
			}

			// Add the disk.
			req := &types.DisksPost{
				Path: diskPath,
				Wipe: diskWipe,
			}

			err = cephClient.AddDisk(context.Background(), lc, req)
			if err != nil {
				return err
			}

			moreDisks, err := cli.AskBool("Would you like to add another disk on this peer? [default=no]: ", "no")
			if err != nil {
				return err
			}

			if !moreDisks {
				break
			}
		}

		moreDisks, err := cli.AskBool("Would you like to add disks on another peer? [default=yes]: ", "no")
		if err != nil {
			return err
		}

		if !moreDisks {
			break
		}
	}

	return nil
}

func listLocalDisks(cli *client.Client) error {
	// List configured disks.
	disks, err := cephClient.GetDisks(context.Background(), cli)
	if err != nil {
		return err
	}

	// List physical disks.
	resources, err := cephClient.GetResources(context.Background(), cli)
	if err != nil {
		return err
	}

	// Get local hostname.
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	// Prepare the table.
	data := [][]string{}
	for _, disk := range resources.Disks {
		if len(disk.Partitions) > 0 {
			continue
		}

		devicePath := fmt.Sprintf("/dev/disk/by-id/%s", disk.DeviceID)

		found := false
		for _, entry := range disks {
			if entry.Location != hostname {
				continue
			}

			if entry.Path == devicePath {
				found = true
				break
			}
		}

		if found {
			continue
		}

		data = append(data, []string{disk.Model, units.GetByteSizeStringIEC(int64(disk.Size), 2), disk.Type, devicePath})
	}

	fmt.Println("")
	fmt.Println("Available unpartitioned disks on this system:")

	header := []string{"MODEL", "CAPACITY", "TYPE", "PATH"}
	sort.Sort(utils.ByName(data))

	err = utils.RenderTable(utils.TableFormatTable, header, data, disks)
	if err != nil {
		return err
	}

	return nil
}
