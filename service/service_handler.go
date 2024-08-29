package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/microcluster/v2/state"
	"github.com/hashicorp/mdns"

	"github.com/canonical/microcloud/microcloud/api/types"
	cloudMDNS "github.com/canonical/microcloud/microcloud/mdns"
)

const (
	// OVNPort is the efault MicroOVN port.
	OVNPort int64 = 6443

	// CephPort is the efault MicroCeph port.
	CephPort int64 = 7443

	// LXDPort is the efault LXD port.
	LXDPort int64 = 8443

	// CloudPort is the efault MicroCloud port.
	CloudPort int64 = 9443
)

// Handler holds a set of services and an mdns server for communication between them.
type Handler struct {
	servers []*mdns.Server

	Services map[types.ServiceType]Service
	Name     string
	Address  string
	Port     int64

	AuthSecret string
}

// NewHandler creates a new Handler with a client for each of the given services.
func NewHandler(name string, addr string, stateDir string, services ...types.ServiceType) (*Handler, error) {
	servicesMap := make(map[types.ServiceType]Service, len(services))
	for _, serviceType := range services {
		var service Service
		var err error
		switch serviceType {
		case types.MicroCloud:
			service, err = NewCloudService(name, addr, stateDir)
		case types.MicroCeph:
			service, err = NewCephService(name, addr, stateDir)
		case types.MicroOVN:
			service, err = NewOVNService(name, addr, stateDir)
		case types.LXD:
			service, err = NewLXDService(name, addr, stateDir)
		}

		if err != nil {
			return nil, fmt.Errorf("Failed to create %q service: %w", serviceType, err)
		}

		servicesMap[serviceType] = service
	}

	return &Handler{
		servers:  []*mdns.Server{},
		Services: servicesMap,
		Name:     name,
		Address:  addr,
		Port:     CloudPort,
	}, nil
}

// Start is run after the MicroCloud daemon has started. It will periodically check for join token broadcasts, and if
// found, will join all known services.
func (s *Handler) Start(ctx context.Context, state state.State) error {
	// If we are already initialized, there's nothing to do.
	err := state.Database().IsOpen(context.Background())
	if err == nil {
		return nil
	}

	err = s.Services[types.LXD].(*LXDService).Restart(ctx, 30)
	if err != nil {
		logger.Error("Failed to restart LXD", logger.Ctx{"error": err})
	}

	s.AuthSecret, err = shared.RandomCryptoString()
	if err != nil {
		return err
	}

	return s.Broadcast()
}

// Broadcast broadcasts service information over mDNS.
func (s *Handler) Broadcast() error {
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
		info.Interface = net.Interface.Name

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

			iface, err := net.InterfaceByName(info.Interface)
			if err != nil {
				return err
			}

			if s.Port < 1 || s.Port > math.MaxUint16 {
				return fmt.Errorf("Port number for service %q (%q) is out of range", s.Name, s.Port)
			}

			server, err := cloudMDNS.NewBroadcast(info.LookupKey(), iface, info.Address, int(s.Port), service, bytes)
			if err != nil {
				return err
			}

			s.servers = append(s.servers, server)
		}
	}

	return nil
}

// StopBroadcast stops the mDNS broadcast and token lookup, as we are initiating a new cluster.
func (s *Handler) StopBroadcast() error {
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
// If firstService or lastService are empty strings, they will be ignored and all services will run concurrently.
func (s *Handler) RunConcurrent(firstService types.ServiceType, lastService types.ServiceType, f func(s Service) error) error {
	errors := make([]error, 0, len(s.Services))
	mut := sync.Mutex{}
	wg := sync.WaitGroup{}

	first, ok := s.Services[firstService]
	if ok {
		err := f(first)
		if err != nil {
			return err
		}
	}

	for _, s := range s.Services {
		if s.Type() == firstService || s.Type() == lastService {
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

	last, ok := s.Services[lastService]
	if ok {
		err := f(last)
		if err != nil {
			return err
		}
	}

	return nil
}

// Exists returns true if we can stat the unix socket in the state directory of the given service.
func Exists(service types.ServiceType, stateDir string) bool {
	socketPath := filepath.Join(stateDir, "control.socket")
	if service == types.LXD {
		socketPath = filepath.Join(stateDir, "unix.socket")
	}

	_, err := os.Stat(socketPath)

	return err == nil
}
