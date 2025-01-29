package service

import (
	"testing"

	"github.com/canonical/lxd/shared/api"
	"github.com/stretchr/testify/suite"
)

type networkInterfaceSuite struct {
	suite.Suite
}

func TestNetworkInterfaceSuite(t *testing.T) {
	suite.Run(t, new(networkInterfaceSuite))
}

func (s *versionSuite) Test_defaultNetworkInterfacesFilter() {
	cases := []struct {
		desc     string
		network  api.Network
		state    *api.NetworkState
		filtered bool
	}{
		{
			desc: "Valid interface",
			network: api.Network{
				Name: "eth0",
				Type: "physical",
			},
			state: &api.NetworkState{
				State: "up",
			},
			filtered: true,
		},
		{
			desc: "Valid bridge",
			network: api.Network{
				Name: "br-valid",
				Type: "bridge",
			},
			state: &api.NetworkState{
				State: "up",
			},
			filtered: true,
		},
		{
			desc: "Invalid managed interface",
			network: api.Network{
				Name:    "eth0",
				Type:    "physical",
				Managed: true,
			},
			filtered: false,
		},
		{
			desc: "Invalid down interface",
			network: api.Network{
				Name: "eth0",
				Type: "physical",
			},
			state: &api.NetworkState{
				State: "down",
			},
			filtered: false,
		},
		{
			desc: "Invalid managed bridge",
			network: api.Network{
				Name:    "br-valid",
				Type:    "bridge",
				Managed: true,
			},
			filtered: false,
		},
		{
			desc: "Invalid down bridge",
			network: api.Network{
				Name: "br-valid",
				Type: "bridge",
			},
			state: &api.NetworkState{
				State: "down",
			},
			filtered: false,
		},
	}

	for i, c := range cases {
		s.T().Logf("%d: %s", i, c.desc)

		filtered := defaultNetworkInterfacesFilter(c.network, c.state)
		s.Equal(c.filtered, filtered)
	}
}

func (s *versionSuite) Test_ovnNetworkInterfacesFilter() {
	cases := []struct {
		desc     string
		network  api.Network
		state    *api.NetworkState
		filtered bool
	}{
		{
			desc: "Valid physical interface",
			network: api.Network{
				Name: "eth0",
				Type: "physical",
			},
			state: &api.NetworkState{
				Type: "broadcast",
			},
			filtered: true,
		},
		{
			desc: "Valid bridge interface",
			network: api.Network{
				Name: "br-valid",
				Type: "bridge",
			},
			state: &api.NetworkState{
				Type: "broadcast",
			},
			filtered: true,
		},
		{
			desc: "Valid bond interface",
			network: api.Network{
				Name: "bond0",
				Type: "bond",
			},
			state: &api.NetworkState{
				Type: "broadcast",
			},
			filtered: true,
		},
		{
			desc: "Valid VLAN interface",
			network: api.Network{
				Name: "vlan0",
				Type: "vlan",
			},
			state: &api.NetworkState{
				Type: "broadcast",
			},
			filtered: true,
		},
		{
			desc: "Invalid interface type",
			network: api.Network{
				Name: "invalid0",
				Type: "invalid",
			},
			filtered: false,
		},
		{
			desc: "Invalid physical interface type",
			network: api.Network{
				Name: "lo",
				Type: "physical",
			},
			state: &api.NetworkState{
				Type: "loopback",
			},
			filtered: false,
		},
	}

	for i, c := range cases {
		s.T().Logf("%d: %s", i, c.desc)

		filtered := ovnNetworkInterfacesFilter(c.network, c.state)
		s.Equal(c.filtered, filtered)
	}
}
