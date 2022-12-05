package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/canonical/microcluster/state"
	"github.com/hashicorp/mdns"
	"github.com/lxc/lxd/shared/logger"

	cloudMDNS "github.com/canonical/microcloud/microcloud/mdns"
)

const (
	// CephPort is the efault MicroCeph port.
	CephPort int = 7443

	// LXDPort is the efault LXD port.
	LXDPort int = 8443

	// CloudPort is the efault MicroCloud port.
	CloudPort int = 9443
)

// ServiceHandler holds a set of services and an mdns server for communication between them.
type ServiceHandler struct {
	server      *mdns.Server
	tokenCancel context.CancelFunc

	Services map[ServiceType]Service
	Name     string
	Address  string
	Port     int
}

// ServiceType represents supported services.
type ServiceType string

const (
	// MicroCloud represents a MicroCloud service.
	MicroCloud ServiceType = "MicroCloud"

	// MicroCeph represents a MicroCeph service.
	MicroCeph ServiceType = "MicroCeph"

	// LXD represents a LXD service.
	LXD ServiceType = "LXD"
)

// NewServiceHandler creates a new ServiceHandler with a client for each of the given services.
func NewServiceHandler(name string, addr string, services ...Service) *ServiceHandler {
	servicesMap := make(map[ServiceType]Service, len(services))
	for _, service := range services {
		servicesMap[service.Type()] = service
	}

	return &ServiceHandler{
		Services: servicesMap,
		Name:     name,
		Address:  addr,
		Port:     CloudPort,
	}
}

// Start is run after the MicroCloud daemon has started. It will periodically check for join token broadcasts, and if
// found, will join all known services.
func (s *ServiceHandler) Start(state *state.State) error {
	var ctx context.Context
	ctx, s.tokenCancel = context.WithCancel(state.Context)

	var err error
	s.server, err = cloudMDNS.NewBroadcast(cloudMDNS.ClusterService, s.Name, s.Address, s.Port, nil)
	if err != nil {
		return err
	}

	cloudMDNS.LookupJoinToken(ctx, s.Name, func(tokens map[string]string) error {
		// Join MicroCloud first.
		service, ok := s.Services[MicroCloud]
		if !ok {
			return fmt.Errorf("Missing MicroCloud service")
		}

		token, ok := tokens[string(service.Type())]
		if !ok {
			return fmt.Errorf("Invalid service type %q", service.Type())
		}

		err := service.Join(token)
		if err != nil {
			return fmt.Errorf("Failed to join %q cluster: %w", service.Type(), err)
		}

		err = s.RunAsync(func(s Service) error {
			if s.Type() == MicroCloud {
				return nil
			}

			token, ok := tokens[string(s.Type())]
			if !ok {
				return fmt.Errorf("Invalid service type %q", s.Type())
			}

			err := s.Join(token)
			if err != nil {
				return fmt.Errorf("Failed to join %q cluster: %w", s.Type(), err)
			}

			return nil
		})
		if err != nil {
			return err
		}

		err = s.server.Shutdown()
		if err != nil {
			return fmt.Errorf("Failed to shutdown mdns server after joining the cluster: %w", err)
		}

		s.server, err = cloudMDNS.NewBroadcast(cloudMDNS.JoinedService, s.Name, s.Address, s.Port, nil)
		if err != nil {
			return err
		}

		timeAfter := time.After(time.Second * 5)
		go func() {
			for {
				select {
				case <-timeAfter:
					err := s.server.Shutdown()
					if err != nil {
						logger.Error("Failed to shutdown mdns server after joining the cluster", logger.Ctx{"error": err})
						return
					}
				default:
					// Sleep a bit so the loop doesn't push the CPU as hard.
					time.Sleep(100 * time.Millisecond)
				}
			}
		}()

		return nil
	})

	return nil
}

// Bootstrap stops the mDNS broadcast and token lookup, as we are initiating a new cluster.
func (s *ServiceHandler) Bootstrap(state *state.State) error {
	s.tokenCancel()
	err := s.server.Shutdown()
	if err != nil {
		return fmt.Errorf("Failed to shut down %q server: %w", cloudMDNS.ClusterService, err)
	}

	return nil
}

// RunAsync runs the given hook asynchronously across all services.
func (s *ServiceHandler) RunAsync(f func(s Service) error) error {
	errors := make([]error, 0, len(s.Services))
	mut := sync.Mutex{}
	wg := sync.WaitGroup{}
	for _, s := range s.Services {
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
