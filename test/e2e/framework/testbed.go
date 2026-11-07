package framework

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/canonical/lxd/shared/api"
	//nolint:staticcheck // Ginkgo's DSL is conventionally dot-imported.
	. "github.com/onsi/ginkgo/v2"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

// expandHome replaces a leading "~" (or "~/...") in path with the current
// user's home directory, since paths in a YAML testbed file (e.g.
// private_key_file: ~/.ssh/id_ed25519) are not shell-expanded for us.
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

// defaultSSHPort is used for any SSHNodeConfig that doesn't specify a port.
const defaultSSHPort = 22

// TestbedFilePath returns the path to the testbed config file from the
// E2E_TESTBED_FILE environment variable.
func TestbedFilePath() string {
	return os.Getenv("E2E_TESTBED_FILE")
}

// SSHNodeConfig describes how to reach a single cluster member over SSH.
type SSHNodeConfig struct {
	Address        string `yaml:"address"`
	Port           int    `yaml:"port"`
	User           string `yaml:"user"`
	PrivateKeyFile string `yaml:"private_key_file"`
}

// SSHConfig configures the SSHController.
type SSHConfig struct {
	Nodes map[string]SSHNodeConfig `yaml:"nodes"`
}

// LXDConfig configures the LXDController.
type LXDConfig struct {
	Project string `yaml:"project"`
}

// TestbedConfig is the top-level shape of the YAML file pointed to by
// TestbedFilePath().
type TestbedConfig struct {
	LXD *LXDConfig `yaml:"lxd"`
	SSH *SSHConfig `yaml:"ssh"`
}

// testbedConfig is the TestbedConfig loaded once by LoadTestbedConfig and
// cached for the remainder of the suite run.
var testbedConfig TestbedConfig

// Testbed returns the config loaded by LoadTestbedConfig.
func Testbed() TestbedConfig {
	return testbedConfig
}

// SetTestbed overrides the cached TestbedConfig.
func SetTestbed(cfg TestbedConfig) {
	testbedConfig = cfg
}

// LoadTestbedConfig reads and parses the file at TestbedFilePath(), if any.
func LoadTestbedConfig() error {
	path := TestbedFilePath()
	if path == "" {
		SetTestbed(TestbedConfig{})
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("Cannot read testbed file %q: %w", path, err)
	}

	var cfg TestbedConfig

	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return fmt.Errorf("Cannot parse testbed file %q: %w", path, err)
	}

	if cfg.LXD != nil && cfg.LXD.Project == "" {
		return fmt.Errorf("Testbed file %q: lxd: project is required", path)
	}

	if cfg.SSH != nil && len(cfg.SSH.Nodes) == 0 {
		return fmt.Errorf("Testbed file %q: ssh: at least one node under ssh.nodes is required", path)
	}

	if cfg.SSH != nil {
		for member, node := range cfg.SSH.Nodes {
			err = requireField(path, member, "address", node.Address)
			if err != nil {
				return err
			}

			err = requireField(path, member, "user", node.User)
			if err != nil {
				return err
			}

			err = requireField(path, member, "private_key_file", node.PrivateKeyFile)
			if err != nil {
				return err
			}

			keyFile, err := expandHome(node.PrivateKeyFile)
			if err != nil {
				return fmt.Errorf("Testbed file %q: ssh.nodes.%s: cannot resolve private_key_file %q: %w", path, member, node.PrivateKeyFile, err)
			}

			node.PrivateKeyFile = keyFile

			keyData, err := os.ReadFile(node.PrivateKeyFile)
			if err != nil {
				return fmt.Errorf("Testbed file %q: ssh.nodes.%s: cannot read private_key_file %q: %w", path, member, node.PrivateKeyFile, err)
			}

			_, err = ssh.ParsePrivateKey(keyData)
			if err != nil {
				return fmt.Errorf("Testbed file %q: ssh.nodes.%s: cannot parse private_key_file %q: %w", path, member, node.PrivateKeyFile, err)
			}

			if node.Port == 0 {
				node.Port = defaultSSHPort
			}

			cfg.SSH.Nodes[member] = node
		}
	}

	SetTestbed(cfg)

	return nil
}

// requireField returns an error if value is empty, naming the missing field by
// its testbed file path, member, and field name.
func requireField(path, member, field, value string) error {
	if value == "" {
		return fmt.Errorf("Testbed file %q: ssh.nodes.%s: %s is required", path, member, field)
	}

	return nil
}

// validateLocalLXDProject reports whether every given cluster member name
// corresponds to a local LXD instance in LocalProjectName() -- i.e.  whether a
// testbed file's lxd.project claim actually holds, checked against the local
// LXD daemon (independent of REMOTE). Returns false if the local daemon is
// unreachable or any member doesn't match, logging the reason to GinkgoWriter
// either way.
func validateLocalLXDProject(members []string) bool {
	local, err := LocalServer()
	if err != nil {
		GinkgoWriter.Printf("lxd testbed validation: local LXD daemon not reachable (%v)\n", err)
		return false
	}

	names, err := local.GetInstanceNames(api.InstanceTypeAny)
	if err != nil {
		GinkgoWriter.Printf("lxd testbed validation: cannot list instances in local project %q (%v)\n", LocalProjectName(), err)
		return false
	}

	missing, ok := membersMatchInstances(members, names)
	if !ok {
		GinkgoWriter.Printf("lxd testbed validation: no local LXD instance named %q in project %q\n", missing, LocalProjectName())
		return false
	}

	return true
}

// membersMatchInstances reports whether every name in members is present in
// instanceNames. If not, it also returns the first missing member name for use
// in diagnostics; ok is true iff every member matched.
func membersMatchInstances(members, instanceNames []string) (missing string, ok bool) {
	present := make(map[string]bool, len(instanceNames))
	for _, n := range instanceNames {
		present[n] = true
	}

	for _, m := range members {
		if !present[m] {
			return m, false
		}
	}

	return "", true
}

// validateSSHNodes reports whether every node configured in nodes is reachable
// over SSH -- i.e. whether a testbed file's ssh.nodes claims actually hold --
// dialing each once (see SSHController.dial) and closing the connection
// immediately. Returns false at the first unreachable node, logging the reason
// to GinkgoWriter either way.
func validateSSHNodes(nodes map[string]SSHNodeConfig) bool {
	c := newSSHController(nodes)

	for member := range nodes {
		client, err := c.dial(member)
		if err != nil {
			GinkgoWriter.Printf("ssh testbed validation: cannot connect to %q (%v)\n", member, err)
			return false
		}

		client.Close()
	}

	return true
}
