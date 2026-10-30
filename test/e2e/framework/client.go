package framework

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/lxc/config"
)

// server is the shared LXD connection to REMOTE.	
var server lxd.InstanceServer

// Server returns the shared LXD connection.
func Server() lxd.InstanceServer {
	return server
}

// SetServer sets the shared LXD connection.
func SetServer(s lxd.InstanceServer) {
	server = s
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

// ConnectRemoteServer connects to remoteName (as configured via `lxc
// remote add`), scoped to ProjectName().
func ConnectRemoteServer(remoteName string) (lxd.InstanceServer, error) {
	return connectNamedRemote(remoteName, ProjectName())
}

// connectLocalLXD connects to the special "local" remote (the LXD daemon
// running on this test host, reached over its unix socket, independent of
// REMOTE), scoped to LocalProjectName().
func connectLocalLXD() (lxd.InstanceServer, error) {
	return connectNamedRemote("local", LocalProjectName())
}

// localServer caches LocalServer's connection, avoiding reconnecting and
// re-parsing the LXD client config from disk on every validateLocalLXDProject
// or LXDController call.
var localServer lxd.InstanceServer

// LocalServer returns a cached connection to the local LXD daemon,
// connecting lazily on first use and shared across all callers for the
// remainder of the process.
func LocalServer() (lxd.InstanceServer, error) {
	if localServer != nil {
		return localServer, nil
	}

	s, err := connectLocalLXD()
	if err != nil {
		return nil, err
	}

	localServer = s

	return localServer, nil
}

// connectNamedRemote connects to remoteName (as configured via `lxc remote
// add`, or the special "local" remote), scoped to project.
func connectNamedRemote(remoteName, project string) (lxd.InstanceServer, error) {
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
