package main

import (
	"testing"

	lxdAPI "github.com/canonical/lxd/shared/api"

	"github.com/canonical/microcloud/microcloud/mdns"
	"github.com/canonical/microcloud/microcloud/service"
)

func newSystemWithNetworks(address string, networks []lxdAPI.NetworksPost) InitSystem {
	return InitSystem{
		ServerInfo: mdns.ServerInfo{
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
