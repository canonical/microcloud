package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	cephTypes "github.com/canonical/microceph/microceph/api/types"
	"github.com/canonical/microcluster/config"
	"github.com/canonical/microcluster/microcluster"
	"github.com/canonical/microcluster/rest"
	"github.com/canonical/microcluster/state"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/client"
)

// CloudService is a MicroCloud service.
type CloudService struct {
	client *microcluster.MicroCluster

	name    string
	address string
	port    int64
}

// JoinConfig represents configuration for cluster joining.
type JoinConfig struct {
	Token      string
	LXDConfig  []api.ClusterMemberConfigKey
	CephConfig []cephTypes.DisksPost
}

// NewCloudService creates a new MicroCloud service with a client attached.
func NewCloudService(ctx context.Context, name string, addr string, dir string, verbose bool, debug bool) (*CloudService, error) {
	client, err := microcluster.App(ctx, microcluster.Args{StateDir: dir, ListenPort: strconv.FormatInt(CloudPort, 10), Debug: debug, Verbose: verbose})
	if err != nil {
		return nil, err
	}

	return &CloudService{
		client:  client,
		name:    name,
		address: addr,
		port:    CloudPort,
	}, nil
}

// StartCloud launches the MicroCloud daemon with the appropriate hooks.
func (s *CloudService) StartCloud(service *Handler, endpoints []rest.Endpoint) error {
	return s.client.Start(endpoints, nil, &config.Hooks{
		OnBootstrap: func(s *state.State) error { return service.StopBroadcast() },
		PostJoin:    func(s *state.State) error { return service.StopBroadcast() },
		OnStart:     service.Start,
	})
}

// Bootstrap bootstraps the MicroCloud daemon on the default port.
func (s CloudService) Bootstrap(ctx context.Context) error {
	err := s.client.NewCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), 2*time.Minute)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("Timed out waiting for MicroCloud cluster to initialize")
		default:
			names, err := s.ClusterMembers(ctx)
			if err != nil {
				return err
			}

			if len(names) > 0 {
				return nil
			}

			time.Sleep(time.Second)
		}
	}
}

// IssueToken issues a token for the given peer.
func (s CloudService) IssueToken(ctx context.Context, peer string) (string, error) {
	return s.client.NewJoinToken(peer)
}

// Join joins a cluster with the given token.
func (s CloudService) Join(ctx context.Context, joinConfig JoinConfig) error {
	return s.client.JoinCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), joinConfig.Token, 5*time.Minute)
}

// RequestJoin sends the signal to initiate a join to the remote system, or timeout after a maximum of 5 min.
func (s CloudService) RequestJoin(ctx context.Context, secret string, name string, joinConfig types.ServicesPut) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()

	c, err := s.client.RemoteClient(util.CanonicalNetworkAddress(joinConfig.Address, CloudPort))
	if err != nil {
		return err
	}

	c.Client.Client.Transport = &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		DisableKeepAlives: true,
		Proxy: func(r *http.Request) (*url.URL, error) {
			r.Header.Set("X-MicroCloud-Auth", secret)

			return shared.ProxyFromEnvironment(r)
		},
	}

	return client.JoinServices(ctx, c, joinConfig)
}

// ClusterMembers returns a map of cluster member names and addresses.
func (s CloudService) ClusterMembers(ctx context.Context) (map[string]string, error) {
	client, err := s.client.LocalClient()
	if err != nil {
		return nil, err
	}

	members, err := client.GetClusterMembers(ctx)
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
func (s CloudService) Type() types.ServiceType {
	return types.MicroCloud
}

// Name returns the name of this Service instance.
func (s CloudService) Name() string {
	return s.name
}

// Address returns the address of this Service instance.
func (s CloudService) Address() string {
	return s.address
}

// Port returns the port of this Service instance.
func (s CloudService) Port() int64 {
	return s.port
}
