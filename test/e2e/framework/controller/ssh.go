package controller

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework/testbed"
)

// sshDialTimeout bounds how long a single SSH connection attempt may take.
const sshDialTimeout = 10 * time.Second

// unreachableGracePeriod bounds how long Reboot waits to observe the member actually become
// unreachable after "sudo reboot" is issued, before moving on to polling for it to come back up
// (see the comment in Reboot).
const unreachableGracePeriod = 30 * time.Second

// SSHController controls cluster members reached over SSH, using the connection details configured
// in a testbed file.
type SSHController struct {
	nodes map[string]testbed.SSHNodeConfig
}

// newSSHController returns an SSHController for the given per-member SSH connection details.
func newSSHController(nodes map[string]testbed.SSHNodeConfig) *SSHController {
	return &SSHController{nodes: nodes}
}

// clientConfig builds an *ssh.ClientConfig for node, loading and parsing its private key file.
func (c *SSHController) clientConfig(node testbed.SSHNodeConfig) (*ssh.ClientConfig, error) {
	keyData, err := os.ReadFile(node.PrivateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("Cannot read private_key_file %q: %w", node.PrivateKeyFile, err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("Cannot parse private_key_file %q: %w", node.PrivateKeyFile, err)
	}

	return &ssh.ClientConfig{
		User:            node.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // test tool, not production
		Timeout:         sshDialTimeout,
	}, nil
}

// dial connects to the named member.
func (c *SSHController) dial(member string) (*ssh.Client, error) {
	node, ok := c.nodes[member]
	if !ok {
		return nil, fmt.Errorf("No SSH connection details configured for cluster member %q", member)
	}

	config, err := c.clientConfig(node)
	if err != nil {
		return nil, fmt.Errorf("Cannot build SSH client config for %q: %w", member, err)
	}

	addr := net.JoinHostPort(node.Address, strconv.Itoa(node.Port))

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("Cannot connect to %q over SSH (%s): %w", member, addr, err)
	}

	return client, nil
}

// quoteCommand builds a single shell command line from argv and env, suitable for ssh.Session.Run,
// with every argument/value single-quoted (embedded single quotes escaped) since SSH's "exec"
// request takes one command string, not an argv array.
func quoteCommand(argv []string, env map[string]string) string {
	parts := make([]string, 0, len(env)+len(argv))

	for name, value := range env {
		parts = append(parts, fmt.Sprintf("%s=%s", name, shellQuote(value)))
	}

	for _, arg := range argv {
		parts = append(parts, shellQuote(arg))
	}

	return strings.Join(parts, " ")
}

// shellQuote single-quotes s for use in a POSIX shell command line, escaping any embedded single
// quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Exec runs argv on the named cluster member over a freshly dialed SSH connection.
func (c *SSHController) Exec(member string, argv []string, env map[string]string) (stdout string, stderr string, exitCode int, err error) {
	client, err := c.dial(member)
	if err != nil {
		return "", "", -1, err
	}

	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", "", -1, fmt.Errorf("Cannot open SSH session to %q: %w", member, err)
	}

	defer session.Close()

	var outBuf, errBuf bytes.Buffer

	session.Stdout = &outBuf
	session.Stderr = &errBuf

	err = session.Run(quoteCommand(argv, env))

	var exitErr *ssh.ExitError

	switch {
	case err == nil:
		return outBuf.String(), errBuf.String(), 0, nil
	case errors.As(err, &exitErr):
		return outBuf.String(), errBuf.String(), exitErr.ExitStatus(), nil
	default:
		return outBuf.String(), errBuf.String(), -1, fmt.Errorf("Exec on %q failed: %w", member, err)
	}
}

// isExpectedRebootDisconnect reports whether err is one of the errors expected when a command that
// kills the SSH session/connection (e.g. "sudo reboot") is run -- the server tears down the
// connection before it can report a clean exit status or any status at all.
func isExpectedRebootDisconnect(err error) bool {
	if err == nil {
		return true
	}

	var exitMissing *ssh.ExitMissingError
	if errors.As(err, &exitMissing) {
		return true
	}

	msg := err.Error()

	return strings.Contains(msg, "EOF") ||
		strings.Contains(msg, "closed") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe")
}

// Reboot restarts member over SSH and waits for it to be ready.
func (c *SSHController) Reboot(member string) error {
	client, err := c.dial(member)
	if err != nil {
		return fmt.Errorf("Cannot reboot %q: %w", member, err)
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return fmt.Errorf("Cannot reboot %q: cannot open SSH session: %w", member, err)
	}

	runErr := session.Run("sudo reboot")
	session.Close()
	client.Close()

	if !isExpectedRebootDisconnect(runErr) {
		return fmt.Errorf("Cannot reboot %q: %w", member, runErr)
	}

	deadline := time.Now().Add(DefaultRebootTimeout)

	// "sudo reboot" only asynchronously initiates the shutdown: sshd can stay responsive for a few
	// more seconds afterward, serving the still-running pre-reboot system. Without first
	// confirming the member actually became unreachable, the very next reconnect-poll could
	// succeed against that same stale session, falsely reporting the reboot as complete before the
	// system has actually gone down. Wait for that disconnection (bounded by a grace period, in
	// case the shutdown is fast enough that it's already happened) before polling for the member
	// to come back up.
	unreachableDeadline := time.Now().Add(unreachableGracePeriod)
	for time.Now().Before(unreachableDeadline) {
		probe, dialErr := c.dial(member)
		if dialErr != nil {
			break
		}

		probe.Close()
		time.Sleep(rebootPollInterval)
	}

	// SSHController connects as whatever user the testbed file configures, which is expected to
	// require sudo for these commands (see README.md).
	return waitReady(c, member, deadline, true)
}
