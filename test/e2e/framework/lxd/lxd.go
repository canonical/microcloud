package lxd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"

	lxdclient "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/lxc/config"
	"github.com/canonical/lxd/shared/api"
)

// DefaultRetryInterval is the wait time between retries of flaky operations
// (instance readiness polling, etc).
const DefaultRetryInterval = 5 * time.Second

// DefaultEventuallyTimeout bounds how long specs wait for eventually-
// consistent conditions (e.g. an instance acquiring an IPv4 address after
// being evacuated/restored) before failing.
const DefaultEventuallyTimeout = 30 * time.Second

// Connection is a connection to the remote MicroCloud deployment under test,
// scoped to ProjectName.
type Connection struct {
	server lxdclient.InstanceServer
}

// Connect connects to remoteName (as configured via `lxc remote add`),
// scoped to ProjectName.
func Connect(remoteName string, projectName string) (*Connection, error) {
	s, err := connectNamedRemote(remoteName, projectName)
	if err != nil {
		return nil, err
	}

	return &Connection{server: s}, nil
}

// Server returns the connection's underlying LXD server.
func (c *Connection) Server() lxdclient.InstanceServer {
	return c.server
}

// ConnectLocal connects to the special "local" remote (the LXD daemon
// running on this test host, reached over its unix socket, independent of
// REMOTE), scoped to project.
func ConnectLocal(project string) (lxdclient.InstanceServer, error) {
	return connectNamedRemote("local", project)
}

// ExecInInstanceOn runs argv inside the named instance on the given server
// and waits for it to complete. A non-zero exit status is not an error: it
// is returned as exitCode along with the captured output. err is only set
// when the command could not be started or its result could not be
// determined.
func ExecInInstanceOn(s lxdclient.InstanceServer, instance string, argv []string, env map[string]string) (stdout string, stderr string, exitCode int, err error) {
	var outBuf, errBuf bytes.Buffer

	op, err := s.ExecInstance(instance, api.InstanceExecPost{
		Command:     argv,
		WaitForWS:   true,
		Environment: env,
	}, &lxdclient.InstanceExecArgs{Stdout: &outBuf, Stderr: &errBuf})
	if err != nil {
		return "", "", -1, fmt.Errorf("Cannot exec into instance %q: %w", instance, err)
	}

	err = op.Wait()
	if err != nil {
		return outBuf.String(), errBuf.String(), -1, fmt.Errorf("Exec in instance %q failed: %w (output: %s)", instance, err, outBuf.String()+errBuf.String())
	}

	rc, ok := op.Get().Metadata["return"].(float64)
	if !ok {
		return outBuf.String(), errBuf.String(), -1, fmt.Errorf("Exec in instance %q returned no exit code", instance)
	}

	return outBuf.String(), errBuf.String(), int(rc), nil
}

// ConnectImageServer connects to a public, unauthenticated image remote
// (e.g. "ubuntu-minimal-daily") as configured via `lxc remote add --protocol
// simplestreams` (or LXD's own built-in defaults).
func ConnectImageServer(name string) (lxdclient.ImageServer, error) {
	configDir, err := lxcConfigDir()
	if err != nil {
		return nil, err
	}

	confPath := filepath.Join(configDir, "config.yml")

	conf, err := config.LoadConfig(confPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("Cannot load LXD client configuration from %q: %w", confPath, err)
		}

		conf = config.NewConfig(configDir, true)
	}

	s, err := conf.GetImageServer(name)
	if err != nil {
		return nil, fmt.Errorf("Cannot connect to remote %q: %w", name, err)
	}

	return s, nil
}

// connectNamedRemote connects to remoteName (as configured via `lxc remote
// add`, or the special "local" remote), scoped to project.
func connectNamedRemote(remoteName, project string) (lxdclient.InstanceServer, error) {
	configDir, err := lxcConfigDir()
	if err != nil {
		return nil, err
	}

	confPath := filepath.Join(configDir, "config.yml")

	conf, err := config.LoadConfig(confPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("Cannot load LXD client configuration from %q: %w", confPath, err)
		}

		conf = config.NewConfig(configDir, true)
	}

	s, err := conf.GetInstanceServer(remoteName)
	if err != nil {
		return nil, fmt.Errorf("Cannot connect to remote %q (using LXD client config from %q): %w", remoteName, confPath, err)
	}

	return s.UseProject(project), nil
}

// lxcConfigDir resolves the lxc config directory.
func lxcConfigDir() (string, error) {
	lxdConf := os.Getenv("LXD_CONF")
	if lxdConf != "" {
		return lxdConf, nil
	}

	home, err := homeDir()
	if err != nil {
		return "", err
	}

	snapConfigDir := filepath.Join(home, "snap", "lxd", "common", "config")

	_, err = os.Stat(filepath.Join(snapConfigDir, "config.yml"))
	if err == nil {
		return snapConfigDir, nil
	}

	return filepath.Join(home, ".config", "lxc"), nil
}

// homeDir returns the current user's home directory.
func homeDir() (string, error) {
	home := os.Getenv("HOME")
	if home != "" {
		return home, nil
	}

	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("Cannot determine the current user: %w", err)
	}

	return u.HomeDir, nil
}
