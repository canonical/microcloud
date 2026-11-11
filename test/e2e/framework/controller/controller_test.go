package controller

import (
	"testing"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework/testbed"
)

func TestResolveExplicitSSH(t *testing.T) {
	c := New(nil, testbed.Config{
		SSH: &testbed.SSHConfig{Nodes: map[string]testbed.SSHNodeConfig{
			"micro1": {Address: "10.0.0.11", User: "ubuntu", PrivateKeyFile: "/dev/null"},
		}},
	})
	c.env = EnvSSH

	nc, err := c.Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := nc.(*SSHController)
	if !ok {
		t.Fatalf("expected an *SSHController, got %T", nc)
	}
}

func TestResolveUnknownEnvFails(t *testing.T) {
	c := New(nil, testbed.Config{})

	_, err := c.Resolve()
	if err == nil {
		t.Fatal("expected an error when Env() is EnvUnknown")
	}
}

func TestResolveCaches(t *testing.T) {
	c := New(nil, testbed.Config{
		SSH: &testbed.SSHConfig{Nodes: map[string]testbed.SSHNodeConfig{
			"micro1": {Address: "10.0.0.11", User: "ubuntu", PrivateKeyFile: "/dev/null"},
		}},
	})
	c.env = EnvSSH

	first, err := c.Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Clearing env after the first resolution must not affect the cached
	// result.
	c.env = EnvUnknown

	second, err := c.Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if first != second {
		t.Fatal("expected Resolve to return the cached controller")
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
