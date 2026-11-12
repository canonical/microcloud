package lxd

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/canonical/lxd/lxc/config"
)

// loadLXCConfig loads the lxc client configuration from configDir, falling back to a fresh
// in-memory default configuration if none exists yet on disk.
func loadLXCConfig(configDir string) (*config.Config, error) {
	confPath := filepath.Join(configDir, "config.yml")

	conf, err := config.LoadConfig(confPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("Cannot load LXD client configuration from %q: %w", confPath, err)
		}

		return config.NewConfig(configDir, true), nil
	}

	return conf, nil
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
