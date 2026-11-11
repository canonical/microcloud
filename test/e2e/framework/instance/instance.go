// Package instance launches, controls, and tears down LXD instances used by
// e2e scenarios, and checks reachability between them.
package instance

import (
	"fmt"
	"math/rand"

	lxdclient "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework/lxd"
)

// Manager launches, controls, and tears down LXD instances on a Connection.
//
// A Manager is constructed once per Ginkgo process by framework.SetupProcess
// (test/e2e/framework/framework.go) and exposed to scenario packages via DefaultManager.
type Manager struct {
	conn *lxd.Connection
}

// DefaultManager is the Manager constructed by framework.SetupProcess, for scenario packages to
// use. It is nil until SetupProcess (run via SynchronizedBeforeSuite) assigns it.
var DefaultManager *Manager

// New returns a Manager operating on conn.
func New(conn *lxd.Connection) *Manager {
	return &Manager{conn: conn}
}

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

// LaunchInstance creates and starts an instance from spec.
func (m *Manager) LaunchInstance(spec Spec) error {
	op, err := m.conn.Server().UseTarget(spec.Target).CreateInstance(api.InstancesPost{
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

// LaunchMixedFleet launches one instance per cluster member, alternating container/VM in member
// order, each named RandomInstanceName(prefix). It uses the fingerprints returned by
// environment.Bootstrapper.EnsureBaseImages. If any launch fails, any instances already launched
// are deleted before returning the error.
func (m *Manager) LaunchMixedFleet(prefix string, containerFingerprint, vmFingerprint string, profileName string) ([]string, error) {
	members, err := m.conn.Server().GetClusterMembers()
	if err != nil {
		return nil, fmt.Errorf("Cannot list cluster members: %w", err)
	}

	names := make([]string, 0, len(members))
	for i, member := range members {
		name := RandomInstanceName(prefix)

		instanceType := api.InstanceTypeContainer
		fingerprint := containerFingerprint
		if i%2 == 1 {
			instanceType = api.InstanceTypeVM
			fingerprint = vmFingerprint
		}

		err := m.LaunchInstance(Spec{
			Name:        name,
			Type:        instanceType,
			Target:      member.ServerName,
			Fingerprint: fingerprint,
			Profile:     profileName,
		})
		if err != nil {
			// Best-effort cleanup of whatever was already launched: the original launch error
			// above is what's returned/matters here, so each deletion's own error is discarded
			// rather than masking it (DeleteInstance already tolerates "not found").
			for _, n := range names {
				_ = m.DeleteInstance(n)
			}

			return nil, err
		}

		names = append(names, name)
	}

	return names, nil
}

// DeleteInstance stops (if running) and deletes the instance. It is safe to call unconditionally
// from DeferCleanup: a "not found" error is not treated as a failure.
func (m *Manager) DeleteInstance(name string) error {
	state, _, err := m.conn.Server().GetInstanceState(name)
	if err != nil {
		if api.StatusErrorCheck(err, 404) {
			return nil
		}

		return fmt.Errorf("Cannot get state of instance %q: %w", name, err)
	}

	if state.Status == api.Running.String() {
		op, err := m.conn.Server().UpdateInstanceState(name, api.InstanceStatePut{Action: "stop", Force: true}, "")
		if err != nil {
			return fmt.Errorf("Cannot stop instance %q: %w", name, err)
		}

		err = op.Wait()
		if err != nil {
			return fmt.Errorf("Cannot stop instance %q: %w", name, err)
		}
	}

	op, err := m.conn.Server().DeleteInstance(name, false)
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

// connectionProject returns the project m.conn.Server() is scoped to, for use in error messages.
// It falls back to "" if the connection info is unavailable.
func (m *Manager) connectionProject() string {
	info, err := m.conn.Server().GetConnectionInfo()
	if err != nil {
		return ""
	}

	return info.Project
}

// GetInstanceNames returns the names of all instances in the project m.conn.Server() is scoped to.
func (m *Manager) GetInstanceNames() ([]string, error) {
	names, err := m.conn.Server().GetInstanceNames(api.InstanceTypeAny)
	if err != nil {
		return nil, fmt.Errorf("Cannot list instances in project=%s: %w", m.connectionProject(), err)
	}

	return names, nil
}

// GetInstanceNamesByMember returns the names of instances located on member, in the project
// m.conn.Server() is scoped to.
func (m *Manager) GetInstanceNamesByMember(member string) ([]string, error) {
	instances, err := m.conn.Server().GetInstances(lxdclient.GetInstancesArgs{})
	if err != nil {
		return nil, fmt.Errorf("Cannot list instances in member=%s project=%s: %w", member, m.connectionProject(), err)
	}

	names := make([]string, 0, len(instances))
	for _, i := range instances {
		if i.Location != member {
			continue
		}

		names = append(names, i.Name)
	}

	return names, nil
}

// GetInstanceIPv4 returns the first global-scope IPv4 address reported for the instance, or "" if
// it has none yet.
func (m *Manager) GetInstanceIPv4(name string) (string, error) {
	state, _, err := m.conn.Server().GetInstanceState(name)
	if err != nil {
		return "", fmt.Errorf("Cannot get state of instance %q: %w", name, err)
	}

	for _, net := range state.Network {
		for _, addr := range net.Addresses {
			if addr.Family == "inet" && addr.Scope == "global" {
				return addr.Address, nil
			}
		}
	}

	return "", nil
}

// GetInstanceStatus returns the instance's current status (e.g. "Running", "Stopped").
func (m *Manager) GetInstanceStatus(name string) (string, error) {
	state, _, err := m.conn.Server().GetInstanceState(name)
	if err != nil {
		return "", fmt.Errorf("Cannot get state of instance %q: %w", name, err)
	}

	return state.Status, nil
}

// EnsureInstanceRunning starts the instance if it's currently stopped.
func (m *Manager) EnsureInstanceRunning(name string) error {
	state, _, err := m.conn.Server().GetInstanceState(name)
	if err != nil {
		return fmt.Errorf("Cannot get state of instance %q: %w", name, err)
	}

	if state.Status == api.Running.String() {
		return nil
	}

	op, err := m.conn.Server().UpdateInstanceState(name, api.InstanceStatePut{Action: "start"}, "")
	if err != nil {
		return fmt.Errorf("Cannot start instance %q: %w", name, err)
	}

	err = op.Wait()
	if err != nil {
		return fmt.Errorf("Cannot start instance %q: %w", name, err)
	}

	return nil
}

// ExecInInstance runs argv inside the named instance on m.conn.Server() and waits for it to
// complete. A non-zero exit status is not an error: it is returned as exitCode along with the
// captured output. err is only set when the command could not be started or its result could not
// be determined.
func (m *Manager) ExecInInstance(instance string, argv []string, env map[string]string) (stdout string, stderr string, exitCode int, err error) {
	return lxd.ExecInInstanceOn(m.conn.Server(), instance, argv, env)
}
