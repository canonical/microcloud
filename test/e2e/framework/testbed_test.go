package framework

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTestKey generates a fresh test key pair, writes its private key under
// dir, and returns the path.
func writeTestKey(t *testing.T, dir string) string {
	t.Helper()

	_, keyPEM := generateTestKeyPair(t)

	return writeKeyFile(t, dir, keyPEM)
}

func TestLoadTestbedConfigNoPathIsNotAnError(t *testing.T) {
	t.Setenv("E2E_TESTBED_FILE", "")

	err := LoadTestbedConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if Testbed().LXD != nil {
		t.Fatalf("expected no lxd config, got %v", Testbed().LXD)
	}

	if Testbed().SSH != nil {
		t.Fatalf("expected no ssh config, got %v", Testbed().SSH)
	}
}

func TestLoadTestbedConfigValid(t *testing.T) {
	dir := t.TempDir()
	keyPath := writeTestKey(t, dir)

	content := `
ssh:
  nodes:
    micro1:
      address: 10.0.0.11
      user: ubuntu
      private_key_file: ` + keyPath + `
    micro2:
      address: 10.0.0.12
      port: 2222
      user: root
      private_key_file: ` + keyPath + `
`
	path := filepath.Join(dir, "testbed.yaml")

	err := os.WriteFile(path, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("Cannot write testbed file: %v", err)
	}

	t.Setenv("E2E_TESTBED_FILE", path)
	t.Cleanup(func() { SetTestbed(TestbedConfig{}) })

	err = LoadTestbedConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nodes := Testbed().SSH.Nodes
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	if nodes["micro1"].Port != defaultSSHPort {
		t.Fatalf("expected default port %d, got %d", defaultSSHPort, nodes["micro1"].Port)
	}

	if nodes["micro2"].Port != 2222 {
		t.Fatalf("expected port 2222, got %d", nodes["micro2"].Port)
	}
}

func TestLoadTestbedConfigMissingRequiredField(t *testing.T) {
	dir := t.TempDir()
	keyPath := writeTestKey(t, dir)

	content := `
ssh:
  nodes:
    micro1:
      user: ubuntu
      private_key_file: ` + keyPath + `
`
	path := filepath.Join(dir, "testbed.yaml")

	err := os.WriteFile(path, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("Cannot write testbed file: %v", err)
	}

	t.Setenv("E2E_TESTBED_FILE", path)

	err = LoadTestbedConfig()
	if err == nil {
		t.Fatal("expected an error for a missing address field")
	}
}

func TestLoadTestbedConfigBadPrivateKeyFile(t *testing.T) {
	dir := t.TempDir()
	badKeyPath := filepath.Join(dir, "not-a-key")

	err := os.WriteFile(badKeyPath, []byte("not a private key"), 0o600)
	if err != nil {
		t.Fatalf("Cannot write bad key file: %v", err)
	}

	content := `
ssh:
  nodes:
    micro1:
      address: 10.0.0.11
      user: ubuntu
      private_key_file: ` + badKeyPath + `
`
	path := filepath.Join(dir, "testbed.yaml")

	err = os.WriteFile(path, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("Cannot write testbed file: %v", err)
	}

	t.Setenv("E2E_TESTBED_FILE", path)

	err = LoadTestbedConfig()
	if err == nil {
		t.Fatal("expected an error for an unparseable private key file")
	}
}

// writeTestbedFile writes content to a testbed.yaml under t.TempDir() and
// points E2E_TESTBED_FILE at it.
func writeTestbedFile(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "testbed.yaml")

	err := os.WriteFile(path, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("Cannot write testbed file: %v", err)
	}

	t.Setenv("E2E_TESTBED_FILE", path)

	return path
}

func TestLoadTestbedConfigLXDValid(t *testing.T) {
	writeTestbedFile(t, "lxd:\n  project: my-project\n")

	err := LoadTestbedConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if Testbed().LXD == nil {
		t.Fatal("expected a non-nil LXD config")
	}

	if Testbed().LXD.Project != "my-project" {
		t.Fatalf("expected project %q, got %q", "my-project", Testbed().LXD.Project)
	}

	if Testbed().SSH != nil {
		t.Fatalf("expected no ssh config, got %v", Testbed().SSH)
	}
}

func TestLoadTestbedConfigLXDMissingProject(t *testing.T) {
	writeTestbedFile(t, "lxd:\n  project: \"\"\n")

	err := LoadTestbedConfig()
	if err == nil {
		t.Fatal("expected an error for a missing lxd.project")
	}
}

func TestLoadTestbedConfigSSHEmptyNodes(t *testing.T) {
	writeTestbedFile(t, "ssh:\n  nodes: {}\n")

	err := LoadTestbedConfig()
	if err == nil {
		t.Fatal("expected an error for an ssh section with no nodes")
	}
}

func TestExpandHome(t *testing.T) {
	home, err := homeDir()
	if err != nil {
		t.Skipf("cannot determine home dir: %v", err)
	}

	got, err := expandHome("~/.ssh/id_ed25519")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := filepath.Join(home, ".ssh/id_ed25519")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}

	// Paths without a leading ~ are untouched.
	got, err = expandHome("/absolute/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != "/absolute/path" {
		t.Fatalf("expected unchanged path, got %q", got)
	}
}

func TestMembersMatchInstancesAllPresent(t *testing.T) {
	missing, ok := membersMatchInstances(
		[]string{"micro1", "micro2", "micro3"},
		[]string{"micro3", "micro1", "other", "micro2"},
	)
	if !ok {
		t.Fatalf("expected ok=true, got missing=%q", missing)
	}
}

func TestMembersMatchInstancesMissingOne(t *testing.T) {
	missing, ok := membersMatchInstances(
		[]string{"micro1", "micro2", "micro3"},
		[]string{"micro1", "micro2"},
	)
	if ok {
		t.Fatal("expected ok=false")
	}

	if missing != "micro3" {
		t.Fatalf("expected missing=%q, got %q", "micro3", missing)
	}
}

func TestMembersMatchInstancesEmptyInstances(t *testing.T) {
	missing, ok := membersMatchInstances([]string{"micro1"}, nil)
	if ok {
		t.Fatal("expected ok=false")
	}

	if missing != "micro1" {
		t.Fatalf("expected missing=%q, got %q", "micro1", missing)
	}
}

func TestMembersMatchInstancesEmptyMembers(t *testing.T) {
	_, ok := membersMatchInstances(nil, []string{"micro1"})
	if !ok {
		t.Fatal("expected ok=true for an empty members list")
	}
}

func TestValidateSSHNodesUnreachable(t *testing.T) {
	nodes := map[string]SSHNodeConfig{
		"micro1": {Address: "10.0.0.11", User: "ubuntu", PrivateKeyFile: "/nonexistent"},
	}

	if validateSSHNodes(nodes) {
		t.Fatal("expected validateSSHNodes to fail for a bad private key file")
	}
}
