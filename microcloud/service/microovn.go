package service

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/microcluster/client"
	"github.com/canonical/microcluster/microcluster"

	"github.com/canonical/microcloud/microcloud/api/types"
)

// OVNService is a MicroOVN service.
type OVNService struct {
	m *microcluster.MicroCluster

	name    string
	address string
	port    int
}

// NewOVNService creates a new MicroOVN service with a client attached.
func NewOVNService(ctx context.Context, name string, addr string, cloudDir string) (*OVNService, error) {
	proxy := func(r *http.Request) (*url.URL, error) {
		if !strings.HasPrefix(r.URL.Path, "/1.0/services/microovn") {
			r.URL.Path = "/1.0/services/microovn" + r.URL.Path
		}

		return shared.ProxyFromEnvironment(r)
	}

	client, err := microcluster.App(ctx, microcluster.Args{StateDir: cloudDir, Proxy: proxy})
	if err != nil {
		return nil, err
	}

	return &OVNService{
		m:       client,
		name:    name,
		address: addr,
		port:    OVNPort,
	}, nil
}

// Client returns a client to the OVN unix socket.
func (s OVNService) Client() (*client.Client, error) {
	return s.m.LocalClient()
}

// Bootstrap bootstraps the MicroOVN daemon on the default port.
func (s OVNService) Bootstrap() error {
	err := s.m.NewCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), 2*time.Minute)
	if err != nil {
		return err
	}

	for {
		select {
		case <-time.After(30 * time.Second):
			return fmt.Errorf("Timed out waiting for MicroOVN cluster to initialize")
		default:
			names, err := s.ClusterMembers()
			if err != nil {
				return err
			}

			if len(names) > 0 {
				return nil
			}
		}
	}
}

// IssueToken issues a token for the given peer.
func (s OVNService) IssueToken(peer string) (string, error) {
	return s.m.NewJoinToken(peer)
}

// Join joins a cluster with the given token.
func (s OVNService) Join(joinConfig JoinConfig) error {
	return s.m.JoinCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), joinConfig.Token, 5*time.Minute)
}

// ClusterMembers returns a map of cluster member names and addresses.
func (s OVNService) ClusterMembers() (map[string]string, error) {
	client, err := s.Client()
	if err != nil {
		return nil, err
	}

	members, err := client.GetClusterMembers(context.Background())
	if err != nil {
		return nil, err
	}

	genericMembers := make(map[string]string, len(members))
	for _, member := range members {
		genericMembers[member.Name] = member.Address.String()
	}

	return genericMembers, nil
}

// Type returns the type of Service.
func (s OVNService) Type() types.ServiceType {
	return types.MicroOVN
}

// Name returns the name of this Service instance.
func (s OVNService) Name() string {
	return s.name
}

// Address returns the address of this Service instance.
func (s OVNService) Address() string {
	return s.address
}

// Port returns the port of this Service instance.
func (s OVNService) Port() int {
	return s.port
}
