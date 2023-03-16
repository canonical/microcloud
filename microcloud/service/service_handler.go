package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/canonical/microcluster/state"
	"github.com/hashicorp/mdns"
	"github.com/lxc/lxd/shared"

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

func networkInterfaceAddresses() (map[string][]string, error) {
	addrsByIface := map[string][]string{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("Failed to get network interfaces: %w", err)
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		allAddrs := make([]string, 0, len(addrs))
		if len(addrs) == 0 {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			if !ipNet.IP.IsGlobalUnicast() {
				continue
			}

			allAddrs = append(allAddrs, ipNet.IP.String())
		}

		if len(allAddrs) != 0 {
			addrsByIface[iface.Name] = allAddrs
		}
	}

	return addrsByIface, nil
}

// Start is run after the MicroCloud daemon has started. It will periodically check for join token broadcasts, and if
// found, will join all known services.
func (s *ServiceHandler) Start(state *state.State) error {
	// If we are already initialized, there's nothing to do.
	if state.Database.IsOpen() {
		return nil
	}

	services := make([]types.ServiceType, 0, len(s.Services))
	for service := range s.Services {
		services = append(services, service)
	}

	var err error
	s.AuthSecret, err = shared.RandomCryptoString()
	if err != nil {
		return err
	}

	ifaces, err := networkInterfaceAddresses()
	if err != nil {
		return err
	}

	info := cloudMDNS.ServerInfo{
		Version:    cloudMDNS.Version,
		Name:       s.Name,
		Services:   services,
		Interfaces: ifaces,
		AuthSecret: s.AuthSecret,
	}

	for _, addrs := range info.Interfaces {
		for _, addr := range addrs {
			info.Address = addr
			bytes, err := json.Marshal(info)
			if err != nil {
				return fmt.Errorf("Failed to marshal server info: %w", err)
			}

			server, err := cloudMDNS.NewBroadcast(s.Name, addr, s.Port, bytes)
			if err != nil {
				return err
			}

			s.servers = append(s.servers, server)
		}
	}

	return nil
}

// Bootstrap stops the mDNS broadcast and token lookup, as we are initiating a new cluster.
func (s *ServiceHandler) Bootstrap(state *state.State) error {
	for _, server := range s.servers {
		err := server.Shutdown()
		if err != nil {
			return fmt.Errorf("Failed to shut down %q server: %w", cloudMDNS.ClusterService, err)
		}
	}

	return nil
}

// RunConcurrent runs the given hook concurrently across all services.
// If microCloudFirst is true, then MicroCloud will have its hook run before the others.
func (s *ServiceHandler) RunConcurrent(microCloudFirst bool, f func(s Service) error) error {
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
