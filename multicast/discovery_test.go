package multicast

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type multicastSuite struct {
	suite.Suite
}

func TestMulticastSuite(t *testing.T) {
	suite.Run(t, new(multicastSuite))
}

func (m *multicastSuite) Test_Lookup() {
	cases := []struct {
		desc          string
		lookupVersion string
		responseInfo  ServerInfo
	}{
		{
			desc:          "System with matching version can be looked up",
			lookupVersion: "2.0",
			responseInfo: ServerInfo{
				Version: "2.0",
				Name:    "foo",
				Address: "1.2.3.4",
			},
		},
		{
			desc:          "System with maximum allowed server name length, IPv6 address and high version number can be looked up",
			lookupVersion: "142.0",
			responseInfo: ServerInfo{
				Version: "142.0",
				Name:    strings.Repeat("a", 255),
				Address: "fd42:c4cc:2e1d:132d:a216:3eff:fecd:9d15",
			},
		},
	}

	for _, c := range cases {
		m.T().Log(c.desc)

		// Use the loopback interface as it should always be there on any test system.
		discovery := NewDiscovery("lo", 9444)

		ctx, cancel := context.WithCancel(context.Background())
		err := discovery.Respond(ctx, c.responseInfo)
		m.Require().NoError(err)

		receivedInfo, err := discovery.Lookup(ctx, c.lookupVersion)
		m.Require().NoError(err)
		m.Require().Equal(&c.responseInfo, receivedInfo)

		// Stop the responder.
		cancel()
	}
}
