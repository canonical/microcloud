package testbed

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

// defaultSSHPort is used for any SSHNodeConfig that doesn't specify a port.
const defaultSSHPort = 22

// SSHNodeConfig describes how to reach a single cluster member over SSH.
type SSHNodeConfig struct {
	Address        string `yaml:"address"`
	Port           int    `yaml:"port"`
	User           string `yaml:"user"`
	PrivateKeyFile string `yaml:"private_key_file"`
}

// SSHConfig configures SSH-based node control.
type SSHConfig struct {
	Nodes map[string]SSHNodeConfig `yaml:"nodes"`
}

// LXDConfig configures LXD-based node control.
type LXDConfig struct {
	Project string `yaml:"project"`
}

// Config is the top-level shape of a testbed file (see FilePath).
type Config struct {
	LXD *LXDConfig `yaml:"lxd"`
	SSH *SSHConfig `yaml:"ssh"`
}

// Load reads and parses the file at path, if any.
func Load(path string) (Config, error) {
	if path == "" {
		return Config{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("Cannot read testbed file %q: %w", path, err)
	}

	var cfg Config

	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("Cannot parse testbed file %q: %w", path, err)
	}

	if cfg.LXD != nil && cfg.LXD.Project == "" {
		return Config{}, fmt.Errorf("Cannot load testbed file %q: lxd.project is required", path)
	}

	if cfg.SSH != nil && len(cfg.SSH.Nodes) == 0 {
		return Config{}, fmt.Errorf("Cannot load testbed file %q: at least one node under ssh.nodes is required", path)
	}

	if cfg.SSH != nil {
		for member, node := range cfg.SSH.Nodes {
			node, err = validateSSHNode(path, member, node)
			if err != nil {
				return Config{}, err
			}

			cfg.SSH.Nodes[member] = node
		}
	}

	return cfg, nil
}

// validateSSHNode requires every mandatory field on node, resolves and parses its
// private_key_file, and defaults its port.
func validateSSHNode(path, member string, node SSHNodeConfig) (SSHNodeConfig, error) {
	err := requireField(path, member, "address", node.Address)
	if err != nil {
		return SSHNodeConfig{}, err
	}

	err = requireField(path, member, "user", node.User)
	if err != nil {
		return SSHNodeConfig{}, err
	}

	err = requireField(path, member, "private_key_file", node.PrivateKeyFile)
	if err != nil {
		return SSHNodeConfig{}, err
	}

	keyFile, err := expandHome(node.PrivateKeyFile)
	if err != nil {
		return SSHNodeConfig{}, fmt.Errorf("Cannot resolve ssh.nodes.%s.private_key_file %q in testbed file %q: %w", member, node.PrivateKeyFile, path, err)
	}

	node.PrivateKeyFile = keyFile

	keyData, err := os.ReadFile(node.PrivateKeyFile)
	if err != nil {
		return SSHNodeConfig{}, fmt.Errorf("Cannot read ssh.nodes.%s.private_key_file %q in testbed file %q: %w", member, node.PrivateKeyFile, path, err)
	}

	_, err = ssh.ParsePrivateKey(keyData)
	if err != nil {
		return SSHNodeConfig{}, fmt.Errorf("Cannot parse ssh.nodes.%s.private_key_file %q in testbed file %q: %w", member, node.PrivateKeyFile, path, err)
	}

	if node.Port == 0 {
		node.Port = defaultSSHPort
	}

	return node, nil
}

// requireField returns an error if value is empty, naming the missing field by its testbed file
// path, member, and field name.
func requireField(path, member, field, value string) error {
	if value == "" {
		return fmt.Errorf("Cannot load testbed file %q: ssh.nodes.%s.%s is required", path, member, field)
	}

	return nil
}

// expandHome replaces a leading "~" (or "~/...") in path with the current user's home directory,
// since paths in a YAML testbed file (e.g. private_key_file: ~/.ssh/id_ed25519) are not
// shell-expanded for us.
func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}

	home, err := homeDir()
	if err != nil {
		return "", err
	}

	if path == "~" {
		return home, nil
	}

	return filepath.Join(home, path[2:]), nil
}

// homeDir returns the current user's home directory. Deliberately a private copy independent of
// the one in the lxd package: this package must stay a zero-dependency leaf.
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
