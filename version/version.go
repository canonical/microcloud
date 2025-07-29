// Package version provides shared version information.
package version

// RawVersion is the current daemon version of MicroCloud.
// LTS versions also include the patch number.
const RawVersion = "2.1.1"

// LTS should be set if the current version is an LTS (long-term support) version.
const LTS = true

// Version appends "LTS" to the raw version string if MicroCloud is an LTS version.
func Version() string {
	if LTS {
		return RawVersion + " LTS"
	}

	return RawVersion
}
