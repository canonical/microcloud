package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/microcluster/state"
	"github.com/hashicorp/mdns"

	"github.com/canonical/microcloud/microcloud/api/types"
	cloudMDNS "github.com/canonical/microcloud/microcloud/mdns"
)

const (
	// OVNPort is the efault MicroOVN port.
	OVNPort int = 6443

	// CephPort is the efault MicroCeph port.
	CephPort int = 7443

	// LXDPort is the efault LXD port.
	LXDPort int = 8443

	// CloudPort is the efault MicroCloud port.
	CloudPort int = 9443
)

// ServiceHandler holds a set of services and an mdns server for communication between them.
type ServiceHandler struct {
	servers []*mdns.Server

	Services map[types.ServiceType]Service
	Name     string
	Address  string
	Port     int

	AuthSecret string
}

// NewServiceHandler creates a new ServiceHandler with a client for each of the given services.
func NewServiceHandler(name string, addr string, stateDir string, debug bool, verbose bool, services ...types.ServiceType) (*ServiceHandler, error) {
	servicesMap := make(map[types.ServiceType]Service, len(services))
	for _, serviceType := range services {
		var service Service
		var err error
		switch serviceType {
		case types.MicroCloud:
			service, err = NewCloudService(context.Background(), name, addr, stateDir, verbose, debug)
			break
		case types.MicroCeph:
			service, err = NewCephService(context.Background(), name, addr, stateDir)
			break
		case types.MicroOVN:
			service, err = NewOVNService(context.Background(), name, addr, stateDir)
			break
		case types.LXD:
			service, err = NewLXDService(context.Background(), name, addr, stateDir)
			break
		}

		if err != nil {
			return nil, fmt.Errorf("Failed to create %q service: %w", serviceType, err)
		}

		servicesMap[serviceType] = service
	}

	return &ServiceHandler{
		servers:  []*mdns.Server{},
		Services: servicesMap,
		Name:     name,
		Address:  addr,
		Port:     CloudPort,
	}, nil
}

// Start is run after the MicroCloud daemon has started. It will periodically check for join token broadcasts, and if
// found, will join all known services.
func (s *ServiceHandler) Start(state *state.State) error {
	// If we are already initialized, there's nothing to do.
	if state.Database.IsOpen() {
		return nil
	}

	// Attempt to wake up LXD so it can generate certificates already.
	d, err := lxd.ConnectLXDUnix("/var/snap/lxd/common/lxd/unix.socket", nil)
	if err == nil {
		_, _, _ = d.GetServer()
	}

	s.AuthSecret, err = shared.RandomCryptoString()
	if err != nil {
		return err
	}

	return s.Broadcast()
}

// Broadcast broadcasts service information over mDNS.
func (s *ServiceHandler) Broadcast() error {
	services := make([]types.ServiceType, 0, len(s.Services))
	for service := range s.Services {
		services = append(services, service)
	}

	networks, err := cloudMDNS.GetNetworkInfo()
	if err != nil {
		return err
	}

	info := cloudMDNS.ServerInfo{
		Version:    cloudMDNS.Version,
		Name:       s.Name,
		Services:   services,
		AuthSecret: s.AuthSecret,
	}

	// Prepare up to `ServiceSize` variations of the broadcast for each network interface.
	broadcasts := make([][]cloudMDNS.ServerInfo, cloudMDNS.ServiceSize)
	for i, net := range networks {
		info.Address = net.Address
		info.Interface = net.Interface

		services := broadcasts[i%cloudMDNS.ServiceSize]
		if services == nil {
			services = []cloudMDNS.ServerInfo{}
		}

		services = append(services, info)
		broadcasts[i%cloudMDNS.ServiceSize] = services
	}

	// Broadcast up to `ServiceSize` times with different service names before overlapping.
	// The lookup won't know how many records there are, so this will reduce the amount of
	// overlapping records preventing us from finding new ones.
	for i, payloads := range broadcasts {
		service := fmt.Sprintf("%s_%d", cloudMDNS.ClusterService, i)
		for _, info := range payloads {
			bytes, err := json.Marshal(info)
			if err != nil {
				return fmt.Errorf("Failed to marshal server info: %w", err)
			}

			server, err := cloudMDNS.NewBroadcast(info.LookupKey(), info.Address, s.Port, service, bytes)
			if err != nil {
				return err
			}

			s.servers = append(s.servers, server)
		}
	}

	return nil

}

// StopBroadcast stops the mDNS broadcast and token lookup, as we are initiating a new cluster.
func (s *ServiceHandler) StopBroadcast() error {
	for i, server := range s.servers {
		service := fmt.Sprintf("%s_%d", cloudMDNS.ClusterService, i)
		err := server.Shutdown()
		if err != nil {
			return fmt.Errorf("Failed to shut down %q server: %w", service, err)
		}
	}

	return nil
}

// RunConcurrent runs the given hook concurrently across all services.
// If microCloudFirst is true, then MicroCloud will have its hook run before the others.
func (s *ServiceHandler) RunConcurrent(microCloudFirst bool, lxdLast bool, f func(s Service) error) error {
	errors := make([]error, 0, len(s.Services))
	mut := sync.Mutex{}
	wg := sync.WaitGroup{}

	if microCloudFirst {
		for _, s := range s.Services {
			if s.Type() != types.MicroCloud {
				continue
			}

			err := f(s)
			if err != nil {
				return err
			}

			break
		}
	}

	for _, s := range s.Services {
		if microCloudFirst && s.Type() == types.MicroCloud {
			continue
		}

		if lxdLast && s.Type() == types.LXD {
			continue
		}

		wg.Add(1)
		go func(s Service) {
			defer wg.Done()
			err := f(s)
			if err != nil {
				mut.Lock()
				errors = append(errors, err)
				mut.Unlock()
				return
			}
		}(s)
	}

	// Wait for all queries to complete and check for any errors.
	wg.Wait()
	for _, err := range errors {
		if err != nil {
			return err
		}
	}

	if lxdLast {
		for _, s := range s.Services {
			if s.Type() != types.LXD {
				continue
			}

			err := f(s)
			if err != nil {
				return err
			}

			break
		}
	}

	return nil
}

// ServiceExists returns true if we can stat the unix socket in the state directory of the given service.
func ServiceExists(service types.ServiceType, stateDir string) bool {
	socketPath := filepath.Join(stateDir, "control.socket")
	if service == types.LXD {
		socketPath = filepath.Join(stateDir, "unix.socket")
	}

	_, err := os.Stat(socketPath)

	return err == nil
}
