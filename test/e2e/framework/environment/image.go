package environment

import (
	"fmt"

	lxdclient "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
)

// baseImageAlias is the alias looked up/pulled on the chosen registry remote.
const baseImageAlias = "24.04"

// registryCandidates returns the ordered list of public image remotes tried by EnsureBaseImages
// (mirroring the registry the suite's former Terraform-based setup pulled images from,
// ubuntu-minimal-daily). Unlike the best-effort registry-import check in the image-lifecycle
// scenario, this is not optional: the whole suite needs at least one base image per type to exist,
// so failure to reach any candidate is a fatal setup error. Returns a fresh slice on every call so
// callers can't accidentally mutate shared state.
func registryCandidates() []string {
	return []string{"ubuntu-minimal-daily", "ubuntu-minimal", "images"}
}

// pickReachableRegistry returns the first remote in registryCandidates() that responds to a
// GetImageAliases() call.
func (p *Provisioner) pickReachableRegistry() (lxdclient.ImageServer, error) {
	var lastErr error

	candidates := registryCandidates()

	for _, name := range candidates {
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

	return nil, fmt.Errorf("Cannot reach any candidate public image registry %v: %w", candidates, lastErr)
}

// ensureBaseImage copies baseImageAlias from remote into the "e2e-testing" project for the given
// instance type, if an image of that type isn't already cached there, and returns its fingerprint.
func (p *Provisioner) ensureBaseImage(remote lxdclient.ImageServer, instanceType api.InstanceType, projectName string) (string, error) {
	images, err := lxd.ListImages(p.conn)
	if err != nil {
		return "", err
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

	return lxd.CopyImageFromRemote(p.conn, remote, *image)
}

// EnsureBaseImages ensures one container-type and one VM-type base image are cached in the
// "e2e-testing" project, pulling from the first reachable remote in registryCandidates. Returns
// their fingerprints.
func (p *Provisioner) EnsureBaseImages(projectName string) (containerFingerprint, vmFingerprint string, err error) {
	remote, err := p.pickReachableRegistry()
	if err != nil {
		return "", "", err
	}

	containerFingerprint, err = p.ensureBaseImage(remote, api.InstanceTypeContainer, projectName)
	if err != nil {
		return "", "", err
	}

	vmFingerprint, err = p.ensureBaseImage(remote, api.InstanceTypeVM, projectName)
	if err != nil {
		return "", "", err
	}

	return containerFingerprint, vmFingerprint, nil
}

// DeleteAllImages deletes every image in the "e2e-testing" project, unconditionally. The
// teardown-side counterpart to EnsureBaseImages.
func (p *Provisioner) DeleteAllImages(projectName string) error {
	images, err := lxd.ListImages(p.conn)
	if err != nil {
		return err
	}

	for _, image := range images {
		err := lxd.DeleteImage(p.conn, image.Fingerprint)
		if err != nil {
			return err
		}
	}

	return nil
}
