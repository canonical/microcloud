package service

import (
	"testing"

	"github.com/canonical/lxd/shared/api"
	"github.com/stretchr/testify/suite"
)

type diskSuite struct {
	suite.Suite
}

func TestDiskSuite(t *testing.T) {
	suite.Run(t, new(diskSuite))
}

func (s *diskSuite) Test_IsPristineDisk() {
	cases := []struct {
		desc     string
		disk     api.ResourcesStorageDisk
		pristine bool
	}{
		{
			desc:     "Empty disk",
			disk:     api.ResourcesStorageDisk{ID: "sdc"},
			pristine: true,
		},
		{
			desc: "Disk with a filesystem (btrfs, lvm)",
			disk: api.ResourcesStorageDisk{
				ID:           "sdd",
				DeviceFSUUID: "f391ed2f-b18f-4cae-ae4a-a45059cc42e9",
			},
			pristine: false,
		},
		{
			desc: "Disk with partitions but no filesystem (zfs)",
			disk: api.ResourcesStorageDisk{
				ID: "sdb",
				Partitions: []api.ResourcesStorageDiskPartition{
					{ID: "sdb1", DeviceFSUUID: "16839297897617386792"},
					{ID: "sdb9"},
				},
			},
			pristine: false,
		},
	}

	for i, c := range cases {
		s.T().Logf("%d: %s", i, c.desc)

		s.Equal(c.pristine, IsPristineDisk(c.disk))
	}
}
