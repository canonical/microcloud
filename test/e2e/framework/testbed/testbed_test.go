package testbed

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

// generateTestKeyPair returns a freshly generated ed25519 private key,
// encoded as an OpenSSH-format PEM block. Deliberately a private copy
// independent of the one in framework/controller's test suite: the two
// packages' tests don't share any code.
func generateTestKeyPair(t *testing.T) []byte {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Cannot generate test key pair: %v", err)
	}

	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("Cannot marshal test private key: %v", err)
	}

	return pem.EncodeToMemory(block)
}

// writeTestKey generates a fresh test key pair, writes its private key under
// dir, and returns the path.
func writeTestKey(t *testing.T, dir string) string {
	t.Helper()

	keyPEM := generateTestKeyPair(t)
	path := filepath.Join(dir, "id_test")

	err := os.WriteFile(path, keyPEM, 0o600)
	if err != nil {
		t.Fatalf("Cannot write key file: %v", err)
	}

	return path
}

func TestLoadNoPathIsNotAnError(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.LXD != nil {
		t.Fatalf("expected no lxd config, got %v", cfg.LXD)
	}

	if cfg.SSH != nil {
		t.Fatalf("expected no ssh config, got %v", cfg.SSH)
	}
}

func TestLoadValid(t *testing.T) {
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

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nodes := cfg.SSH.Nodes
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

func TestLoadMissingRequiredField(t *testing.T) {
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

	_, err = Load(path)
	if err == nil {
		t.Fatal("expected an error for a missing address field")
	}
}

func TestLoadBadPrivateKeyFile(t *testing.T) {
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

	_, err = Load(path)
	if err == nil {
		t.Fatal("expected an error for an unparseable private key file")
	}
}

// writeTestbedFile writes content to a testbed.yaml under t.TempDir() and
// returns its path.
func writeTestbedFile(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "testbed.yaml")

	err := os.WriteFile(path, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("Cannot write testbed file: %v", err)
	}

	return path
}

func TestLoadLXDValid(t *testing.T) {
	path := writeTestbedFile(t, "lxd:\n  project: my-project\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.LXD == nil {
		t.Fatal("expected a non-nil LXD config")
	}

	if cfg.LXD.Project != "my-project" {
		t.Fatalf("expected project %q, got %q", "my-project", cfg.LXD.Project)
	}

	if cfg.SSH != nil {
		t.Fatalf("expected no ssh config, got %v", cfg.SSH)
	}
}

func TestLoadLXDMissingProject(t *testing.T) {
	path := writeTestbedFile(t, "lxd:\n  project: \"\"\n")

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected an error for a missing lxd.project")
	}
}

func TestLoadSSHEmptyNodes(t *testing.T) {
	path := writeTestbedFile(t, "ssh:\n  nodes: {}\n")

	_, err := Load(path)
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
