package service

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
	cephTypes "github.com/canonical/microceph/microceph/api/types"
	"github.com/canonical/microcluster/v3/microcluster"
	microTypes "github.com/canonical/microcluster/v3/microcluster/types"
	"github.com/gorilla/websocket"

	"github.com/canonical/microcloud/microcloud/api/types"
	cloudClient "github.com/canonical/microcloud/microcloud/client"
)

// CloudService is a MicroCloud service.
type CloudService struct {
	client *microcluster.MicroCluster

	name    string
	address string
	port    int64
	config  map[string]string
}

// JoinConfig represents configuration for cluster joining.
type JoinConfig struct {
	Token      string
	LXDConfig  []api.ClusterMemberConfigKey
	CephConfig []cephTypes.DisksPost
	OVNConfig  map[string]string
}

// Status represents information about a cluster member.
// It represents microcluster's internal Server type and implements a subset of it.
type Status struct {
	Name string `json:"name"    yaml:"name"`
}

// NewCloudService creates a new MicroCloud service with a client attached.
func NewCloudService(name string, addr string, dir string) (*CloudService, error) {
	client, err := microcluster.App(microcluster.Args{StateDir: dir})
	if err != nil {
		return nil, err
	}

	return &CloudService{
		client:  client,
		name:    name,
		address: addr,
		port:    CloudPort,
		config:  make(map[string]string),
	}, nil
}

// StartCloud launches the MicroCloud daemon with the appropriate hooks.
func (s *CloudService) StartCloud(ctx context.Context, args microcluster.DaemonArgs) error {
	return s.client.Start(ctx, args)
}

// Client returns a client to the MicroCloud unix socket.
func (s CloudService) Client() (microTypes.Client, error) {
	return s.client.LocalClient()
}

// Bootstrap bootstraps the MicroCloud daemon on the default port.
func (s CloudService) Bootstrap(ctx context.Context) error {
	err := s.client.NewCluster(ctx, s.name, util.CanonicalNetworkAddress(s.address, s.port), nil)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return errors.New("Timed out waiting for MicroCloud cluster to initialize")
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

// IssueToken issues a token for the given peer. Each token will last 5 minutes in case the system joins the cluster very slowly.
func (s CloudService) IssueToken(ctx context.Context, peer string) (string, error) {
	return s.client.NewJoinToken(ctx, peer, 5*time.Minute)
}

// DeleteToken deletes a token by its name.
func (s CloudService) DeleteToken(ctx context.Context, tokenName string, address string) error {
	var c microTypes.Client
	var err error
	if address != "" {
		c, err = s.client.RemoteClient(util.CanonicalNetworkAddress(address, CloudPort))
		if err != nil {
			return err
		}

		c, err = cloudClient.UseAuthProxy(c, types.MicroCloud, cloudClient.AuthConfig{})
	} else {
		c, err = s.client.LocalClient()
	}

	if err != nil {
		return err
	}

	return cloudClient.DeleteToken(ctx, tokenName, c)
}

// RemoteIssueToken issues a token for the given peer on a remote MicroCloud where we are authorized using mTLS.
func (s CloudService) RemoteIssueToken(ctx context.Context, clusterAddress string, peer string, serviceType types.ServiceType) (string, error) {
	c, err := s.client.RemoteClient(util.CanonicalNetworkAddress(clusterAddress, CloudPort))
	if err != nil {
		return "", err
	}

	c, err = cloudClient.UseAuthProxy(c, types.MicroCloud, cloudClient.AuthConfig{})
	if err != nil {
		return "", err
	}

	return cloudClient.RemoteIssueToken(ctx, c, serviceType, types.ServiceTokensPost{ClusterAddress: c.URL().Host, JoinerName: peer})
}

// Join joins a cluster with the given token.
func (s CloudService) Join(ctx context.Context, joinConfig JoinConfig) error {
	return s.client.JoinCluster(ctx, s.name, util.CanonicalNetworkAddress(s.address, s.port), joinConfig.Token, nil)
}

// remoteClient returns an https client for the given address:port.
// It picks the cluster certificate if none is provided to verify the remote.
func (s CloudService) remoteClient(cert *x509.Certificate, address string) (microTypes.Client, error) {
	var err error
	var client microTypes.Client

	canonicalAddress := util.CanonicalNetworkAddress(address, CloudPort)
	if cert != nil {
		client, err = s.client.RemoteClientWithCert(canonicalAddress, cert)
	} else {
		client, err = s.client.RemoteClient(canonicalAddress)
	}

	if err != nil {
		return nil, err
	}

	return client, nil
}

// RequestJoin sends the signal to initiate a join to the remote system, or timeout after a maximum of 5 min.
func (s CloudService) RequestJoin(ctx context.Context, name string, cert *x509.Certificate, joinConfig types.ServicesPut) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()

	var c microTypes.Client
	var err error
	if name == s.name {
		c, err = s.client.LocalClient()
		if err != nil {
			return err
		}
	} else {
		c, err = s.remoteClient(cert, joinConfig.Address)
		if err != nil {
			return err
		}

		c, err = cloudClient.UseAuthProxy(c, types.MicroCloud, cloudClient.AuthConfig{})
		if err != nil {
			return err
		}
	}

	return cloudClient.JoinServices(ctx, c, joinConfig)
}

// RequestJoinIntent send the intent to join the remote cluster.
func (s CloudService) RequestJoinIntent(ctx context.Context, clusterAddress string, conf cloudClient.AuthConfig, intent types.SessionJoinPost) (*x509.Certificate, error) {
	c, err := s.client.RemoteClientWithCert(util.CanonicalNetworkAddress(clusterAddress, CloudPort), conf.TLSServerCertificate)
	if err != nil {
		return nil, err
	}

	c, err = cloudClient.UseAuthProxy(c, types.MicroCloud, conf)
	if err != nil {
		return nil, err
	}

	return cloudClient.JoinIntent(ctx, c, intent)
}

// RemoteClusterMembers returns a map of cluster member names and addresses from the MicroCloud at the given address.
// Provide the certificate of the remote server for mTLS.
func (s CloudService) RemoteClusterMembers(ctx context.Context, cert *x509.Certificate, address string) (map[string]string, error) {
	client, err := s.remoteClient(cert, address)
	if err != nil {
		return nil, err
	}

	client, err = cloudClient.UseAuthProxy(client, types.MicroCloud, cloudClient.AuthConfig{})
	if err != nil {
		return nil, err
	}

	return clusterMembers(ctx, client)
}

// RemoteStatus returns the status of a remote member which doesn't have to be part of any cluster.
func (s CloudService) RemoteStatus(ctx context.Context, cert *x509.Certificate, address string) (*Status, error) {
	client, err := s.remoteClient(cert, address)
	if err != nil {
		return nil, err
	}

	client, err = cloudClient.UseAuthProxy(client, types.MicroCloud, cloudClient.AuthConfig{})
	if err != nil {
		return nil, err
	}

	status := Status{}
	err = client.Query(ctx, "GET", "core/1.0", nil, nil, &status)
	if err != nil {
		return nil, fmt.Errorf("Failed to get status: %w", err)
	}

	return &status, nil
}

// ClusterMembers returns a map of cluster member names and addresses.
func (s CloudService) ClusterMembers(ctx context.Context) (map[string]string, error) {
	client, err := s.client.LocalClient()
	if err != nil {
		return nil, err
	}

	return clusterMembers(ctx, client)
}

// clusterMembers returns a map of cluster member names and addresses.
func clusterMembers(ctx context.Context, c microTypes.Client) (map[string]string, error) {
	members, err := cloudClient.GetClusterMembers(ctx, c)
	if err != nil {
		return nil, err
	}

	genericMembers := make(map[string]string, len(members))
	for _, member := range members {
		genericMembers[member.Name] = member.Address.String()
	}

	return genericMembers, nil
}

// DeleteClusterMember removes the given cluster member from the service.
func (s CloudService) DeleteClusterMember(ctx context.Context, name string, force bool) error {
	return s.client.RemoveClusterMember(ctx, name, "", force)
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

// SetConfig sets the config of this Service instance.
func (s *CloudService) SetConfig(config map[string]string) {
	if s.config == nil {
		s.config = make(map[string]string)
	}

	for key, value := range config {
		s.config[key] = value
	}
}

// GetVersion gets the installed daemon version of the service, and returns an error if the version is not supported.
func (s CloudService) GetVersion(ctx context.Context) (string, error) {
	status, err := s.client.Status(ctx)
	if err != nil && api.StatusErrorCheck(err, http.StatusNotFound) {
		return "", fmt.Errorf("The installed version of %s is not supported", s.Type())
	}

	if err != nil {
		return "", err
	}

	return status.Version, nil
}

// IsInitialized returns whether the service is initialized.
func (s CloudService) IsInitialized(ctx context.Context) (bool, error) {
	err := s.client.Ready(ctx)
	if err != nil {
		return false, fmt.Errorf("Failed to wait for %s to get ready: %w", s.Type(), err)
	}

	status, err := s.client.Status(ctx)
	if err != nil {
		return false, fmt.Errorf("Failed to get %s status: %w", s.Type(), err)
	}

	return status.Ready, nil
}

// SupportsFeature checks if the specified API feature of this Service instance if supported.
func (s *CloudService) SupportsFeature(ctx context.Context, feature string) (bool, error) {
	server, err := s.client.Status(ctx)
	if err != nil {
		return false, fmt.Errorf("Failed to get MicroCloud server status while checking for features: %v", err)
	}

	if server.Extensions == nil {
		logger.Warnf("MicroCloud server does not expose API extensions")
		return false, nil
	}

	return server.Extensions.HasExtension(feature), nil
}

// ServerCert returns the local clusters server certificate.
func (s *CloudService) ServerCert() (*shared.CertInfo, error) {
	return s.client.FileSystem.ServerCert()
}

// ClusterCert returns the local clusters certificate.
func (s *CloudService) ClusterCert() (*shared.CertInfo, error) {
	return s.client.FileSystem.ClusterCert()
}

// StartSession starts a trust establishment session via the unix socket.
func (s *CloudService) StartSession(ctx context.Context, role string, sessionTimeout time.Duration) (*websocket.Conn, error) {
	c, err := s.client.LocalClient()
	if err != nil {
		return nil, err
	}

	return cloudClient.StartSession(ctx, c, role, sessionTimeout)
}

// RemoteClient returns a client targeting a remote MicroCloud.
func (s *CloudService) RemoteClient(cert *x509.Certificate, address string) (microTypes.Client, error) {
	c, err := s.remoteClient(cert, address)
	if err != nil {
		return nil, err
	}

	c, err = cloudClient.UseAuthProxy(c, types.MicroCloud, cloudClient.AuthConfig{})
	if err != nil {
		return nil, err
	}

	return c, nil
}

// Microcluster returns the internal app struct.
func (s *CloudService) Microcluster() *microcluster.MicroCluster {
	return s.client
}
