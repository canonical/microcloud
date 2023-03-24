package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	cephTypes "github.com/canonical/microceph/microceph/api/types"
	"github.com/canonical/microceph/microceph/client"
	ovnClient "github.com/canonical/microovn/microovn/client"
	"github.com/lxc/lxd/lxd/util"
	lxdAPI "github.com/lxc/lxd/shared/api"
	cli "github.com/lxc/lxd/shared/cmd"
	"github.com/lxc/lxd/shared/logger"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/mdns"
	"github.com/canonical/microcloud/microcloud/service"
)

type cmdInit struct {
	common *CmdControl

	flagAutoSetup    bool
	flagWipeAllDisks bool
}

func (c *cmdInit) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init",
		Aliases: []string{"bootstrap"},
		Short:   "Initialize the network endpoint and create or join a new cluster",
		RunE:    c.Run,
	}

	cmd.Flags().BoolVar(&c.flagAutoSetup, "auto", false, "Automatic setup with default configuration")
	cmd.Flags().BoolVar(&c.flagWipeAllDisks, "wipe", false, "Wipe disks to add to MicroCeph")

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

	if !c.flagAutoSetup {
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

	services := []types.ServiceType{types.MicroCloud, types.LXD}
	optionalServices := map[types.ServiceType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	services, err = askMissingServices(services, optionalServices, c.flagAutoSetup)
	if err != nil {
		return err
	}

	s, err := service.NewServiceHandler(name, addr, c.common.FlagMicroCloudDir, c.common.FlagLogDebug, c.common.FlagLogVerbose, services...)
	if err != nil {
		return err
	}

	peers, err := lookupPeers(s, c.flagAutoSetup)
	if err != nil {
		return err
	}

	lxdDisks, cephDisks, err := askDisks(s, peers, true, c.flagAutoSetup, c.flagWipeAllDisks)
	if err != nil {
		return err
	}

	fmt.Println("Initializing a new cluster")
	err = s.RunConcurrent(true, func(s service.Service) error {
		err := s.Bootstrap()
		if err != nil {
			return fmt.Errorf("Failed to bootstrap local %s: %w", s.Type(), err)
		}

		fmt.Printf(" Local %s is ready\n", s.Type())

		return nil
	})
	if err != nil {
		return err
	}

	if len(cephDisks) > 0 {
		c, err := s.Services[types.MicroCeph].(*service.CephService).Client("", "")
		if err != nil {
			return err
		}

		for _, disk := range cephDisks[s.Name] {
			err = client.AddDisk(context.Background(), c, &disk)
			if err != nil {
				return err
			}
		}
	}

	err = AddPeers(s, peers, lxdDisks, cephDisks)
	if err != nil {
		return err
	}

	err = postClusterSetup(true, s, peers, lxdDisks, cephDisks)
	if err != nil {
		return err
	}

	fmt.Println("MicroCloud is ready")

	return nil
}

func lookupPeers(s *service.ServiceHandler, autoSetup bool) (map[string]mdns.ServerInfo, error) {
	stdin := bufio.NewReader(os.Stdin)
	totalPeers := map[string]mdns.ServerInfo{}
	skipPeers := map[string]bool{}

	fmt.Println("Scanning for eligible servers...")
	if !autoSetup {
		fmt.Println("Press enter to end scanning for servers")
	}

	// Wait for input to stop scanning.
	var doneCh chan error
	if !autoSetup {
		doneCh = make(chan error)
		go func() {
			_, err := stdin.ReadByte()
			if err != nil {
				doneCh <- err
			} else {
				close(doneCh)
			}

			fmt.Println("Ending scan")
		}()
	}

	done := false
	for !done {
		select {
		case err := <-doneCh:
			if err != nil {
				return nil, err
			}

			done = true
		default:
			peers, err := mdns.LookupPeers(context.Background(), mdns.Version, s.Name)
			if err != nil {
				return nil, err
			}

			for peer, info := range peers {
				if skipPeers[peer] {
					continue
				}

				_, ok := totalPeers[peer]
				if !ok {
					serviceMap := make(map[types.ServiceType]bool, len(info.Services))
					for _, service := range info.Services {
						serviceMap[service] = true
					}

					for service := range s.Services {
						if !serviceMap[service] {
							skipPeers[peer] = true
							logger.Infof("Skipping peer %q due to missing services (%s)", peer, string(service))
							break
						}
					}

					if !skipPeers[peer] {
						fmt.Printf(" Found %q at %q\n", peer, info.Address)
						totalPeers[peer] = info
					}
				}
			}

			if autoSetup {
				done = true
				break
			}

			// Sleep for a few seconds before retrying.
			time.Sleep(5 * time.Second)
		}
	}

	if len(totalPeers) == 0 {
		return nil, fmt.Errorf("Found no available systems")
	}

	return totalPeers, nil
}

func AddPeers(sh *service.ServiceHandler, peers map[string]mdns.ServerInfo, localDisks map[string][]lxdAPI.ClusterMemberConfigKey, cephDisks map[string][]cephTypes.DisksPost) error {
	joinConfig := make(map[string]types.ServicesPut, len(peers))
	secrets := make(map[string]string, len(peers))
	for peer, info := range peers {
		joinConfig[peer] = types.ServicesPut{
			Tokens:     []types.ServiceToken{},
			Address:    info.Address,
			LXDConfig:  localDisks[peer],
			CephConfig: cephDisks[peer],
		}

		secrets[peer] = info.AuthSecret
	}

	mut := sync.Mutex{}
	err := sh.RunConcurrent(false, func(s service.Service) error {
		for peer := range peers {
			token, err := s.IssueToken(peer)
			if err != nil {
				return fmt.Errorf("Failed to issue %s token for peer %q: %w", s.Type(), peer, err)
			}

			mut.Lock()
			cfg := joinConfig[peer]
			cfg.Tokens = append(joinConfig[peer].Tokens, types.ServiceToken{Service: s.Type(), JoinToken: token})
			joinConfig[peer] = cfg
			mut.Unlock()
		}

		return nil
	})
	if err != nil {
		return err
	}

	fmt.Println("Awaiting cluster formation...")
	// Initially add just 2 peers for each dqlite service to handle issues with role reshuffling while another
	// node is joining the cluster.
	initialSize := 2
	if len(joinConfig) > initialSize {
		initialCfg := map[string]types.ServicesPut{}
		for peer, info := range joinConfig {
			initialCfg[peer] = info

			if len(initialCfg) == initialSize {
				break
			}
		}

		initialSecrets := make(map[string]string, len(initialCfg))
		for peer := range initialCfg {
			delete(joinConfig, peer)
			initialSecrets[peer] = secrets[peer]
			delete(secrets, peer)
		}

		err := waitForCluster(sh, initialSecrets, initialCfg)
		if err != nil {
			return err
		}

		// Sleep 3 seconds to give the cluster roles time to reshuffle before adding more members.
		time.Sleep(3 * time.Second)
	}

	err = waitForCluster(sh, secrets, joinConfig)
	if err != nil {
		return err
	}

	fmt.Println("Cluster initialization is complete")
	cloudService, ok := sh.Services[types.MicroCloud]
	if !ok {
		return fmt.Errorf("Missing MicroCloud service")
	}

	cloudCluster, err := cloudService.ClusterMembers()
	if err != nil {
		return fmt.Errorf("Failed to get %s service cluster members: %w", cloudService.Type(), err)
	}

	err = sh.RunConcurrent(false, func(s service.Service) error {
		if s.Type() == types.MicroCloud {
			return nil
		}

		cluster, err := s.ClusterMembers()
		if err != nil {
			return fmt.Errorf("Failed to get %s service cluster members: %w", s.Type(), err)
		}

		if len(cloudCluster) != len(cluster) {
			return fmt.Errorf("%s service cluster does not match %s", s.Type(), cloudService.Type())
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// waitForCluster will loop until the timeout has passed, or all cluster members for all services have reported that
// their join process is complete.
func waitForCluster(sh *service.ServiceHandler, secrets map[string]string, peers map[string]types.ServicesPut) error {
	cloud := sh.Services[types.MicroCloud].(*service.CloudService)
	joinedChan := cloud.RequestJoin(context.Background(), secrets, peers)
	timeAfter := time.After(5 * time.Minute)
	for {
		select {
		case <-timeAfter:
			return fmt.Errorf("Timed out waiting for a response from all cluster members")
		case entry, ok := <-joinedChan:
			if !ok {
				logger.Info("Join response channel has closed")

				if len(peers) != 0 {
					return fmt.Errorf("%q members failed to join the cluster", len(peers))
				}

				return nil
			}

			if entry.Error != nil {
				return fmt.Errorf("Peer %q failed to join the cluster: %w", entry.Name, entry.Error)
			}

			_, ok = peers[entry.Name]
			if !ok {
				return fmt.Errorf("Unexpected response from unknown server %q", entry.Name)
			}

			fmt.Printf(" Peer %q has joined the cluster\n", entry.Name)

			delete(peers, entry.Name)

			if len(peers) == 0 {
				close(joinedChan)

				return nil
			}
		}
	}
}

func postClusterSetup(bootstrap bool, sh *service.ServiceHandler, peers map[string]mdns.ServerInfo, lxdDisks map[string][]lxdAPI.ClusterMemberConfigKey, cephDisks map[string][]cephTypes.DisksPost) error {
	cephTargets := map[string]string{}
	for target := range cephDisks {
		cephTargets[target] = peers[target].AuthSecret
	}

	ovnTargets := map[string]string{}
	var ovnConfig string
	if sh.Services[types.MicroOVN] != nil {
		ovn := sh.Services[types.MicroOVN].(*service.OVNService)
		c, err := ovn.Client()
		if err != nil {
			return err
		}

		services, err := ovnClient.GetServices(context.Background(), c)
		if err != nil {
			return err
		}

		conns := []string{}
		for _, service := range services {
			if service.Service == "central" {
				conns = append(conns, fmt.Sprintf("tcp:%s", util.CanonicalNetworkAddress(peers[service.Location].Address, 6641)))
			}
		}

		ovnConfig = strings.Join(conns, ",")
		for peer, info := range peers {
			ovnTargets[peer] = info.AuthSecret
		}
	}

	lxdTargets := map[string]string{}
	for peer := range lxdDisks {
		lxdTargets[peer] = peers[peer].AuthSecret
	}

	return sh.Services[types.LXD].(*service.LXDService).Configure(bootstrap, lxdTargets, cephTargets, ovnConfig, ovnTargets)
}
