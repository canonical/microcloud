package component

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
		component types.ComponentType
		expectErr bool
	}{
		{
			desc:      "Valid MicroCeph",
			version:   fmt.Sprintf("ceph-version: %s.0~git", microCephMinVersion),
			component: types.MicroCeph,
			expectErr: false,
		},
		{
			desc:      "Valid MicroOVN",
			version:   microOVNMinVersion,
			component: types.MicroOVN,
			expectErr: false,
		},
		{
			desc:      "Valid MicroCloud",
			version:   version.RawVersion,
			component: types.MicroCloud,
			expectErr: false,
		},
		{
			desc:      "Valid LXD",
			version:   lxdMinVersion,
			component: types.LXD,
			expectErr: false,
		},
		{
			desc:      "Invalid MicroCeph",
			version:   microCephMinVersion,
			component: types.MicroCeph,
			expectErr: true,
		},
		{
			desc:      "Valid LXD with different patch version",
			version:   fmt.Sprintf("%s.999", lxdMinVersion),
			component: types.LXD,
			expectErr: false,
		},
		{
			desc:      "Valid MicroCeph with different patch version",
			version:   fmt.Sprintf("ceph-version: %s.999~git", microCephMinVersion),
			component: types.MicroCeph,
			expectErr: false,
		},
		{
			desc:      "MicroCloud is always valid because it's local",
			version:   "",
			component: types.MicroCloud,
			expectErr: false,
		},
		{
			desc:      "Unsupported LXD with different minor version",
			version:   fmt.Sprintf("%s.999", strings.Split(lxdMinVersion, ".")[0]),
			component: types.LXD,
			expectErr: true,
		},
		{
			desc:      "Unsupported MicroCeph with larger minor version",
			version:   fmt.Sprintf("ceph-version: %s.999~git", strings.Split(microCephMinVersion, ".")[0]),
			component: types.MicroCeph,
			expectErr: true,
		},
		{
			desc:      "Unsupported MicroCeph with smaller minor version",
			version:   fmt.Sprintf("ceph-version: %s.0~git", strings.Split(microCephMinVersion, ".")[0]),
			component: types.MicroCeph,
			expectErr: true,
		},
		{
			desc:      "Unsupported LXD with larger major version",
			version:   "999.0",
			component: types.LXD,
			expectErr: true,
		},
		{
			desc:      "Unsupported LXD with smaller major version",
			version:   "1.0",
			component: types.LXD,
			expectErr: true,
		},
		{
			desc:      "Unsupported MicroCeph with larger major version",
			version:   "ceph-version: 999.0.0~git",
			component: types.MicroCeph,
			expectErr: true,
		},
		{
			desc:      "Unsupported MicroCeph with smaller major version",
			version:   "ceph-version: 1.0.0~git",
			component: types.MicroCeph,
			expectErr: true,
		},

		{
			desc:      "Unsupported MicroOVN (direct string comparison)",
			version:   microOVNMinVersion + ".0",
			component: types.MicroOVN,
			expectErr: true,
		},
	}

	for i, c := range cases {
		s.T().Logf("%d: %s", i, c.desc)

		err := validateVersion(c.component, c.version)
		if c.expectErr {
			s.Error(err)
		} else {
			s.NoError(err)
		}
	}
}
