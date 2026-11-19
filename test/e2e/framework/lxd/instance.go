package lxd

import (
	"fmt"
	"math/rand"

	"github.com/canonical/lxd/shared/api"
)

// RandomInstanceName returns a random instance name prefixed with prefix, safe to use concurrently
// across specs.
func RandomInstanceName(prefix string) string {
	return fmt.Sprintf("%s%06d", prefix, rand.Intn(1_000_000))
}

// Spec describes the instance LaunchInstance should create.
type Spec struct {
	// Name is the instance's name.
	Name string
	// Type is the instance type (container or VM).
	Type api.InstanceType
	// Target is the cluster member to launch the instance on.
	Target string
	// Fingerprint is the source image's fingerprint.
	Fingerprint string
	// Profile is the single profile to apply to the instance.
	Profile string
}

// LaunchInstance creates and starts an instance from spec on conn.
func LaunchInstance(conn *Connection, spec Spec) error {
	op, err := conn.Server().UseTarget(spec.Target).CreateInstance(api.InstancesPost{
		Name: spec.Name,
		Type: spec.Type,
		Source: api.InstanceSource{
			Type:        "image",
			Fingerprint: spec.Fingerprint,
		},
		InstancePut: api.InstancePut{
			Profiles: []string{spec.Profile},
		},
		Start: true,
	})
	if err != nil {
		return fmt.Errorf("Cannot create instance %q on %q: %w", spec.Name, spec.Target, err)
	}

	err = op.Wait()
	if err != nil {
		return fmt.Errorf("Cannot create instance %q on %q: %w", spec.Name, spec.Target, err)
	}

	return nil
}

// DeleteInstance stops (if running) and deletes the instance on conn. It is safe to call
// unconditionally from DeferCleanup: a "not found" error is not treated as a failure.
func DeleteInstance(conn *Connection, name string) error {
	state, _, err := conn.Server().GetInstanceState(name)
	if err != nil {
		if api.StatusErrorCheck(err, 404) {
			return nil
		}

		return fmt.Errorf("Cannot get state of instance %q: %w", name, err)
	}

	if state.Status == api.Running.String() {
		op, err := conn.Server().UpdateInstanceState(name, api.InstanceStatePut{Action: "stop", Force: true}, "")
		if err != nil {
			return fmt.Errorf("Cannot stop instance %q: %w", name, err)
		}

		err = op.Wait()
		if err != nil {
			return fmt.Errorf("Cannot stop instance %q: %w", name, err)
		}
	}

	op, err := conn.Server().DeleteInstance(name, false)
	if err != nil {
		if api.StatusErrorCheck(err, 404) {
			return nil
		}

		return fmt.Errorf("Cannot delete instance %q: %w", name, err)
	}

	err = op.Wait()
	if err != nil {
		return fmt.Errorf("Cannot delete instance %q: %w", name, err)
	}

	return nil
}

// GetInstanceNames returns the names of all instances in conn's project.
func GetInstanceNames(conn *Connection) ([]string, error) {
	names, err := conn.Server().GetInstanceNames(api.InstanceTypeAny)
	if err != nil {
		return nil, fmt.Errorf("Cannot list instances in project=%s: %w", conn.Project(), err)
	}

	return names, nil
}
