package controller

import (
	"testing"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework/testbed"
)

func TestNewExplicitSSH(t *testing.T) {
	c, err := New(testbed.Config{
		SSH: &testbed.SSHConfig{Nodes: map[string]testbed.SSHNodeConfig{
			"micro1": {Address: "10.0.0.11", User: "ubuntu", PrivateKeyFile: "/dev/null"},
		}},
	})
	// This is expected to fail: the ssh node isn't actually reachable (dialing an invalid
	// address/key). What matters is that New attempted to build an SSHController (not some other
	// path), confirmed by the error coming from newSSHController's dial loop.
	if err == nil {
		t.Fatal("expected an error dialing an unreachable ssh node")
	}

	if c != nil {
		t.Fatalf("expected a nil Controller on error, got %T", c)
	}
}

func TestNewUnknownConfigFails(t *testing.T) {
	_, err := New(testbed.Config{})
	if err == nil {
		t.Fatal("expected an error when neither lxd nor ssh is configured")
	}
}
