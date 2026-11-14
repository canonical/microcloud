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

const (
	// sshDialTimeout bounds how long a single SSH connection attempt may take.
	sshDialTimeout = 10 * time.Second

	// commandTimeout bounds how long a single remote command may block before the session is
	// forcibly closed and a timeout error is returned.
	commandTimeout = 3 * time.Minute

	// unreachableGracePeriod bounds how long Reboot waits to observe the member actually become
	// unreachable after "sudo reboot" is issued.
	unreachableGracePeriod = 30 * time.Second
)

// shellQuote single-quotes s for use in a POSIX shell command line, escaping any embedded single
// quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
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

// runWithTimeout runs cmd on session, forcibly closing the session (which unblocks session.Run)
// and returning a timeout error if it hasn't completed within timeout, bounding what would
// otherwise be an indefinite wait on a wedged remote command.
func runWithTimeout(session *ssh.Session, cmd string, timeout time.Duration) error {
	done := make(chan error, 1)

	go func() {
		done <- session.Run(cmd)
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		_ = session.Close()
		runErr := <-done // wait for the goroutine above to actually return, so it doesn't leak

		return fmt.Errorf("Cannot run command: timed out after %s (session.Run returned: %v)", timeout, runErr)
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

// IsSudoPermissionDenied reports whether output (e.g. a command's combined stdout+stderr) looks
// like sudo rejected a command for lacking privileges -- a password is required and no
// TTY/askpass is available, the user isn't in the sudoers file, an incorrect password was
// supplied, etc.
func IsSudoPermissionDenied(output string) bool {
	lower := strings.ToLower(output)

	return strings.Contains(lower, "password is required") ||
		strings.Contains(lower, "not in the sudoers file") ||
		strings.Contains(lower, "permission denied") ||
		strings.Contains(lower, "incorrect password") ||
		strings.Contains(lower, "no tty present and no askpass program specified")
}

// isPermissionDenied reports whether err/stderr indicate "sudo reboot" was rejected for lacking
// privileges (e.g. the testbed user isn't in sudoers, a password is required, or sudo refused to
// prompt for one since Exec/Reboot run over a plain SSH exec channel with no PTY/stdin attached)
// rather than the connection being torn down by an actual reboot. Only an error carrying an actual
// exit status (*ssh.ExitError) is considered: a disconnect is never a permission issue -- if sudo
// had really rejected the command, the session would exit with a status rather than losing the
// connection.
func isPermissionDenied(err error, stderr string) bool {
	if err == nil {
		return false
	}

	var exitErr *ssh.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}

	return IsSudoPermissionDenied(stderr)
}

// SSHController controls cluster members reached over SSH, using the connection details configured
// in a testbed file.
type SSHController struct {
	nodes map[string]testbed.SSHNodeConfig
}

// newSSHController returns an SSHController for the given per-member SSH connection details,
// failing fast if any configured node cannot actually be dialed.
func newSSHController(nodes map[string]testbed.SSHNodeConfig) (*SSHController, error) {
	c := &SSHController{nodes: nodes}

	for member := range nodes {
		client, err := c.dial(member)
		if err != nil {
			return nil, fmt.Errorf("Cannot control cluster members via ssh: %w", err)
		}

		client.Close()
	}

	return c, nil
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

	err = runWithTimeout(session, quoteCommand(argv, env), commandTimeout)

	var exitErr *ssh.ExitError

	switch {
	case err == nil:
		return outBuf.String(), errBuf.String(), 0, nil
	case errors.As(err, &exitErr):
		return outBuf.String(), errBuf.String(), exitErr.ExitStatus(), nil
	default:
		return outBuf.String(), errBuf.String(), -1, fmt.Errorf("Cannot execute command on %q: %w", member, err)
	}
}

// Reboot restarts member over SSH and waits for it to be ready.
func (c *SSHController) Reboot(member string) error {
	client, err := c.dial(member)
	if err != nil {
		return fmt.Errorf("Cannot reboot %q: %w", member, err)
	}

	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("Cannot reboot %q: cannot open SSH session: %w", member, err)
	}

	defer session.Close()

	var stderr bytes.Buffer
	session.Stderr = &stderr

	runErr := runWithTimeout(session, "sudo reboot", commandTimeout)

	if isPermissionDenied(runErr, stderr.String()) {
		return fmt.Errorf("Cannot reboot %q: %w (stderr=%q)", member, ErrSudoPermissionDenied, stderr.String())
	}

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

	// Wait for the member to come back up, bounded by a timeout.
	return waitReady(c, member, deadline, true)
}

// GetMembers returns the cluster member names configured under the testbed file's ssh.nodes.
func (c *SSHController) GetMembers() ([]string, error) {
	members := make([]string, 0, len(c.nodes))
	for member := range c.nodes {
		members = append(members, member)
	}

	return members, nil
}

// dial connects to the named member.
func (c *SSHController) dial(member string) (*ssh.Client, error) {
	node, ok := c.nodes[member]
	if !ok {
		return nil, fmt.Errorf("Cannot connect to cluster member %q: no SSH connection details configured", member)
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
