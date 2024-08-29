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
		subnet  string
		iface   string
		systems []System
		ovn     InitNetwork
		storage StorageFilter

		addErr bool
		err    error
	}{
		{
			desc:    "No systems",
			subnet:  "10.0.0.1/24",
			iface:   "enp5s0",
			systems: nil,
			ovn:     InitNetwork{IPv4Gateway: "10.0.0.1/24", IPv4Range: "10.0.0.100-10.0.0.254", IPv6Gateway: "cafe::1/64"},
			storage: StorageFilter{
				Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
				Ceph:  []DiskFilter{{Find: "def", FindMin: 0, FindMax: 3, Wipe: false}},
			},

			addErr: true,
			err:    errors.New("No systems given"),
		},
		{
			desc:    "Single node preseed",
			subnet:  "10.0.0.1/24",
			iface:   "enp5s0",
			systems: []System{{Name: "n1", UplinkInterface: "eth0", Storage: InitStorage{}}},
			ovn:     InitNetwork{IPv4Gateway: "10.0.0.1/24", IPv4Range: "10.0.0.100-10.0.0.254", IPv6Gateway: "cafe::1/64"},
			storage: StorageFilter{
				Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
				Ceph:  []DiskFilter{{Find: "def", FindMin: 1, FindMax: 3, Wipe: false}},
			},

			addErr: false,
			err:    nil,
		},
		{
			desc:    "Missing lookup subnet",
			systems: []System{{Name: "n1"}, {Name: "n2"}},
			storage: StorageFilter{
				Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
			},

			addErr: true,
			err:    errors.New("invalid CIDR address: "),
		},
		{
			desc:    "Missing lookup interface",
			subnet:  "10.0.0.1/24",
			systems: []System{{Name: "n1"}, {Name: "n2"}},
			storage: StorageFilter{
				Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
			},

			addErr: true,
			err:    errors.New("Missing interface name for machine lookup"),
		},
		{
			desc:    "Systems missing name",
			subnet:  "10.0.0.1/24",
			iface:   "enp5s0",
			systems: []System{{Name: "", UplinkInterface: "eth0"}, {Name: "n2", UplinkInterface: "eth0"}},
			ovn:     InitNetwork{IPv4Gateway: "10.0.0.1/24", IPv4Range: "10.0.0.100-10.0.0.254", IPv6Gateway: "cafe::1/64"},
			storage: StorageFilter{
				Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
				Ceph:  []DiskFilter{{Find: "def", FindMin: 0, FindMax: 3, Wipe: false}},
			},

			addErr: true,
			err:    errors.New("Missing system name"),
		},
		{
			desc:    "FindMin too low for ceph filter",
			subnet:  "10.0.0.1/24",
			iface:   "enp5s0",
			systems: []System{{Name: "n1"}, {Name: "n2"}},
			storage: StorageFilter{
				Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
				Ceph:  []DiskFilter{{Find: "def", FindMin: 0, FindMax: 3, Wipe: false}},
			},

			addErr: true,
			err:    errors.New("Remote storage filter cannot be defined with find_min less than 1"),
		},
		{
			desc:   "Ceph direct selection (3) with more systems (4)",
			subnet: "10.0.0.1/24",
			iface:  "enp5s0",
			systems: []System{
				{Name: "n1", Storage: InitStorage{Ceph: []DirectStorage{{Path: "def"}}}},
				{Name: "n2", Storage: InitStorage{Ceph: []DirectStorage{{Path: "def"}}}},
				{Name: "n3", Storage: InitStorage{Ceph: []DirectStorage{{Path: "def"}}}},
				{Name: "n4"}},
			addErr: false,
			err:    nil,
		},
		{
			desc:   "Minimum ceph direct selection (1) with more systems (4)",
			subnet: "10.0.0.1/24",
			iface:  "enp5s0",
			systems: []System{
				{Name: "n1", Storage: InitStorage{Ceph: []DirectStorage{{Path: "def"}}}},
				{Name: "n2"},
				{Name: "n3"},
				{Name: "n4"}},
			addErr: false,
			err:    nil,
		},
		{
			desc:    "Incomplete zfs direct selection",
			subnet:  "10.0.0.1/24",
			iface:   "enp5s0",
			systems: []System{{Name: "n1", Storage: InitStorage{Local: DirectStorage{Path: "def"}}}, {Name: "n2", Storage: InitStorage{Local: DirectStorage{Path: "def"}}}, {Name: "n3"}},
			addErr:  true,
			err:     errors.New("Some systems are missing local storage disks"),
		},
		{
			desc:    "Invalid zfs filter constraint",
			subnet:  "10.0.0.1/24",
			iface:   "enp5s0",
			systems: []System{{Name: "n1"}, {Name: "n2"}},
			storage: StorageFilter{
				Local: []DiskFilter{{Find: "abc", FindMin: 3, FindMax: 2, Wipe: false}},
			},
			addErr: true,
			err:    errors.New("Invalid local storage filter constraints find_max (2) larger than find_min (3)"),
		},
		{
			desc:    "Invalid zfs filter value",
			subnet:  "10.0.0.1/24",
			iface:   "enp5s0",
			systems: []System{{Name: "n1"}, {Name: "n2"}},
			storage: StorageFilter{
				Local: []DiskFilter{{Find: "", FindMin: 3, FindMax: 2, Wipe: false}},
			},
			addErr: true,
			err:    errors.New("Received empty local disk filter"),
		},
		{
			desc:    "Invalid ceph filter min > max",
			subnet:  "10.0.0.1/24",
			iface:   "enp5s0",
			systems: []System{{Name: "n1"}, {Name: "n2"}, {Name: "n3"}},
			storage: StorageFilter{
				Ceph: []DiskFilter{{Find: "def", FindMin: 4, FindMax: 3, Wipe: false}},
			},
			addErr: true,
			err:    errors.New("Invalid remote storage filter constraints find_max (3) must be larger than find_min (4)"),
		},
		{
			desc:    "Invalid ceph filter constraints",
			subnet:  "10.0.0.1/24",
			iface:   "enp5s0",
			systems: []System{{Name: "n1"}, {Name: "n2"}, {Name: "n3"}},
			storage: StorageFilter{
				Ceph: []DiskFilter{{Find: "", FindMin: 4, FindMax: 3, Wipe: false}},
			},
			addErr: true,
			err:    errors.New("Received empty remote disk filter"),
		},
		{
			desc:    "Systems missing interface",
			subnet:  "10.0.0.1/24",
			iface:   "enp5s0",
			systems: []System{{Name: "n1", UplinkInterface: ""}, {Name: "n2", UplinkInterface: "eth0"}, {Name: "n3", UplinkInterface: "eth0"}},
			ovn:     InitNetwork{IPv4Gateway: "10.0.0.1/24", IPv4Range: "10.0.0.100-10.0.0.254", IPv6Gateway: "cafe::1/64"},
			storage: StorageFilter{
				Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
				Ceph:  []DiskFilter{{Find: "def", FindMin: 3, FindMax: 3, Wipe: false}},
			},

			addErr: true,
			err:    errors.New("Some systems are missing an uplink interface"),
		},
		{
			desc:    "OVN IPv4 Ranges with no gateway",
			subnet:  "10.0.0.1/24",
			iface:   "enp5s0",
			systems: []System{{Name: "n1", UplinkInterface: "eth0"}, {Name: "n2", UplinkInterface: "eth0"}, {Name: "n3", UplinkInterface: "eth0"}},
			ovn:     InitNetwork{IPv4Range: "10.0.0.100-10.0.0.254", IPv6Gateway: "cafe::1/64"},
			storage: StorageFilter{
				Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
				Ceph:  []DiskFilter{{Find: "def", FindMin: 3, FindMax: 3, Wipe: false}},
			},

			addErr: true,
			err:    errors.New("Cannot specify IPv4 range without IPv4 gateway"),
		},
		{
			desc:    "Invalid OVN IPv4 Ranges",
			subnet:  "10.0.0.1/24",
			iface:   "enp5s0",
			systems: []System{{Name: "n1", UplinkInterface: "eth0"}, {Name: "n2", UplinkInterface: "eth0"}, {Name: "n3", UplinkInterface: "eth0"}},
			ovn:     InitNetwork{IPv4Gateway: "10.0.0.1/24", IPv4Range: "10.0.0.100,10.0.0.254", IPv6Gateway: "cafe::1/64"},
			storage: StorageFilter{
				Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
				Ceph:  []DiskFilter{{Find: "def", FindMin: 3, FindMax: 3, Wipe: false}},
			},

			addErr: true,
			err:    errors.New("Invalid IPv4 range (must be of the form <ip>-<ip>)"),
		},
	}

	s.T().Log("Preseed init missing local system")
	p := Preseed{LookupSubnet: "10.0.0.1/24", Systems: []System{{Name: "B"}, {Name: "C"}}}
	err := p.validate("A", true)
	s.EqualError(err, "Local MicroCloud must be included in the list of systems when initializing")
	s.T().Log("Preseed add includes local system")
	p = Preseed{LookupSubnet: "10.0.0.1/24", Systems: []System{{Name: "A"}, {Name: "B"}}}
	err = p.validate("A", false)
	s.EqualError(err, "Local MicroCloud must not be included in the list of systems when adding new members")

	for _, c := range cases {
		s.T().Log(c.desc)
		p := Preseed{
			LookupSubnet:    c.subnet,
			LookupInterface: c.iface,
			Systems:         c.systems,
			OVN:             c.ovn,
			Storage:         c.storage,
		}

		err := p.validate("n1", true)
		if c.err == nil {
			s.NoError(err)
		} else {
			s.EqualError(err, c.err.Error())
		}

		s.T().Logf("%s in add mode", c.desc)
		err = p.validate("n0", false)
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
	p := Preseed{ReuseExistingClusters: true, LookupSubnet: "10.0.0.1/24", LookupInterface: "enp5s0", Systems: []System{{Name: "B"}, {Name: "C"}}}

	s.NoError(p.validate("B", true))

	s.Error(p.validate("A", false))

	p.ReuseExistingClusters = false
	s.NoError(p.validate("A", false))
}
