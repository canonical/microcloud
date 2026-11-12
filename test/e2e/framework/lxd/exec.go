package lxd

import (
	"bytes"
	"fmt"

	lxdclient "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
)

// ExecInInstanceOn runs argv inside the named instance on the given server and waits for it to
// complete. A non-zero exit status is not an error: it is returned as exitCode along with the
// captured output. err is only set when the command could not be started or its result could not
// be determined.
func ExecInInstanceOn(s lxdclient.InstanceServer, instance string, argv []string, env map[string]string) (stdout string, stderr string, exitCode int, err error) {
	var outBuf, errBuf bytes.Buffer

	op, err := s.ExecInstance(instance, api.InstanceExecPost{
		Command:     argv,
		WaitForWS:   true,
		Environment: env,
	}, &lxdclient.InstanceExecArgs{Stdout: &outBuf, Stderr: &errBuf})
	if err != nil {
		return "", "", -1, fmt.Errorf("Cannot exec into instance %q: %w", instance, err)
	}

	err = op.Wait()
	if err != nil {
		return outBuf.String(), errBuf.String(), -1, fmt.Errorf("Cannot exec in instance %q: %w (output: %s)", instance, err, outBuf.String()+errBuf.String())
	}

	rc, ok := op.Get().Metadata["return"].(float64)
	if !ok {
		return outBuf.String(), errBuf.String(), -1, fmt.Errorf("Cannot determine exit code for instance %q", instance)
	}

	return outBuf.String(), errBuf.String(), int(rc), nil
}
