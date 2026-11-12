package lxd

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	lxdclient "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
)

// sshProbePort is the TCP port probed by ProbeSSHReachability to check reachability between
// instances.
const sshProbePort = 22

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

// LaunchMixedInstances launches one instance per cluster member on conn, alternating container/VM
// in member order, each named RandomInstanceName(prefix). It uses the fingerprints returned by
// environment.Provisioner.EnsureBaseImages. If any launch fails, any instances already launched
// are deleted before returning the error.
func LaunchMixedInstances(conn *Connection, prefix string, containerFingerprint, vmFingerprint string, profileName string) ([]string, error) {
	members, err := conn.Server().GetClusterMembers()
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

		err := LaunchInstance(conn, Spec{
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
				_ = DeleteInstance(conn, n)
			}

			return nil, err
		}

		names = append(names, name)
	}

	return names, nil
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

// GetInstanceNamesByMember returns the names of instances located on member, in conn's project.
func GetInstanceNamesByMember(conn *Connection, member string) ([]string, error) {
	instances, err := conn.Server().GetInstances(lxdclient.GetInstancesArgs{})
	if err != nil {
		return nil, fmt.Errorf("Cannot list instances in member=%s project=%s: %w", member, conn.Project(), err)
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

// GetInstanceIPv4 returns the first global-scope IPv4 address reported for the instance on conn,
// or "" if it has none yet.
func GetInstanceIPv4(conn *Connection, name string) (string, error) {
	state, _, err := conn.Server().GetInstanceState(name)
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

// GetInstanceStatus returns the instance's current status (e.g. "Running", "Stopped") on conn.
func GetInstanceStatus(conn *Connection, name string) (string, error) {
	state, _, err := conn.Server().GetInstanceState(name)
	if err != nil {
		return "", fmt.Errorf("Cannot get state of instance %q: %w", name, err)
	}

	return state.Status, nil
}

// EnsureInstanceRunning starts the instance on conn if it's currently stopped.
func EnsureInstanceRunning(conn *Connection, name string) error {
	state, _, err := conn.Server().GetInstanceState(name)
	if err != nil {
		return fmt.Errorf("Cannot get state of instance %q: %w", name, err)
	}

	if state.Status == api.Running.String() {
		return nil
	}

	op, err := conn.Server().UpdateInstanceState(name, api.InstanceStatePut{Action: "start"}, "")
	if err != nil {
		return fmt.Errorf("Cannot start instance %q: %w", name, err)
	}

	err = op.Wait()
	if err != nil {
		return fmt.Errorf("Cannot start instance %q: %w", name, err)
	}

	return nil
}

// ProbeSSHReachability attempts, once, to open a TCP connection from inside src to dst's SSH port,
// returning the combined stdout/stderr output. It returns an error if dst has no IPv4 address yet,
// the probe could not be started, or it exited non-zero -- callers are expected to wrap this in
// their own retry logic (e.g. Gomega's Eventually) while dst is still booting.
func ProbeSSHReachability(conn *Connection, src, dst string) (output string, err error) {
	dstIPv4, err := GetInstanceIPv4(conn, dst)
	if err != nil {
		return "", err
	}

	if dstIPv4 == "" {
		return "", fmt.Errorf("Instance %q has no IPv4 address yet", dst)
	}

	// The connection attempt is bounded to probeTimeout: bash's /dev/tcp has no timeout of its
	// own, and a genuinely unreachable destination (as opposed to one that actively refuses the
	// connection) would otherwise block for the kernel's default TCP SYN-retry duration (commonly
	// ~2 minutes), which would blow through a caller's retry budget in a single attempt.
	const probeTimeout = 5 * time.Second

	command := []string{"timeout", strconv.Itoa(int(probeTimeout.Seconds())), "bash", "-c",
		fmt.Sprintf("grep -qm1 ^SSH- < /dev/tcp/%s/%d", dstIPv4, sshProbePort)}

	stdout, stderr, rc, err := ExecInInstanceOn(conn.Server(), src, command, nil)
	output = stdout + stderr
	if err != nil {
		return output, fmt.Errorf("Cannot probe from %q to %s:%d: %w", src, dstIPv4, sshProbePort, err)
	}

	if rc != 0 {
		return output, fmt.Errorf("Cannot probe from %q to %s:%d: exited with code %d", src, dstIPv4, sshProbePort, rc)
	}

	return output, nil
}
