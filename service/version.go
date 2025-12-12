package service

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/cmd/tui"
)

const (
	// lxdMinVersion is the minimum version of LXD that fully supports all MicroCloud features.
	lxdMinVersion = "5.21"

	// microCephMinVersion is the minimum version of MicroCeph that fully supports all MicroCloud features.
	microCephMinVersion = "19.2"

	// microOVNMinVersion is the minimum version of MicroOVN that fully supports all MicroCloud features.
	microOVNMinVersion = "24.03"
)

func cleanVersion(version string) string {
	// Account for semantic version with major, minor and patch number.
	versionCleaned := make([]string, 0, 3)

	// Remove leading zeros.
	// 24.03 will result in 24.3
	// 25.09 will result in 25.9
	// 25.0 will stay 25.0
	for part := range strings.SplitSeq(version, ".") {
		if part != "0" {
			part = strings.TrimLeft(part, "0")
		}

		versionCleaned = append(versionCleaned, part)
	}

	return strings.Join(versionCleaned, ".")
}

func compareVersion(presentVersion string, minVersion string, serviceType types.ServiceType) error {
	canonicalPresentVersion := semver.Canonical("v" + cleanVersion(presentVersion))
	canonicalMinVersion := semver.Canonical("v" + cleanVersion(minVersion))

	// semver.Compare returns
	// * 0 in case presentVersion == minVersion
	// * 1 in case presentVersion > minVersion
	// * -1 in case presentVersion < minVersion
	comparison := semver.Compare(semver.MajorMinor(canonicalPresentVersion), semver.MajorMinor(canonicalMinVersion))

	// Only if the present version is lower than the expected version MicroCloud should error out.
	if comparison == -1 {
		return fmt.Errorf("%s version %q is not supported", serviceType, presentVersion)
	}

	// Print a warning in case a non-LTS (higher than the min) version is used.
	if comparison == 1 {
		tui.PrintWarning(fmt.Sprintf("Discovered non-LTS version %q of %s", presentVersion, serviceType))
	}

	return nil
}

// validateVersion checks that the daemon version for the given service is at a supported version for this version of MicroCloud.
func validateVersion(serviceType types.ServiceType, daemonVersion string) error {
	switch serviceType {
	case types.LXD:
		err := compareVersion(daemonVersion, lxdMinVersion, serviceType)
		if err != nil {
			return err
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

		daemonVersion = semver.Canonical("v" + match)
		expectedVersion := semver.Canonical("v" + microCephMinVersion)
		if semver.Compare(semver.MajorMinor(daemonVersion), semver.MajorMinor(expectedVersion)) != 0 {
			return fmt.Errorf("%s version %q is not supported", serviceType, daemonVersion)
		}
	}

	return nil
}
