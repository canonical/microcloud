package service

import (
	"fmt"
	"regexp"

	"golang.org/x/mod/semver"

	"github.com/canonical/microcloud/microcloud/api/types"
)

const (
	// lxdMinVersion is the minimum version of LXD that fully supports all MicroCloud features.
	lxdMinVersion = "5.21"

	// microCephMinVersion is the minimum version of MicroCeph that fully supports all MicroCloud features.
	microCephMinVersion = "19.2"

	// microOVNMinVersion is the minimum version of MicroOVN that fully supports all MicroCloud features.
	microOVNMinVersion = "24.03"
)

// validateVersion checks that the daemon version for the given service is at a supported version for this version of MicroCloud.
func validateVersion(serviceType types.ServiceType, daemonVersion string) error {
	switch serviceType {
	case types.LXD:
		lxdVersion := semver.Canonical(fmt.Sprintf("v%s", daemonVersion))
		expectedVersion := semver.Canonical(fmt.Sprintf("v%s", lxdMinVersion))
		if semver.Compare(semver.MajorMinor(lxdVersion), semver.MajorMinor(expectedVersion)) != 0 {
			return fmt.Errorf("%s version %q is not supported", serviceType, daemonVersion)
		}

	case types.MicroOVN:
		if daemonVersion != microOVNMinVersion {
			return fmt.Errorf("%s version %q is not supported", serviceType, daemonVersion)
		}

	case types.MicroCeph:
		regex := regexp.MustCompile(`\d+\.\d+\.\d+`)
		match := regex.FindString(daemonVersion)
		if match == "" {
			return fmt.Errorf("%s version format not supported (%s)", serviceType, daemonVersion)
		}

		daemonVersion = semver.Canonical(fmt.Sprintf("v%s", match))
		expectedVersion := semver.Canonical(fmt.Sprintf("v%s", microCephMinVersion))
		if semver.Compare(semver.MajorMinor(daemonVersion), semver.MajorMinor(expectedVersion)) != 0 {
			return fmt.Errorf("%s version %q is not supported", serviceType, daemonVersion)
		}
	}

	return nil
}
