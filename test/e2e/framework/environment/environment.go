// Package environment bootstraps and tears down the "e2e-testing" LXD
// project, profile, and base images shared by every scenario in this suite.
package environment

import (
	"fmt"
	"strings"

	lxdclient "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
)

// registryCandidates is the ordered list of public image remotes tried by
// EnsureBaseImages, mirroring the remote the Terraform fixture used to pull
// from (ubuntu-minimal-daily). Unlike the best-effort registry-import check
// in the image-lifecycle scenario, this is not optional: the whole suite
// needs at least one base image per type to exist, so failure to reach any
// candidate is a fatal setup error.
var registryCandidates = []string{"ubuntu-minimal-daily", "ubuntu-minimal", "images"}

// baseImageAlias is the alias looked up/pulled on the chosen registry remote.
const baseImageAlias = "24.04"

// Bootstrapper creates and tears down the "e2e-testing" project, profile,
// and base images on a Connection.
//
// A Bootstrapper is constructed once by the test harness
// (e2e_suite_test.go) and exposed to scenario packages (e.g. to look up
// already-cached base image fingerprints via EnsureBaseImages, which is
// idempotent) via DefaultBootstrapper.
type Bootstrapper struct {
	conn *lxd.Connection
}

// DefaultBootstrapper is the Bootstrapper constructed by the test harness,
// for scenario packages to use. It is nil until the harness's
// SynchronizedBeforeSuite assigns it.
var DefaultBootstrapper *Bootstrapper

// New returns a Bootstrapper operating on conn.
func New(conn *lxd.Connection) *Bootstrapper {
	return &Bootstrapper{conn: conn}
}

// EnsureProject creates the "e2e-testing" project if it doesn't already
// exist. It is idempotent: an "already exists" error from the server is not
// treated as a failure.
func (b *Bootstrapper) EnsureProject(projectName string) error {
	err := b.conn.Server().CreateProject(api.ProjectsPost{
		Name: projectName,
		ProjectPut: api.ProjectPut{
			Config: map[string]string{
				"features.images":          "true",
				"features.profiles":        "true",
				"features.storage.volumes": "true",
				"features.storage.buckets": "true",
			},
		},
	})
	if err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("Cannot create project %q: %w", projectName, err)
	}

	return nil
}

// EnsureProfile creates the e2e-testing profile (a root disk on the "remote"
// pool and an eth0 NIC on the "default" network) in the "e2e-testing"
// project if it doesn't already exist. Mirrors the profile that
// test/e2e/terraform/main.tf used to create.
func (b *Bootstrapper) EnsureProfile(profileName string) error {
	err := b.conn.Server().CreateProfile(api.ProfilesPost{
		Name: profileName,
		ProfilePut: api.ProfilePut{
			Config: map[string]string{
				"limits.cpu":    "1",
				"limits.memory": "384MiB",
			},
			Devices: map[string]map[string]string{
				"root": {
					"type": "disk",
					"pool": "remote",
					"path": "/",
					"size": "4GiB",
				},
				"eth0": {
					"type":    "nic",
					"network": "default",
				},
			},
		},
	})
	if err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("Cannot create profile %q: %w", profileName, err)
	}

	return nil
}

// EnsureBaseImages ensures one container-type and one VM-type base image are
// cached in the "e2e-testing" project, pulling from the first reachable
// remote in registryCandidates. Returns their fingerprints.
func (b *Bootstrapper) EnsureBaseImages(projectName string) (containerFingerprint, vmFingerprint string, err error) {
	remote, err := b.pickReachableRegistry()
	if err != nil {
		return "", "", err
	}

	containerFingerprint, err = b.ensureBaseImage(remote, api.InstanceTypeContainer, projectName)
	if err != nil {
		return "", "", err
	}

	vmFingerprint, err = b.ensureBaseImage(remote, api.InstanceTypeVM, projectName)
	if err != nil {
		return "", "", err
	}

	return containerFingerprint, vmFingerprint, nil
}

// pickReachableRegistry returns the first remote in registryCandidates that
// responds to a GetImageAliases() call.
func (b *Bootstrapper) pickReachableRegistry() (lxdclient.ImageServer, error) {
	var lastErr error

	for _, name := range registryCandidates {
		s, err := lxd.ConnectImageServer(name)
		if err != nil {
			lastErr = err
			continue
		}

		_, err = s.GetImageAliases()
		if err != nil {
			lastErr = err
			continue
		}

		return s, nil
	}

	return nil, fmt.Errorf("Cannot reach any candidate public image registry %v: %w", registryCandidates, lastErr)
}

// ensureBaseImage copies baseImageAlias from remote into the "e2e-testing"
// project for the given instance type, if an image of that type isn't
// already cached there, and returns its fingerprint.
func (b *Bootstrapper) ensureBaseImage(remote lxdclient.ImageServer, instanceType api.InstanceType, projectName string) (string, error) {
	images, err := b.conn.Server().GetImages()
	if err != nil {
		return "", fmt.Errorf("Cannot list images in project %q: %w", projectName, err)
	}

	for _, img := range images {
		if img.Type == string(instanceType) {
			return img.Fingerprint, nil
		}
	}

	alias, _, err := remote.GetImageAliasType(string(instanceType), baseImageAlias)
	if err != nil {
		return "", fmt.Errorf("Cannot resolve %s alias %q on the registry: %w", instanceType, baseImageAlias, err)
	}

	image, _, err := remote.GetImage(alias.Target)
	if err != nil {
		return "", fmt.Errorf("Cannot get image %q from the registry: %w", alias.Target, err)
	}

	op, err := b.conn.Server().CopyImage(remote, *image, &lxdclient.ImageCopyArgs{})
	if err != nil {
		return "", fmt.Errorf("Cannot copy %s base image into project %q: %w", instanceType, projectName, err)
	}

	err = op.Wait()
	if err != nil {
		return "", fmt.Errorf("Cannot copy %s base image into project %q: %w", instanceType, projectName, err)
	}

	return image.Fingerprint, nil
}

// TeardownEnvironment deletes the "e2e-testing" project and everything in
// it, unconditionally.
//
// It deletes the profile and base images explicitly and then issues a plain
// (non-force) DeleteProject, rather than a force-delete: a non-force project
// delete only tolerates the implicit "default" profile and images that LXD
// considers genuinely "cached" (auto-pulled-and-cleaned-up, an internal
// distinction unrelated to whether the image happens to be used by any
// instance) — our own non-default profile and the base images
// EnsureBaseImages copies via CopyImage (which are not marked "cached" in
// LXD's sense) both count as blocking entities otherwise. By deleting both
// explicitly first, a plain delete succeeds without ever invoking the
// force-delete code path: deleting entities one at a time here is a
// narrower, sequential set of operations, as opposed to force-delete's bulk,
// concurrent deletion of every remaining volume, which is specifically where
// known-transient Ceph/LXD races (cluster members racing to clean up the
// same storage volume) were previously observed.
//
// Any future scenario that creates custom storage volumes or networks at
// the project level (rather than scenario-scoped resources that clean
// themselves up) must fully delete them itself, or this plain delete will
// fail outright rather than sweeping them away.
func (b *Bootstrapper) TeardownEnvironment(projectName, profileName string) error {
	err := b.conn.Server().DeleteProfile(profileName)
	if err != nil && !api.StatusErrorCheck(err, 404) {
		return fmt.Errorf("Cannot delete profile %q: %w", profileName, err)
	}

	images, err := b.conn.Server().GetImages()
	if err != nil {
		return fmt.Errorf("Cannot list images in project %q: %w", projectName, err)
	}

	for _, image := range images {
		op, err := b.conn.Server().DeleteImage(image.Fingerprint)
		if err == nil {
			err = op.Wait()
		}

		if err != nil && !api.StatusErrorCheck(err, 404) {
			return fmt.Errorf("Cannot delete image %q: %w", image.Fingerprint, err)
		}
	}

	op, err := b.conn.Server().DeleteProject(projectName, false)
	if err == nil {
		err = op.Wait()
	}

	if err != nil {
		return fmt.Errorf("Cannot delete project %q: %w", projectName, err)
	}

	return nil
}

// isAlreadyExists reports whether err looks like an LXD "already exists"
// error, so setup helpers can be idempotent.
func isAlreadyExists(err error) bool {
	if err == nil {
		return false
	}

	return api.StatusErrorCheck(err, 409) || strings.Contains(strings.ToLower(err.Error()), "already exists")
}
