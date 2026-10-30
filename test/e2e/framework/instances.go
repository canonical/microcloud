package framework

import (
	"bytes"
	"fmt"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
)

// connectionProject returns the project Server() is scoped to, for use in
// error messages. It falls back to "" if the connection info is unavailable.
func connectionProject() string {
	info, err := Server().GetConnectionInfo()
	if err != nil {
		return ""
	}

	return info.Project
}

// GetInstanceNames returns the names of all instances in the project Server()
// is scoped to.
func GetInstanceNames() ([]string, error) {
	names, err := Server().GetInstanceNames(api.InstanceTypeAny)
	if err != nil {
		return nil, fmt.Errorf("Cannot list instances in project=%s: %w", connectionProject(), err)
	}

	return names, nil
}

// GetInstanceNamesByMember returns the names of instances located on member,
// in the project Server() is scoped to.
func GetInstanceNamesByMember(member string) ([]string, error) {
	instances, err := Server().GetInstances(lxd.GetInstancesArgs{})
	if err != nil {
		return nil, fmt.Errorf("Cannot list instances in member=%s project=%s: %w", member, connectionProject(), err)
	}

	var names []string
	for _, i := range instances {
		if i.Location != member {
			continue
		}

		names = append(names, i.Name)
	}

	return names, nil
}

// GetInstanceIPv4 returns the first global-scope IPv4 address reported for the
// instance, or "" if it has none yet.
func GetInstanceIPv4(name string) (string, error) {
	state, _, err := Server().GetInstanceState(name)
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

// EnsureInstanceRunning starts the instance if it's currently stopped.
func EnsureInstanceRunning(name string) error {
	state, _, err := Server().GetInstanceState(name)
	if err != nil {
		return fmt.Errorf("Cannot get state of instance %q: %w", name, err)
	}

	if state.Status == api.Running.String() {
		return nil
	}

	op, err := Server().UpdateInstanceState(name, api.InstanceStatePut{Action: "start"}, "")
	if err != nil {
		return fmt.Errorf("Cannot start instance %q: %w", name, err)
	}

	err = op.Wait()
	if err != nil {
		return fmt.Errorf("Cannot start instance %q: %w", name, err)
	}

	return nil
}

// ExecInInstance runs argv inside the named instance on Server() and waits for
// it to complete. A non-zero exit status is not an error: it is returned as
// exitCode along with the captured output. err is only set when the command
// could not be started or its result could not be determined.
func ExecInInstance(instance string, argv []string, env map[string]string) (stdout string, stderr string, exitCode int, err error) {
	return execInInstanceOn(Server(), instance, argv, env)
}

// execInInstanceOn is like ExecInInstance but against an explicit
// lxd.InstanceServer, so callers (e.g. LXDController) can target a connection
// other than Server() (which is scoped to REMOTE).
func execInInstanceOn(s lxd.InstanceServer, instance string, argv []string, env map[string]string) (stdout string, stderr string, exitCode int, err error) {
	var outBuf, errBuf bytes.Buffer

	op, err := s.ExecInstance(instance, api.InstanceExecPost{
		Command:     argv,
		WaitForWS:   true,
		Environment: env,
	}, &lxd.InstanceExecArgs{Stdout: &outBuf, Stderr: &errBuf})
	if err != nil {
		return "", "", -1, fmt.Errorf("Cannot exec into instance %q: %w", instance, err)
	}

	err = op.Wait()
	if err != nil {
		return outBuf.String(), errBuf.String(), -1, fmt.Errorf("Exec in instance %q failed: %w (output: %s)", instance, err, outBuf.String()+errBuf.String())
	}

	rc, ok := op.Get().Metadata["return"].(float64)
	if !ok {
		return outBuf.String(), errBuf.String(), -1, fmt.Errorf("Exec in instance %q returned no exit code", instance)
	}

	return outBuf.String(), errBuf.String(), int(rc), nil
}
