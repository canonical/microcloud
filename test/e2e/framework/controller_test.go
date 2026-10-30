package framework

import "testing"

// resetControllerState clears the package-level state resolveController
// depends on/caches, restoring it on test cleanup so tests don't leak into
// each other.
func resetControllerState(t *testing.T) {
	t.Helper()

	prevController := controller
	prevEnv := detectedEnv
	prevTestbed := testbedConfig

	controller = nil
	detectedEnv = EnvUnknown
	testbedConfig = TestbedConfig{}

	t.Cleanup(func() {
		controller = prevController
		detectedEnv = prevEnv
		testbedConfig = prevTestbed
	})
}

func TestResolveControllerExplicitSSH(t *testing.T) {
	resetControllerState(t)

	SetTestbed(TestbedConfig{
		SSH: &SSHConfig{Nodes: map[string]SSHNodeConfig{
			"micro1": {Address: "10.0.0.11", User: "ubuntu", PrivateKeyFile: "/dev/null"},
		}},
	})
	SetEnv(EnvSSH)

	c, err := resolveController()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := c.(*SSHController); !ok {
		t.Fatalf("expected an *SSHController, got %T", c)
	}
}

func TestResolveControllerUnknownEnvFails(t *testing.T) {
	resetControllerState(t)

	_, err := resolveController()
	if err == nil {
		t.Fatal("expected an error when Env() is EnvUnknown")
	}
}

func TestResolveControllerCaches(t *testing.T) {
	resetControllerState(t)

	SetTestbed(TestbedConfig{
		SSH: &SSHConfig{Nodes: map[string]SSHNodeConfig{
			"micro1": {Address: "10.0.0.11", User: "ubuntu", PrivateKeyFile: "/dev/null"},
		}},
	})
	SetEnv(EnvSSH)

	first, err := resolveController()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Clearing Testbed()/Env() after the first resolution must not affect
	// the cached result.
	SetTestbed(TestbedConfig{})
	SetEnv(EnvUnknown)

	second, err := resolveController()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if first != second {
		t.Fatal("expected resolveController to return the cached controller")
	}
}
