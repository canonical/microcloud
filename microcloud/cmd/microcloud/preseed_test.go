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
			desc:    "Not enough systems",
			subnet:  "10.0.0.1/24",
			iface:   "enp5s0",
			systems: []System{{Name: "n1", UplinkInterface: "eth0", Storage: InitStorage{}}},
			ovn:     InitNetwork{IPv4Gateway: "10.0.0.1/24", IPv4Range: "10.0.0.100-10.0.0.254", IPv6Gateway: "cafe::1/64"},
			storage: StorageFilter{
				Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
				Ceph:  []DiskFilter{{Find: "def", FindMin: 0, FindMax: 3, Wipe: false}},
			},

			addErr: false,
			err:    errors.New("At least 2 systems are required to set up MicroCloud"),
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
			desc:    "Too few systems for ceph filter",
			subnet:  "10.0.0.1/24",
			iface:   "enp5s0",
			systems: []System{{Name: "n1"}, {Name: "n2"}},
			storage: StorageFilter{
				Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
				Ceph:  []DiskFilter{{Find: "def", FindMin: 0, FindMax: 3, Wipe: false}},
			},

			addErr: false,
			err:    errors.New("At least 3 systems are required to configure distributed storage"),
		},
		{
			desc:    "Too few systems for ceph direct",
			subnet:  "10.0.0.1/24",
			iface:   "enp5s0",
			systems: []System{{Name: "n1", Storage: InitStorage{Ceph: []DirectStorage{{Path: "def"}}}}, {Name: "n2", Storage: InitStorage{Ceph: []DirectStorage{{Path: "def"}}}}},
			addErr:  false,
			err:     errors.New("At least 3 systems must specify ceph storage disks"),
		},
		{
			desc:    "Incomplete ceph direct selection",
			subnet:  "10.0.0.1/24",
			iface:   "enp5s0",
			systems: []System{{Name: "n1", Storage: InitStorage{Ceph: []DirectStorage{{Path: "def"}}}}, {Name: "n2", Storage: InitStorage{Ceph: []DirectStorage{{Path: "def"}}}}, {Name: "n3"}},
			addErr:  false,
			err:     errors.New("At least 3 systems must specify ceph storage disks"),
		},
		{
			desc:   "Minimum ceph direct selection (3) with more systems (4)",
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
			desc:   "Multiple disks on the same system don't count towards the minimum quota:",
			subnet: "10.0.0.1/24",
			iface:  "enp5s0",
			systems: []System{
				{Name: "n1", Storage: InitStorage{Ceph: []DirectStorage{{Path: "def"}}}},
				{Name: "n2", Storage: InitStorage{Ceph: []DirectStorage{{Path: "def"}, {Path: "def2"}}}},
				{Name: "n3"}},
			addErr: false,
			err:    errors.New("At least 3 systems must specify ceph storage disks"),
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
			desc:    "Invalid ceph filter min count",
			subnet:  "10.0.0.1/24",
			iface:   "enp5s0",
			systems: []System{{Name: "n1"}, {Name: "n2"}, {Name: "n3"}},
			storage: StorageFilter{
				Ceph: []DiskFilter{{Find: "def", FindMin: 0, FindMax: 2, Wipe: false}},
			},
			addErr: false,
			err:    errors.New("Invalid remote storage filter constraints find_max (2) must be at least 3 and larger than find_min (0)"),
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
			err:    errors.New("Invalid remote storage filter constraints find_max (3) must be at least 3 and larger than find_min (4)"),
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
				Ceph:  []DiskFilter{{Find: "def", FindMin: 0, FindMax: 3, Wipe: false}},
			},

			addErr: true,
			err:    errors.New("Some systems are missing an uplink interface"),
		},
		{
			desc:    "Too few systems for ovn",
			subnet:  "10.0.0.1/24",
			iface:   "enp5s0",
			systems: []System{{Name: "n1", UplinkInterface: "eth0"}, {Name: "n2", UplinkInterface: "eth0"}},
			ovn:     InitNetwork{IPv4Gateway: "10.0.0.1/24", IPv4Range: "10.0.0.100-10.0.0.254", IPv6Gateway: "cafe::1/64"},
			storage: StorageFilter{
				Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
			},

			addErr: false,
			err:    errors.New("At least 3 systems are required to configure distributed networking"),
		},
		{
			desc:    "OVN IPv4 Ranges with no gateway",
			subnet:  "10.0.0.1/24",
			iface:   "enp5s0",
			systems: []System{{Name: "n1", UplinkInterface: "eth0"}, {Name: "n2", UplinkInterface: "eth0"}, {Name: "n3", UplinkInterface: "eth0"}},
			ovn:     InitNetwork{IPv4Range: "10.0.0.100-10.0.0.254", IPv6Gateway: "cafe::1/64"},
			storage: StorageFilter{
				Local: []DiskFilter{{Find: "abc", FindMin: 0, FindMax: 3, Wipe: false}},
				Ceph:  []DiskFilter{{Find: "def", FindMin: 0, FindMax: 3, Wipe: false}},
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
				Ceph:  []DiskFilter{{Find: "def", FindMin: 0, FindMax: 3, Wipe: false}},
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
