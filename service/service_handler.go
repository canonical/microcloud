package service

import (
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/canonical/lxd/shared/api"

	"github.com/canonical/microcloud/microcloud/api/types"
	cloudClient "github.com/canonical/microcloud/microcloud/client"
)

const (
	// OVNPort is the default MicroOVN port.
	OVNPort int64 = 6443

	// CephPort is the default MicroCeph port.
	CephPort int64 = 7443

	// LXDPort is the default LXD port.
	LXDPort int64 = 8443

	// CloudPort is the default MicroCloud port.
	CloudPort int64 = 9443

	// CloudMulticastPort is the default MicroCloud multicast discovery port.
	CloudMulticastPort int64 = 9444
)

// Handler holds a set of stateful services.
type Handler struct {
	Services map[types.ServiceType]Service
	Name     string
	Port     int64

	sessionLock sync.RWMutex
	Session     *Session

	initMu  sync.RWMutex
	address string
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
		Services: servicesMap,
		Name:     name,
		address:  addr,
		Port:     CloudPort,
	}, nil
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

// StartSession starts a new local trust establishment session.
func (s *Handler) StartSession(role types.SessionRole, passphrase string, gw *cloudClient.WebsocketGateway) error {
	session, err := NewSession(role, passphrase, gw)
	if err != nil {
		return err
	}

	s.sessionLock.Lock()
	s.Session = session
	s.sessionLock.Unlock()

	return nil
}

// StopSession stops the current session started on this handler.
// If there isn't an active session it's a no-op.
func (s *Handler) StopSession(cause error) error {
	s.sessionLock.Lock()
	defer s.sessionLock.Unlock()

	if s.Session != nil {
		err := s.Session.Stop(cause)
		if err != nil {
			return fmt.Errorf("Failed to stop session: %w", err)
		}
	}

	return nil
}

// ActiveSession returns true if there is an active trust establishment session.
func (s *Handler) ActiveSession() bool {
	// Try to open a transaction in the current session.
	// If it succeeds there is an active session.
	err := s.SessionTransaction(true, func(session *Session) error {
		return nil
	})
	return err == nil
}

// SessionTransaction allows running f within the current handler's session.
// It allows running multiple operations on the handler's session struct without always
// checking if the session is still alive.
// Set readOnly to false if you don't modify the session.
// Set it to false if you intend to perform any modifications on the session.
func (s *Handler) SessionTransaction(readOnly bool, f func(session *Session) error) error {
	if readOnly {
		s.sessionLock.RLock()
		defer s.sessionLock.RUnlock()
	} else {
		s.sessionLock.Lock()
		defer s.sessionLock.Unlock()
	}

	if s.Session == nil || s.Session != nil && s.Session.Passphrase() == "" {
		return api.NewStatusError(http.StatusBadRequest, "No active session")
	}

	return f(s.Session)
}

// TemporaryTrustStore returns a copy of the trust establishment's session truststore.
func (s *Handler) TemporaryTrustStore() map[string]x509.Certificate {
	var trustStore = make(map[string]x509.Certificate, 0)

	// Ignore the error from the session and return the empty trust store instead.
	_ = s.SessionTransaction(true, func(session *Session) error {
		trustStore = session.TemporaryTrustStore()
		return nil
	})

	return trustStore
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

// Address gets the address used for the MicroCloud API.
func (s *Handler) Address() string {
	s.initMu.RLock()
	defer s.initMu.RUnlock()

	return s.address
}

// SetAddress sets the address used for the MicroCloud API.
func (s *Handler) SetAddress(addr string) {
	s.initMu.Lock()
	defer s.initMu.Unlock()

	s.address = addr
}
