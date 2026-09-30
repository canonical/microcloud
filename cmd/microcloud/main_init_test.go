package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/canonical/lxd/shared"
	lxdAPI "github.com/canonical/lxd/shared/api"
	microTypes "github.com/canonical/microcluster/v3/microcluster/types"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/multicast"
	"github.com/canonical/microcloud/microcloud/service"
)

func newSystemWithNetworks(address string, networks []lxdAPI.NetworksPost) InitSystem {
	return InitSystem{
		ServerInfo: multicast.ServerInfo{
			Name:    "testSystem",
			Address: address,
		},
		Networks: networks,
	}
}

func newSystemWithUplinkNetConfig(address string, config map[string]string) InitSystem {
	return newSystemWithNetworks(address, []lxdAPI.NetworksPost{{
		Name: service.DefaultUplinkNetwork,
		Type: "physical",
		NetworkPut: lxdAPI.NetworkPut{
			Config: config,
		},
	}})
}

func newTestHandler(addr string, t *testing.T) *service.Handler {
	handler, err := service.NewHandler("testSystem", addr, "/tmp/microcloud_test_hander")
	if err != nil {
		t.Fatalf("Failed to create test service handler: %s", err)
	}

	return handler
}

func newTestSystemsMap(systems ...InitSystem) map[string]InitSystem {
	systemsMap := map[string]InitSystem{}

	for _, system := range systems {
		systemsMap[system.ServerInfo.Name] = system
	}

	return systemsMap
}

func ensureValidateSystemsPasses(handler *service.Handler, testSystems map[string]InitSystem, t *testing.T) {
	for testName, system := range testSystems {
		systems := newTestSystemsMap(system)
		cfg := initConfig{systems: systems, bootstrap: true}

		err := cfg.validateSystems(handler)
		if err != nil {
			t.Fatalf("Valid system %q failed validate: %s", testName, err)
		}
	}
}

func ensureValidateSystemsFails(handler *service.Handler, testSystems map[string]InitSystem, t *testing.T) {
	for testName, system := range testSystems {
		systems := newTestSystemsMap(system)
		cfg := initConfig{systems: systems, bootstrap: true}

		err := cfg.validateSystems(handler)
		if err == nil {
			t.Fatalf("Invalid system %q passed validation", testName)
		}
	}
}

func TestValidateSystemsIP4(t *testing.T) {
	address := "192.168.1.27"
	handler := newTestHandler(address, t)

	// Each entry in these maps is validated individually
	validSystems := map[string]InitSystem{
		"plainGateway": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv4.gateway": "10.234.0.1/16",
		}),
		"dns": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv4.gateway":    "10.234.0.1/16",
			"dns.nameservers": "1.1.1.1,8.8.8.8",
		}),
		"16Net": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv4.gateway":    "10.42.0.1/16",
			"ipv4.ovn.ranges": "10.42.1.1-10.42.5.255",
		}),
		"24Net": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv4.gateway":    "190.168.4.1/24",
			"ipv4.ovn.ranges": "190.168.4.50-190.168.4.60",
		}),
	}

	ensureValidateSystemsPasses(handler, validSystems, t)

	invalidSystems := map[string]InitSystem{
		"invalidNameservers1": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv4.gateway":    "10.234.0.1/16",
			"dns.nameservers": "8.8",
		}),
		"invalidNameservers2": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv4.gateway":    "10.234.0.1/16",
			"dns.nameservers": "8.8.8.128/23",
		}),
		"gatewayIsSubnetAddr": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv4.gateway": "192.168.28.0/24",
		}),
		"backwardsRange": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv4.gateway":    "10.42.0.1/16",
			"ipv4.ovn.ranges": "10.42.5.255-10.42.1.1",
		}),
		"rangesOutsideGateway": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv4.gateway":    "10.1.1.0/24",
			"ipv4.ovn.ranges": "10.2.2.50-10.2.2.100",
		}),
		"rangesContainGateway": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv4.gateway":    "192.168.1.1/24",
			"ipv4.ovn.ranges": "192.168.1.1-192.168.1.20",
		}),
		"rangesContainSystem": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv4.gateway":    "192.168.1.1/16",
			"ipv4.ovn.ranges": "192.168.1.20-192.168.1.30",
		}),
	}

	ensureValidateSystemsFails(handler, invalidSystems, t)
}

func TestValidateSystemsIP6(t *testing.T) {
	address := "fc00:feed:beef::bed1"
	handler := newTestHandler(address, t)

	validSystems := map[string]InitSystem{
		"plainGateway": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv6.gateway": "fc00:bad:feed::1/64",
		}),
		"dns": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv6.gateway":    "fc00:feed:f00d::1/64",
			"dns.nameservers": "2001:4860:4860::8888,2001:4860:4860::8844",
		}),
		"dnsMulti": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv6.gateway":    "fc00:feed:f00d::1/64",
			"dns.nameservers": "2001:4860:4860::8888,1.1.1.1",
		}),
		"64Net": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv6.gateway":    "fc00:bad:feed::1/64",
			"ipv6.ovn.ranges": "fc00:bad:feed::f-fc00:bad:feed::fffe",
		}),
	}

	ensureValidateSystemsPasses(handler, validSystems, t)

	invalidSystems := map[string]InitSystem{
		"invalidNameservers1": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv6.gateway":    "fc00:feed:f00d::1/64",
			"dns.nameservers": "8:8",
		}),
		"invalidNameservers2": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv6.gateway":    "fc00:feed:f00d::1/64",
			"dns.nameservers": "2001:4860:4860::8888/32",
		}),
		"gatewayIsSubnetAddr": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv6.gateway": "fc00:feed:f00d::0/64",
		}),
		"rangesOutsideGateway": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv6.gateway":    "fc00:feed:f00d::1/64",
			"ipv6.ovn.ranges": "fc00:feed:beef::f-fc00:feed:beef::fffe",
		}),
		"rangesContainGateway": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv6.gateway":    "fc00:feed:f00d::1/64",
			"ipv6.ovn.ranges": "fc00:feed:f00d::1-fc00:feed:f00d::ff",
		}),
		"rangesContainSystem": newSystemWithUplinkNetConfig(address, map[string]string{
			"ipv6.gateway":    "fc00:feed:beef::1/64",
			"ipv6.ovn.ranges": "fc00:feed:beef::bed1-fc00:feed:beef::bedf",
		}),
	}

	ensureValidateSystemsFails(handler, invalidSystems, t)
}

func TestValidateSystemsMultiSystem(t *testing.T) {
	localAddr := "10.23.1.20"
	handler := newTestHandler(localAddr, t)

	uplinkConfig := map[string]string{
		"ipv4.gateway":    "10.23.1.1/16",
		"ipv4.ovn.ranges": "10.23.1.50-10.23.1.100",
	}

	sys1 := newSystemWithUplinkNetConfig(localAddr, uplinkConfig)

	sys2 := newSystemWithUplinkNetConfig("10.23.1.72", uplinkConfig)
	sys2.ServerInfo.Name = "sys2"

	systems := newTestSystemsMap(sys1, sys2)
	cfg := initConfig{systems: systems, bootstrap: true}

	err := cfg.validateSystems(handler)
	if err == nil {
		t.Fatalf("sys2 with conflicting management IP and ipv4.ovn.ranges passed validation")
	}

	localAddr = "fc00:bad:feed::f00d"
	handler = newTestHandler(localAddr, t)

	uplinkConfig = map[string]string{
		"ipv6.gateway":    "fc00:bad:feed::1/64",
		"ipv6.ovn.ranges": "fc00:bad:feed::1-fc00:bad:feed::ff",
	}

	sys3 := newSystemWithUplinkNetConfig(localAddr, uplinkConfig)

	sys4 := newSystemWithUplinkNetConfig("fc00:bad:feed::60", uplinkConfig)
	sys4.ServerInfo.Name = "sys4"

	systems = newTestSystemsMap(sys3, sys4)
	cfg = initConfig{systems: systems, bootstrap: true}

	err = cfg.validateSystems(handler)
	if err == nil {
		t.Fatalf("sys4 with conflicting management IP and ipv6.ovn.ranges passed validation")
	}
}

// joinTracker records the tokens issued to peers and the peers that have joined.
// It checks that a peer's tokens are only issued once every peer before it has joined.
type joinTracker struct {
	t *testing.T

	mu     sync.Mutex
	tokens map[string]string // Token to the peer it was issued for.
	issued map[string]int    // Peer to the number of tokens issued for it.
	joined map[string]bool
}

// issue records a token for the peer and returns it.
func (j *joinTracker) issue(serviceType types.ServiceType, peer string) string {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.joined[peer] {
		j.t.Errorf("Issued %s token for peer %q after it joined", serviceType, peer)
	}

	token := fmt.Sprintf("%s-%s", serviceType, peer)
	j.tokens[token] = peer
	j.issued[peer]++

	return token
}

// members returns the local system and the peers that have joined.
func (j *joinTracker) members() map[string]string {
	j.mu.Lock()
	defer j.mu.Unlock()

	members := map[string]string{"local": "10.0.0.1"}
	for peer := range j.joined {
		members[peer] = peer
	}

	return members
}

// ServeHTTP answers the join request that waitForJoin sends to a peer.
// The peer is told apart by the tokens it was asked to join with.
func (j *joinTracker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fail := func(format string, args ...any) {
		j.t.Errorf(format, args...)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(lxdAPI.Response{Type: lxdAPI.ErrorResponse, Code: http.StatusBadRequest, Error: fmt.Sprintf(format, args...)})
	}

	path := "/" + string(types.APIVersion) + "/services"
	if r.Method != http.MethodPut || r.URL.Path != path {
		fail("Unexpected join request %s %s", r.Method, r.URL.Path)
		return
	}

	var cfg types.ServicesPut
	err := json.NewDecoder(r.Body).Decode(&cfg)
	if err != nil {
		fail("Failed to decode join request: %v", err)
		return
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	peer := ""
	for _, token := range cfg.Tokens {
		tokenPeer, ok := j.tokens[token.JoinToken]
		if !ok || (peer != "" && tokenPeer != peer) {
			fail("Join request carried %s token %q that was not issued for the joining peer", token.Service, token.JoinToken)
			return
		}

		peer = tokenPeer
	}

	if peer == "" {
		fail("Join request carried no tokens")
		return
	}

	if j.joined[peer] {
		fail("Peer %q joined twice", peer)
		return
	}

	// Only the joining peer and the peers that already joined may have tokens.
	for tokenPeer := range j.issued {
		if tokenPeer != peer && !j.joined[tokenPeer] {
			j.t.Errorf("Token for peer %q was issued before peer %q joined", tokenPeer, peer)
		}
	}

	j.joined[peer] = true

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(lxdAPI.Response{Type: lxdAPI.SyncResponse, Status: "Success", StatusCode: http.StatusOK, Metadata: json.RawMessage("null")})
}

// tokenTestService is a service whose tokens and cluster members are kept by a joinTracker.
type tokenTestService struct {
	service.Service

	serviceType types.ServiceType
	tracker     *joinTracker
}

func (s *tokenTestService) Type() types.ServiceType {
	return s.serviceType
}

func (s *tokenTestService) IssueToken(ctx context.Context, peer string) (string, error) {
	return s.tracker.issue(s.serviceType, peer), nil
}

func (s *tokenTestService) DeleteToken(ctx context.Context, tokenName string, address string) error {
	return nil
}

func (s *tokenTestService) ClusterMembers(ctx context.Context) (map[string]string, error) {
	return s.tracker.members(), nil
}

func TestAddPeersIssuesTokensBeforeEachJoin(t *testing.T) {
	serviceTypes := []types.ServiceType{types.LXD, types.MicroCeph, types.MicroOVN}
	peers := []string{"peer1", "peer2", "peer3", "peer4", "peer5"}

	tracker := &joinTracker{t: t, tokens: map[string]string{}, issued: map[string]int{}, joined: map[string]bool{}}

	// The peers answer join requests from a server presenting the cluster certificate in the state directory,
	// which is what MicroCloud verifies peers against once they are members.
	stateDir := t.TempDir()
	clusterCert, err := shared.KeyPairAndCA(stateDir, string(microTypes.ClusterCertificateName), shared.CertServer, shared.CertOptions{})
	if err != nil {
		t.Fatalf("Failed to generate cluster certificate: %v", err)
	}

	server := httptest.NewUnstartedServer(tracker)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{clusterCert.KeyPair()}}
	server.StartTLS()
	t.Cleanup(server.Close)

	cloud, err := service.NewCloudService("local", "10.0.0.1", stateDir)
	if err != nil {
		t.Fatalf("Failed to create MicroCloud service: %v", err)
	}

	handler := &service.Handler{Name: "local", Services: map[types.ServiceType]service.Service{types.MicroCloud: cloud}}
	for _, serviceType := range serviceTypes {
		handler.Services[serviceType] = &tokenTestService{serviceType: serviceType, tracker: tracker}
	}

	// The local system has bootstrapped every service, and the peers have already joined MicroCloud.
	localServices := map[types.ServiceType]map[string]string{types.MicroCloud: {"local": "10.0.0.1"}}
	for _, serviceType := range serviceTypes {
		localServices[serviceType] = map[string]string{"local": "10.0.0.1"}
	}

	cfg := initConfig{
		systems: map[string]InitSystem{"local": {ServerInfo: multicast.ServerInfo{Name: "local", Address: "10.0.0.1"}}},
		state:   map[string]service.SystemInformation{"local": {ExistingServices: localServices}},
	}

	// Every peer is reached at the test server, which tells them apart by their tokens.
	address := server.Listener.Addr().String()
	for _, peer := range peers {
		localServices[types.MicroCloud][peer] = address
		cfg.systems[peer] = InitSystem{ServerInfo: multicast.ServerInfo{Name: peer, Address: address}}
	}

	_, err = cfg.addPeers(handler)
	if err != nil {
		t.Fatalf("Failed to add peers: %v", err)
	}

	for _, peer := range peers {
		if !tracker.joined[peer] {
			t.Errorf("Peer %q did not join", peer)
		}

		if tracker.issued[peer] != len(serviceTypes) {
			t.Errorf("Issued %d tokens for peer %q, expected %d", tracker.issued[peer], peer, len(serviceTypes))
		}
	}

	if tracker.joined["local"] || tracker.issued["local"] != 0 {
		t.Errorf("Local system should not join or receive tokens")
	}
}
