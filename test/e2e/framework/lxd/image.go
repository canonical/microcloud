package lxd

import (
	"fmt"

	lxdclient "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
)

// ConnectImageServer connects to a public, unauthenticated image remote (e.g.
// "ubuntu-minimal-daily") as configured via `lxc remote add --protocol simplestreams` (or LXD's
// own built-in defaults).
func ConnectImageServer(name string) (lxdclient.ImageServer, error) {
	configDir, err := lxcConfigDir()
	if err != nil {
		return nil, err
	}

	conf, err := loadLXCConfig(configDir)
	if err != nil {
		return nil, err
	}

	s, err := conf.GetImageServer(name)
	if err != nil {
		return nil, fmt.Errorf("Cannot connect to remote %q: %w", name, err)
	}

	return s, nil
}

// ListImages returns every image in conn's project.
func ListImages(conn *Connection) ([]api.Image, error) {
	images, err := conn.Server().GetImages()
	if err != nil {
		return nil, fmt.Errorf("Cannot list images in project %q: %w", conn.Project(), err)
	}

	return images, nil
}

// CopyImageFromRemote copies image from remote into conn's project, waiting for completion, and
// returns its fingerprint.
func CopyImageFromRemote(conn *Connection, remote lxdclient.ImageServer, image api.Image) (string, error) {
	op, err := conn.Server().CopyImage(remote, image, &lxdclient.ImageCopyArgs{})
	if err != nil {
		return "", fmt.Errorf("Cannot copy image %q into project %q: %w", image.Fingerprint, conn.Project(), err)
	}

	err = op.Wait()
	if err != nil {
		return "", fmt.Errorf("Cannot copy image %q into project %q: %w", image.Fingerprint, conn.Project(), err)
	}

	return image.Fingerprint, nil
}

// DeleteImage deletes the image with the given fingerprint on conn. It is safe to call
// unconditionally: a "not found" error is not treated as a failure.
func DeleteImage(conn *Connection, fingerprint string) error {
	op, err := conn.Server().DeleteImage(fingerprint)
	if err == nil {
		err = op.Wait()
	}

	if err != nil && !api.StatusErrorCheck(err, 404) {
		return fmt.Errorf("Cannot delete image %q: %w", fingerprint, err)
	}

	return nil
}
