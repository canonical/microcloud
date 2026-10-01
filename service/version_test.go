package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/version"
)

type versionSuite struct {
	suite.Suite
}

func TestVersionSuite(t *testing.T) {
	suite.Run(t, new(versionSuite))
}

func (s *versionSuite) Test_validateVersions() {
	cases := []struct {
		desc      string
		version   string
		service   types.ServiceType
		expectErr bool
	}{
		{
			desc:      "Valid MicroCeph",
			version:   fmt.Sprintf("ceph-version: %s.0~git", microCephMinVersion),
			service:   types.MicroCeph,
			expectErr: false,
		},
		{
			desc:      "Valid MicroOVN",
			version:   microOVNMinVersion,
			service:   types.MicroOVN,
			expectErr: false,
		},
		{
			desc:      "Valid MicroCloud",
			version:   version.RawVersion,
			service:   types.MicroCloud,
			expectErr: false,
		},
		{
			desc:      "Valid LXD",
			version:   lxdMinVersion,
			service:   types.LXD,
			expectErr: false,
		},
		{
			desc:      "Invalid MicroCeph",
			version:   microCephMinVersion,
			service:   types.MicroCeph,
			expectErr: true,
		},
		{
			desc:      "Valid LXD with different patch version",
			version:   lxdMinVersion + ".999",
			service:   types.LXD,
			expectErr: false,
		},
		{
			desc:      "Valid MicroCeph with different patch version",
			version:   fmt.Sprintf("ceph-version: %s.999~git", microCephMinVersion),
			service:   types.MicroCeph,
			expectErr: false,
		},
		{
			desc:      "MicroCloud is always valid because it's local",
			version:   "",
			service:   types.MicroCloud,
			expectErr: false,
		},
		{
			desc:      "Supported LXD with higher minor version",
			version:   strings.Split(lxdMinVersion, ".")[0] + ".999",
			service:   types.LXD,
			expectErr: false,
		},
		{
			desc:      "Unsupported MicroCeph with larger minor version",
			version:   fmt.Sprintf("ceph-version: %s.999~git", strings.Split(microCephMinVersion, ".")[0]),
			service:   types.MicroCeph,
			expectErr: true,
		},
		{
			desc:      "Unsupported MicroCeph with smaller minor version",
			version:   fmt.Sprintf("ceph-version: %s.0~git", strings.Split(microCephMinVersion, ".")[0]),
			service:   types.MicroCeph,
			expectErr: true,
		},
		{
			desc:      "Supported LXD with larger major version",
			version:   "999.0",
			service:   types.LXD,
			expectErr: false,
		},
		{
			desc:      "Unsupported LXD with smaller major version",
			version:   "1.0",
			service:   types.LXD,
			expectErr: true,
		},
		{
			desc:      "Unsupported LXD with smaller minor version",
			version:   "5.20",
			service:   types.LXD,
			expectErr: true,
		},
		{
			desc:      "Unsupported MicroCeph with smaller major version",
			version:   "ceph-version: 1.0.0~git",
			service:   types.MicroCeph,
			expectErr: true,
		},

		{
			desc:      "Unsupported MicroOVN with lower minor",
			version:   "24.02",
			service:   types.MicroOVN,
			expectErr: true,
		},
		{
			desc:      "Supported MicroOVN with larger major version",
			version:   "25.09",
			service:   types.MicroOVN,
			expectErr: false,
		},
		{
			desc:      "Supported MicroCeph with larger major version",
			version:   "ceph-version: 20.2.0-0ubuntu0.24.04.1~bpo24.04.1~ppa202602041400; microceph-git: e6eb74a064",
			service:   types.MicroCeph,
			expectErr: false,
		},
	}

	for i, c := range cases {
		s.T().Logf("%d: %s", i, c.desc)

		err := validateVersion(c.service, c.version)
		if c.expectErr {
			s.Error(err)
		} else {
			s.NoError(err)
		}
	}
}

func (s *versionSuite) Test_VersionsCompatible() {
	cases := []struct {
		desc         string
		service      types.ServiceType
		version      string
		otherVersion string
		compatible   bool
	}{
		{
			desc:         "Equal LXD versions are compatible",
			service:      types.LXD,
			version:      "6.5",
			otherVersion: "6.5",
			compatible:   true,
		},
		{
			desc:         "Different LXD versions are incompatible",
			service:      types.LXD,
			version:      "6.5",
			otherVersion: "6.6",
			compatible:   false,
		},
		{
			desc:         "Different MicroOVN versions are incompatible",
			service:      types.MicroOVN,
			version:      "24.03.2-a2c59c105b",
			otherVersion: "24.03.2-0123456789",
			compatible:   false,
		},
		{
			desc:         "Equal MicroCeph versions are compatible",
			service:      types.MicroCeph,
			version:      "ceph-version: 19.2.3-0ubuntu0.24.04.3; microceph-git: 897bcdd902",
			otherVersion: "ceph-version: 19.2.3-0ubuntu0.24.04.3; microceph-git: 897bcdd902",
			compatible:   true,
		},
		{
			desc:         "MicroCeph versions with equal Ceph version but different git version are compatible",
			service:      types.MicroCeph,
			version:      "ceph-version: 19.2.3-0ubuntu0.24.04.3; microceph-git: 897bcdd902",
			otherVersion: "ceph-version: 19.2.3-0ubuntu0.24.04.3; microceph-git: d95f07173f",
			compatible:   true,
		},
		{
			desc:         "MicroCeph versions with different Ceph version are incompatible",
			service:      types.MicroCeph,
			version:      "ceph-version: 19.2.3-0ubuntu0.24.04.3; microceph-git: 897bcdd902",
			otherVersion: "ceph-version: 19.2.3-0ubuntu0.24.04.4; microceph-git: 897bcdd902",
			compatible:   false,
		},
		{
			desc:         "MicroCeph versions without Ceph version are incompatible if not equal",
			service:      types.MicroCeph,
			version:      "897bcdd902",
			otherVersion: "d95f07173f",
			compatible:   false,
		},
		{
			desc:         "MicroCeph version without Ceph version is incompatible with one that has it",
			service:      types.MicroCeph,
			version:      "ceph-version: 19.2.3-0ubuntu0.24.04.3; microceph-git: 897bcdd902",
			otherVersion: "d95f07173f",
			compatible:   false,
		},
	}

	for i, c := range cases {
		s.T().Logf("%d: %s", i, c.desc)

		s.Equal(c.compatible, VersionsCompatible(c.service, c.version, c.otherVersion))
	}
}
