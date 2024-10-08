package main

import (
	"errors"
	"testing"

	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/units"
	"github.com/stretchr/testify/suite"
)

type preseedSuite struct {
	suite.Suite
}

func TestPreseedSuite(t *testing.T) {
	suite.Run(t, new(preseedSuite))
}

func (s *preseedSuite) Test_preseedValidateInvalid() {
	cases := []struct {
		desc    string
		preseed Preseed

		addErr bool
		err    error
	}{
		{
			desc: "No systems",
			preseed: Preseed{
				Systems: nil,
				OVN:     InitNetwork{IPv4Gateway: "10.0.0.1/24", IPv4Range: "10.0.0.100-10.0.0.254", IPv6Gateway: "cafe::1/64"},
				Storage: StorageFilter{
					Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
					Ceph:  []DiskFilter{{Find: "def", FindMin: 0, FindMax: 3, Wipe: false}},
				},
			},
			addErr: true,
			err:    errors.New("No systems given"),
		},
		{
			desc: "Duplicate systems",
			preseed: Preseed{
				SessionPassphrase: "foo",
				Initiator:         "n1",
				Systems:           []System{{Name: "n1"}, {Name: "n1"}},
			},
			addErr: true,
			err:    errors.New(`Duplicate system name "n1"`),
		},
		{
			desc: "Single node preseed",
			preseed: Preseed{
				Initiator: "n1",
				Systems:   []System{{Name: "n1", UplinkInterface: "eth0", Storage: InitStorage{}}},
				OVN:       InitNetwork{IPv4Gateway: "10.0.0.1/24", IPv4Range: "10.0.0.100-10.0.0.254", IPv6Gateway: "cafe::1/64"},
				Storage: StorageFilter{
					Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
					Ceph:  []DiskFilter{{Find: "def", FindMin: 1, FindMax: 3, Wipe: false}},
				},
			},
			addErr: false,
			err:    nil,
		},
		{
			desc: "Missing session passphrase",
			preseed: Preseed{
				Initiator: "n1",
				Systems:   []System{{Name: "n1"}, {Name: "n2"}},
			},
			addErr: true,
			err:    errors.New(`Missing session passphrase`),
		},
		{
			desc: "Missing initiator's name or address",
			preseed: Preseed{
				Systems: []System{{Name: "n1"}},
			},
			addErr: true,
			err:    errors.New(`Missing initiator's name or address`),
		},
		{
			desc: "Cannot provide both the initiator's name and address",
			preseed: Preseed{
				Initiator:        "n1",
				InitiatorAddress: "1.0.0.1",
				Systems:          []System{{Name: "n1"}},
			},
			addErr: true,
			err:    errors.New(`Cannot provide both the initiator's name and address`),
		},
		{
			desc: "Cannot provide both the initiator's address and lookup subnet",
			preseed: Preseed{
				InitiatorAddress: "1.0.0.1",
				LookupSubnet:     "1.0.0.0/24",
				Systems:          []System{{Name: "n1"}},
			},
			addErr: true,
			err:    errors.New(`Cannot provide both the initiator's address and lookup subnet`),
		},
		{
			desc: "Cannot provide both system address and lookup subnet",
			preseed: Preseed{
				Initiator:    "n1",
				LookupSubnet: "1.0.0.0/24",
				Systems:      []System{{Name: "n1", Address: "1.0.0.1"}},
			},
			addErr: true,
			err:    errors.New(`Cannot provide both the address for system "n1" and the lookup subnet`),
		},
		{
			desc: "Missing initiator address if one system has an address",
			preseed: Preseed{
				Initiator: "n1",
				Systems:   []System{{Name: "n1", Address: "1.0.0.1"}},
			},
			addErr: true,
			err:    errors.New(`Missing the initiator's address as system "n1" has an address`),
		},
		{
			desc: "Missing listen address",
			preseed: Preseed{
				SessionPassphrase: "foo",
				InitiatorAddress:  "1.0.0.1",
				Systems:           []System{{Name: "n1"}, {Name: "n2", Address: "1.0.0.2"}},
				Storage:           StorageFilter{},
			},
			addErr: true,
			err:    errors.New(`Missing address for system "n1" when the initiator's address is set`),
		},
		{
			desc: "Systems missing name",
			preseed: Preseed{
				SessionPassphrase: "foo",
				InitiatorAddress:  "1.0.0.1",
				Systems:           []System{{Name: "", UplinkInterface: "eth0", Address: "1.0.0.1"}, {Name: "n2", UplinkInterface: "eth0", Address: "1.0.0.2"}},
				OVN:               InitNetwork{IPv4Gateway: "10.0.0.1/24", IPv4Range: "10.0.0.100-10.0.0.254", IPv6Gateway: "cafe::1/64"},
				Storage: StorageFilter{
					Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
					Ceph:  []DiskFilter{{Find: "def", FindMin: 0, FindMax: 3, Wipe: false}},
				},
			},
			addErr: true,
			err:    errors.New("Missing system name"),
		},
		{
			desc: "FindMin too low for ceph filter",
			preseed: Preseed{
				SessionPassphrase: "foo",
				InitiatorAddress:  "1.0.0.1",
				Systems:           []System{{Name: "n1", Address: "1.0.0.1"}, {Name: "n2", Address: "1.0.0.2"}},
				Storage: StorageFilter{
					Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
					Ceph:  []DiskFilter{{Find: "def", FindMin: 0, FindMax: 3, Wipe: false}},
				},
			},
			addErr: true,
			err:    errors.New("Remote storage filter cannot be defined with find_min less than 1"),
		},
		{
			desc: "Ceph direct selection (3) with more systems (4)",
			preseed: Preseed{
				SessionPassphrase: "foo",
				InitiatorAddress:  "1.0.0.1",
				Systems: []System{
					{Name: "n1", Address: "1.0.0.1", Storage: InitStorage{Ceph: []DirectStorage{{Path: "def"}}}},
					{Name: "n2", Address: "1.0.0.2", Storage: InitStorage{Ceph: []DirectStorage{{Path: "def"}}}},
					{Name: "n3", Address: "1.0.0.3", Storage: InitStorage{Ceph: []DirectStorage{{Path: "def"}}}},
					{Name: "n4", Address: "1.0.0.4"}},
			},
			addErr: false,
			err:    nil,
		},
		{
			desc: "Minimum ceph direct selection (1) with more systems (4)",
			preseed: Preseed{
				SessionPassphrase: "foo",
				InitiatorAddress:  "1.0.0.1",
				Systems: []System{
					{Name: "n1", Address: "1.0.0.1", Storage: InitStorage{Ceph: []DirectStorage{{Path: "def"}}}},
					{Name: "n2", Address: "1.0.0.2"},
					{Name: "n3", Address: "1.0.0.3"},
					{Name: "n4", Address: "1.0.0.4"}},
			},
			addErr: false,
			err:    nil,
		},
		{
			desc: "Incomplete zfs direct selection",
			preseed: Preseed{
				SessionPassphrase: "foo",
				InitiatorAddress:  "1.0.0.1",
				Systems:           []System{{Name: "n1", Address: "1.0.0.1", Storage: InitStorage{Local: DirectStorage{Path: "def"}}}, {Name: "n2", Address: "1.0.0.2", Storage: InitStorage{Local: DirectStorage{Path: "def"}}}, {Name: "n3", Address: "1.0.0.3"}},
			},
			addErr: true,
			err:    errors.New("Some systems are missing local storage disks"),
		},
		{
			desc: "Invalid zfs filter constraint",
			preseed: Preseed{
				SessionPassphrase: "foo",
				InitiatorAddress:  "1.0.0.1",
				Systems:           []System{{Name: "n1", Address: "1.0.0.1"}, {Name: "n2", Address: "1.0.0.2"}},
				Storage: StorageFilter{
					Local: []DiskFilter{{Find: "abc", FindMin: 3, FindMax: 2, Wipe: false}},
				},
			},
			addErr: true,
			err:    errors.New("Invalid local storage filter constraints find_max (2) larger than find_min (3)"),
		},
		{
			desc: "Invalid zfs filter value",
			preseed: Preseed{
				SessionPassphrase: "foo",
				InitiatorAddress:  "1.0.0.1",
				Systems:           []System{{Name: "n1", Address: "1.0.0.1"}, {Name: "n2", Address: "1.0.0.2"}},
				Storage: StorageFilter{
					Local: []DiskFilter{{Find: "", FindMin: 3, FindMax: 2, Wipe: false}},
				},
			},
			addErr: true,
			err:    errors.New("Received empty local disk filter"),
		},
		{
			desc: "Invalid ceph filter min > max",
			preseed: Preseed{
				SessionPassphrase: "foo",
				InitiatorAddress:  "1.0.0.1",
				Systems:           []System{{Name: "n1", Address: "1.0.0.1"}, {Name: "n2", Address: "1.0.0.2"}, {Name: "n3", Address: "1.0.0.3"}},
				Storage: StorageFilter{
					Ceph: []DiskFilter{{Find: "def", FindMin: 4, FindMax: 3, Wipe: false}},
				},
			},
			addErr: true,
			err:    errors.New("Invalid remote storage filter constraints find_max (3) must be larger than find_min (4)"),
		},
		{
			desc: "Invalid ceph filter constraints",
			preseed: Preseed{
				SessionPassphrase: "foo",
				InitiatorAddress:  "1.0.0.1",
				Systems:           []System{{Name: "n1", Address: "1.0.0.1"}, {Name: "n2", Address: "1.0.0.2"}, {Name: "n3", Address: "1.0.0.3"}},
				Storage: StorageFilter{
					Ceph: []DiskFilter{{Find: "", FindMin: 4, FindMax: 3, Wipe: false}},
				},
			},
			addErr: true,
			err:    errors.New("Received empty remote disk filter"),
		},
		{
			desc: "Systems missing interface",
			preseed: Preseed{
				SessionPassphrase: "foo",
				InitiatorAddress:  "1.0.0.1",
				Systems:           []System{{Name: "n1", Address: "1.0.0.1", UplinkInterface: ""}, {Name: "n2", Address: "1.0.0.2", UplinkInterface: "eth0"}, {Name: "n3", Address: "1.0.0.3", UplinkInterface: "eth0"}},
				OVN:               InitNetwork{IPv4Gateway: "10.0.0.1/24", IPv4Range: "10.0.0.100-10.0.0.254", IPv6Gateway: "cafe::1/64"},
				Storage: StorageFilter{
					Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
					Ceph:  []DiskFilter{{Find: "def", FindMin: 3, FindMax: 3, Wipe: false}},
				},
			},
			addErr: true,
			err:    errors.New("Some systems are missing an uplink interface"),
		},
		{
			desc: "OVN IPv4 Ranges with no gateway",
			preseed: Preseed{
				SessionPassphrase: "foo",
				InitiatorAddress:  "1.0.0.1",
				Systems:           []System{{Name: "n1", Address: "1.0.0.1", UplinkInterface: "eth0"}, {Name: "n2", Address: "1.0.0.2", UplinkInterface: "eth0"}, {Name: "n3", Address: "1.0.0.3", UplinkInterface: "eth0"}},
				OVN:               InitNetwork{IPv4Range: "10.0.0.100-10.0.0.254", IPv6Gateway: "cafe::1/64"},
				Storage: StorageFilter{
					Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
					Ceph:  []DiskFilter{{Find: "def", FindMin: 3, FindMax: 3, Wipe: false}},
				},
			},
			addErr: true,
			err:    errors.New("Cannot specify IPv4 range without IPv4 gateway"),
		},
		{
			desc: "Invalid OVN IPv4 Ranges",
			preseed: Preseed{
				SessionPassphrase: "foo",
				InitiatorAddress:  "1.0.0.1",
				Systems:           []System{{Name: "n1", Address: "1.0.0.1", UplinkInterface: "eth0"}, {Name: "n2", Address: "1.0.0.2", UplinkInterface: "eth0"}, {Name: "n3", Address: "1.0.0.3", UplinkInterface: "eth0"}},
				OVN:               InitNetwork{IPv4Gateway: "10.0.0.1/24", IPv4Range: "10.0.0.100,10.0.0.254", IPv6Gateway: "cafe::1/64"},
				Storage: StorageFilter{
					Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
					Ceph:  []DiskFilter{{Find: "def", FindMin: 3, FindMax: 3, Wipe: false}},
				},
			},
			addErr: true,
			err:    errors.New("Invalid IPv4 range (must be of the form <ip>-<ip>)"),
		},
	}

	s.T().Log("Preseed init missing local system")
	p := Preseed{SessionPassphrase: "foo", InitiatorAddress: "1.0.0.1", Systems: []System{{Name: "B", Address: "1.0.0.1"}, {Name: "C", Address: "1.0.0.2"}}}
	err := p.validate("A", true)
	s.EqualError(err, "Local MicroCloud must be included in the list of systems when initializing")

	for _, c := range cases {
		s.T().Log(c.desc)

		err := c.preseed.validate("n1", true)
		if c.err == nil {
			s.NoError(err)
		} else {
			s.EqualError(err, c.err.Error())
		}

		s.T().Logf("%s in add mode", c.desc)
		err = c.preseed.validate("n0", false)
		if c.addErr {
			s.EqualError(err, c.err.Error())
		} else {
			s.NoError(err)
		}
	}
}

func (s *preseedSuite) Test_preseedMatchDisksMemory() {
	unit1, err := units.ParseByteSizeString("1MiB")
	s.NoError(err)

	unit2, err := units.ParseByteSizeString("2MiB")
	s.NoError(err)

	disks := []api.ResourcesStorageDisk{{Size: uint64(unit1)}, {Size: uint64(unit2)}}
	filter := DiskFilter{Find: "size == 1MiB"}

	results, err := filter.Match(disks)
	s.NoError(err)

	s.Equal(len(results), 1)
	s.Equal(results[0], disks[0])
}

// Tests that ReuseExistingClusters only works when initializing, not when growing the cluster.
func (s *preseedSuite) Test_restrictClusterReuse() {
	p := Preseed{SessionPassphrase: "foo", Initiator: "B", ReuseExistingClusters: true, Systems: []System{{Name: "B"}, {Name: "C"}}}

	s.NoError(p.validate("B", true))

	s.Error(p.validate("A", false))

	p.ReuseExistingClusters = false
	s.NoError(p.validate("A", false))
}

func (s *preseedSuite) Test_isInitiator() {
	cases := []struct {
		desc        string
		preseed     Preseed
		name        string
		address     string
		isInitiator bool
	}{
		{
			desc:        "System name matches initiator",
			preseed:     Preseed{Initiator: "A"},
			name:        "A",
			isInitiator: true,
		},
		{
			desc:        "System name doesn't match initiator",
			preseed:     Preseed{Initiator: "A"},
			name:        "B",
			isInitiator: false,
		},
		{
			desc:        "System address does match initiator address",
			preseed:     Preseed{InitiatorAddress: "1.0.0.1"},
			address:     "1.0.0.1",
			isInitiator: true,
		},
		{
			desc:        "System address doesn't match initiator address",
			preseed:     Preseed{InitiatorAddress: "1.0.0.1"},
			address:     "1.0.0.2",
			isInitiator: false,
		},
	}

	for _, c := range cases {
		s.T().Log(c.desc)

		s.Equal(c.isInitiator, c.preseed.isInitiator(c.name, c.address))
	}
}

func (s *preseedSuite) Test_isBootstrap() {
	cases := []struct {
		desc        string
		preseed     Preseed
		isBootstrap bool
	}{
		{
			desc:        "Initiator is in the list of systems",
			preseed:     Preseed{Initiator: "A", Systems: []System{{Name: "A"}}},
			isBootstrap: true,
		},
		{
			desc:        "Initiator is not in the list of systems",
			preseed:     Preseed{Initiator: "B", Systems: []System{{Name: "A"}}},
			isBootstrap: false,
		},
		{
			desc:        "Initiator address is in the list of systems",
			preseed:     Preseed{InitiatorAddress: "1.0.0.1", Systems: []System{{Name: "A", Address: "1.0.0.1"}}},
			isBootstrap: true,
		},
		{
			desc:        "Initiator address is not in the list of systems",
			preseed:     Preseed{InitiatorAddress: "1.0.0.2", Systems: []System{{Name: "A", Address: "1.0.0.1"}}},
			isBootstrap: false,
		},
	}

	for _, c := range cases {
		s.T().Log(c.desc)

		s.Equal(c.isBootstrap, c.preseed.isBootstrap())
	}
}

func (s *preseedSuite) Test_address() {
	cases := []struct {
		desc    string
		name    string
		preseed Preseed
		address string
		err     error
	}{
		{
			desc: "Local address is the one specified in preseed",
			name: "A",
			preseed: Preseed{
				Systems: []System{{Name: "A", Address: "1.0.0.1"}},
			},
			address: "1.0.0.1",
		},
		{
			// Assumption that the test system has a `lo` with 127.0.0.0/8.
			// This allows not specifying/creating a custom interface for testing.
			desc: "Local address is the first one from lookup subnet",
			preseed: Preseed{
				LookupSubnet: "127.0.0.0/8",
			},
			address: "127.0.0.1",
		},
		{
			desc: "Failed to parse lookup subnet",
			preseed: Preseed{
				LookupSubnet: "foo",
			},
			err: errors.New("invalid CIDR address: foo"),
		},
		{
			desc: "Failed to find address in non-existing subnet",
			preseed: Preseed{
				LookupSubnet: "1.2.3.0/24",
			},
			err: errors.New(`Failed to determine MicroCloud address within subnet "1.2.3.0/24"`),
		},
	}

	for _, c := range cases {
		s.T().Log(c.desc)

		addr, err := c.preseed.address(c.name)
		if c.err != nil {
			s.Equal(c.err.Error(), err.Error())
		} else {
			s.Equal(c.address, addr)
		}
	}
}
