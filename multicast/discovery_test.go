package multicast

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

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
		lookupIface   string
		lookupPort    int64
		responseInfo  ServerInfo
		lookupErr     error
		lookupTimeout time.Duration
		modifier      func(server *Discovery)
	}{
		{
			desc:          "System with matching version can be looked up",
			lookupVersion: "2.0",
			lookupIface:   "lo",
			lookupPort:    9444,
			responseInfo: ServerInfo{
				Version: "2.0",
				Name:    "foo",
				Address: "1.2.3.4",
			},
		},
		{
			desc:          "System with maximum allowed server name length, IPv6 address and high version number can be looked up",
			lookupVersion: "142.0",
			lookupIface:   "lo",
			lookupPort:    9444,
			responseInfo: ServerInfo{
				Version: "142.0",
				Name:    strings.Repeat("a", 255),
				Address: "fd42:c4cc:2e1d:132d:a216:3eff:fecd:9d15",
			},
		},
		{
			desc:        "Cannot lookup system if invalid interface is given",
			lookupIface: "invalid-interface",
			lookupErr:   fmt.Errorf(`Failed to resolve lookup interface "invalid-interface": route ip+net: no such network interface`),
		},
		{
			desc:          "Cannot lookup system if the responder is offline",
			lookupVersion: "2.0",
			lookupIface:   "lo",
			lookupPort:    9444,
			responseInfo: ServerInfo{
				Version: "2.0",
				Name:    "foo",
				Address: "1.2.3.4",
			},
			lookupTimeout: 500 * time.Microsecond,
			modifier: func(server *Discovery) {
				_ = server.StopResponder()
			},
			lookupErr: fmt.Errorf("Failed to read from multicast network endpoint: Timeout exceeded"),
		},
		{
			desc:          "Cannot lookup system if the responder uses a different version",
			lookupVersion: "3.0",
			lookupIface:   "lo",
			lookupPort:    9444,
			responseInfo: ServerInfo{
				Version: "2.0",
				Name:    "foo",
				Address: "1.2.3.4",
			},
			lookupTimeout: 500 * time.Microsecond,
			lookupErr:     fmt.Errorf("Failed to read from multicast network endpoint: Timeout exceeded"),
		},
	}

	for _, c := range cases {
		m.T().Log(c.desc)

		// Use the loopback interface as it should always be there on any test system.
		discovery := NewDiscovery("lo", 9444)

		err := discovery.Respond(context.Background(), c.responseInfo)
		m.Require().NoError(err)

		if c.modifier != nil {
			c.modifier(discovery)
		}

		testDiscovery := NewDiscovery(c.lookupIface, c.lookupPort)

		ctx := context.Background()
		if c.lookupTimeout > 0 {
			ctx, _ = context.WithTimeoutCause(ctx, c.lookupTimeout, fmt.Errorf("Timeout exceeded"))
		}

		receivedInfo, err := testDiscovery.Lookup(ctx, c.lookupVersion)
		if c.lookupErr == nil {
			m.Require().NoError(err)
			m.Require().Equal(&c.responseInfo, receivedInfo)
		} else {
			m.Require().Error(err)
			m.Require().Equal(c.lookupErr.Error(), err.Error())
		}

		// Stop the responder.
		err = discovery.StopResponder()
		m.Require().NoError(err)
	}
}
