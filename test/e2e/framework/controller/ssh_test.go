package controller

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/canonical/microcloud/microcloud/test/e2e/framework/testbed"
)

// generateTestKeyPair returns a freshly generated ed25519 key pair, with the private key encoded
// as an OpenSSH-format PEM block.
func generateTestKeyPair(t *testing.T) (ssh.Signer, []byte) {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Cannot generate test key pair: %v", err)
	}

	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("Cannot marshal test private key: %v", err)
	}

	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("Cannot build signer from test key: %v", err)
	}

	return signer, pem.EncodeToMemory(block)
}

// testSSHServer is a minimal in-process SSH server accepting a single authorized public key and
// running "exec" requests via the local shell, used to exercise SSHController without any real
// network/hardware dependency. It can also simulate a reboot: when it sees the exact command "sudo
// reboot" (as issued by SSHController.Reboot), it stays reachable for rebootStaleWindow (mirroring
// how real sshd keeps serving the still-running pre-reboot system for a moment), then refuses new
// connections for rebootDownWindow (mirroring real downtime), then resumes normal service. Not
// currently exercised by a Reboot-level test: Reboot's own tail end (waitReady) shells out to real
// sudo/systemctl/microcloud on the test-running host, which isn't safe to depend on from a
// portable unit test; rebootStaleWindow/rebootDownWindow remain available for a narrower,
// dial-level test of the reconnect timing if one is added later.
type testSSHServer struct {
	listener net.Listener
	addr     string

	mu                sync.Mutex
	up                bool
	rebootStaleWindow time.Duration
	rebootDownWindow  time.Duration
}

// startTestSSHServer starts a server on 127.0.0.1:0 (a random free port), authorizing only
// connections presenting authorizedKey, and stops it on test cleanup.
func startTestSSHServer(t *testing.T, authorizedKey ssh.PublicKey, hostSigner ssh.Signer) *testSSHServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Cannot listen: %v", err)
	}

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if !bytes.Equal(key.Marshal(), authorizedKey.Marshal()) {
				return nil, errUnauthorized
			}

			return nil, nil
		},
	}

	config.AddHostKey(hostSigner)

	srv := &testSSHServer{listener: listener, addr: listener.Addr().String(), up: true}

	go srv.serve(t, config)

	t.Cleanup(func() { _ = listener.Close() })

	return srv
}

var errUnauthorized = errors.New("unauthorized test client key")

func (s *testSSHServer) isUp() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.up
}

func (s *testSSHServer) setUp(up bool) {
	s.mu.Lock()
	s.up = up
	s.mu.Unlock()
}

// simulateReboot schedules the up -> stale -> down -> up transition
// described in testSSHServer's doc comment, starting immediately (called
// right when a "sudo reboot" exec is observed).
func (s *testSSHServer) simulateReboot() {
	go func() {
		time.Sleep(s.rebootStaleWindow)
		s.setUp(false)
		time.Sleep(s.rebootDownWindow)
		s.setUp(true)
	}()
}

func (s *testSSHServer) serve(t *testing.T, config *ssh.ServerConfig) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		if !s.isUp() {
			_ = conn.Close()
			continue
		}

		go s.handleConn(t, conn, config)
	}
}

func (s *testSSHServer) handleConn(t *testing.T, conn net.Conn, config *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}

	defer sshConn.Close()

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported channel type")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}

		go handleTestSessionRequests(s, channel, requests)
	}
}

// handleTestSessionRequests handles "exec" requests on an accepted session channel by running the
// command via the local shell and reporting its exit status, mirroring (a minimal subset of) real
// sshd behavior. A literal "sudo reboot" command triggers srv.simulateReboot instead of actually
// running anything, then closes the channel without reporting an exit status (mirroring a real
// reboot dropping the connection).
func handleTestSessionRequests(srv *testSSHServer, channel ssh.Channel, reqs <-chan *ssh.Request) {
	defer channel.Close()

	for req := range reqs {
		if req.Type != "exec" {
			if req.WantReply {
				_ = req.Reply(false, nil)
			}

			continue
		}

		var payload struct{ Command string }

		err := ssh.Unmarshal(req.Payload, &payload)
		if err != nil {
			_ = req.Reply(false, nil)
			continue
		}

		_ = req.Reply(true, nil)

		if payload.Command == "sudo reboot" {
			srv.simulateReboot()
			return
		}

		cmd := exec.Command("sh", "-c", payload.Command)
		cmd.Stdout = channel
		cmd.Stderr = channel.Stderr()

		runErr := cmd.Run()

		exitCode := 0

		if runErr != nil {
			exitCode = 255

			exitErr, ok := runErr.(*exec.ExitError)
			if ok {
				exitCode = exitErr.ExitCode()
			}
		}

		_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{uint32(exitCode)}))

		return
	}
}

// writeKeyFile writes keyPEM to a file under dir and returns its path.
func writeKeyFile(t *testing.T, dir string, keyPEM []byte) string {
	t.Helper()

	path := filepath.Join(dir, "id_test")

	err := os.WriteFile(path, keyPEM, 0o600)
	if err != nil {
		t.Fatalf("Cannot write key file: %v", err)
	}

	return path
}

func newTestSSHController(t *testing.T, member string) *SSHController {
	t.Helper()

	c, _ := newTestSSHControllerAndServer(t, member)

	return c
}

// newTestSSHControllerAndServer is like newTestSSHController but also returns the underlying
// testSSHServer, for tests that need to drive its simulated-reboot behavior.
func newTestSSHControllerAndServer(t *testing.T, member string) (*SSHController, *testSSHServer) {
	t.Helper()

	clientSigner, clientKeyPEM := generateTestKeyPair(t)
	hostSigner, _ := generateTestKeyPair(t)

	srv := startTestSSHServer(t, clientSigner.PublicKey(), hostSigner)

	host, port, err := net.SplitHostPort(srv.addr)
	if err != nil {
		t.Fatalf("Cannot split address %q: %v", srv.addr, err)
	}

	dir := t.TempDir()
	keyPath := writeKeyFile(t, dir, clientKeyPEM)

	portNum, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("Cannot parse port %q: %v", port, err)
	}

	c, err := newSSHController(map[string]testbed.SSHNodeConfig{
		member: {
			Address:        host,
			Port:           portNum,
			User:           "test",
			PrivateKeyFile: keyPath,
		},
	})
	if err != nil {
		t.Fatalf("Cannot create SSHController: %v", err)
	}

	return c, srv
}

func TestNewSSHControllerSuccess(t *testing.T) {
	c := newTestSSHController(t, "micro1")

	members, err := c.GetMembers()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(members) != 1 || members[0] != "micro1" {
		t.Fatalf("expected [micro1], got %v", members)
	}
}

func TestNewSSHControllerUnreachable(t *testing.T) {
	nodes := map[string]testbed.SSHNodeConfig{
		"micro1": {Address: "10.0.0.11", User: "ubuntu", PrivateKeyFile: "/nonexistent"},
	}

	_, err := newSSHController(nodes)
	if err == nil {
		t.Fatal("expected newSSHController to fail for a bad private key file")
	}
}

func TestSSHControllerExecSuccess(t *testing.T) {
	c := newTestSSHController(t, "micro1")

	stdout, stderr, rc, err := c.Exec("micro1", []string{"echo", "hello world"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rc != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", rc, stderr)
	}

	if stdout != "hello world\n" {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
}

func TestSSHControllerExecNonZeroExit(t *testing.T) {
	c := newTestSSHController(t, "micro1")

	_, _, rc, err := c.Exec("micro1", []string{"sh", "-c", "exit 7"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rc != 7 {
		t.Fatalf("expected exit code 7, got %d", rc)
	}
}

func TestSSHControllerExecWithEnv(t *testing.T) {
	c := newTestSSHController(t, "micro1")

	stdout, _, rc, err := c.Exec("micro1", []string{"sh", "-c", "echo $FOO"}, map[string]string{"FOO": "bar baz"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rc != 0 {
		t.Fatalf("expected exit code 0, got %d", rc)
	}

	if stdout != "bar baz\n" {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
}

func TestSSHControllerExecUnknownMember(t *testing.T) {
	c := newTestSSHController(t, "micro1")

	_, _, _, err := c.Exec("micro-does-not-exist", []string{"true"}, nil)
	if err == nil {
		t.Fatal("expected an error for an unconfigured member")
	}
}

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"hello":       "'hello'",
		"hello world": "'hello world'",
		"it's":        `'it'\''s'`,
	}

	for in, want := range cases {
		got := shellQuote(in)
		if got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsExpectedRebootDisconnect(t *testing.T) {
	if !isExpectedRebootDisconnect(nil) {
		t.Error("expected nil to be treated as an expected disconnect")
	}

	if !isExpectedRebootDisconnect(&ssh.ExitMissingError{}) {
		t.Error("expected ExitMissingError to be treated as an expected disconnect")
	}
}

func TestIsPermissionDenied(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		stderr string
		want   bool
	}{
		{name: "nil error", err: nil, stderr: "", want: false},
		{name: "disconnect (not an exit error) is never a permission issue", err: io.EOF, stderr: "permission denied", want: false},
		{name: "exit error with unrelated stderr", err: &ssh.ExitError{}, stderr: "some other failure", want: false},
		{name: "exit error, password required", err: &ssh.ExitError{}, stderr: "sudo: a password is required", want: true},
		{name: "exit error, not in sudoers", err: &ssh.ExitError{}, stderr: "test is not in the sudoers file.  This incident will be reported.", want: true},
		{name: "exit error, permission denied (any case)", err: &ssh.ExitError{}, stderr: "Permission Denied", want: true},
		{name: "exit error, incorrect password", err: &ssh.ExitError{}, stderr: "Sorry, try again.\nsudo: 1 incorrect password attempt", want: true},
		{name: "exit error, no tty/askpass", err: &ssh.ExitError{}, stderr: "sudo: no tty present and no askpass program specified", want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isPermissionDenied(tc.err, tc.stderr)
			if got != tc.want {
				t.Errorf("isPermissionDenied(%v, %q) = %v, want %v", tc.err, tc.stderr, got, tc.want)
			}
		})
	}
}

func TestIsSudoPermissionDenied(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   bool
	}{
		{name: "empty", output: "", want: false},
		{name: "unrelated failure", output: "some other failure", want: false},
		{name: "password required", output: "sudo: a password is required", want: true},
		{name: "not in sudoers", output: "test is not in the sudoers file.  This incident will be reported.", want: true},
		{name: "permission denied (any case)", output: "Permission Denied", want: true},
		{name: "incorrect password", output: "Sorry, try again.\nsudo: 1 incorrect password attempt", want: true},
		{name: "no tty/askpass", output: "sudo: no tty present and no askpass program specified", want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsSudoPermissionDenied(tc.output)
			if got != tc.want {
				t.Errorf("IsSudoPermissionDenied(%q) = %v, want %v", tc.output, got, tc.want)
			}
		})
	}
}

// TestRunWithTimeout confirms a wedged remote command is bounded by timeout rather than blocking
// indefinitely: "sleep 5" run with a 200ms timeout must return promptly with an error, not hang.
func TestRunWithTimeout(t *testing.T) {
	c := newTestSSHController(t, "micro1")

	client, err := c.dial("micro1")
	if err != nil {
		t.Fatalf("Cannot dial: %v", err)
	}

	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("Cannot open session: %v", err)
	}

	defer session.Close()

	start := time.Now()
	err = runWithTimeout(session, "sleep 5", 200*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}

	if elapsed > 2*time.Second {
		t.Fatalf("runWithTimeout did not return promptly after its timeout: took %s", elapsed)
	}
}
