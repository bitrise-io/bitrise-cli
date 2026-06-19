package rde

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/term"
)

// This file ports the MCP server's execute_ssh.go — same forced-interactive
// login bash semantics, same agent forwarding posture, same auth-method
// fallback chain. Kept self-contained so the rest of the service layer
// doesn't pick up an SSH dependency.

type sshTarget struct {
	Host     string
	Port     int
	User     string
	Password string
}

// ExecResult is the captured result of a remote command execution. Field
// names match the JSON contract emitted by `rde session exec --output json`.
type ExecResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// sshClient wraps an ssh.Client with a run method that executes commands in
// a forced-interactive login bash shell over a fresh session. If a local
// SSH agent was available at dial time, each session gets agent forwarding
// so the remote command (e.g. `git push git@github.com:...`) can
// authenticate with the caller's local SSH keys.
//
// Agent-forwarding threat model: a forwarded agent lets the session VM
// impersonate the caller on any host that trusts the caller's keys for as
// long as the session is open. Acceptable here because session VMs are
// single-tenant, provisioned for and owned by the caller — the VM is a
// peer the user already trusts with their development environment. Do not
// copy this pattern into any context where the session VM is shared or
// third-party-controlled.
type sshClient struct {
	client      *ssh.Client
	localAgent  agent.ExtendedAgent
	agentSocket io.Closer
}

const sshHandshakeTimeout = 15 * time.Second

const (
	// sshDialReadyTimeout bounds how long dialSSHWithRetry keeps retrying a
	// connection that's refused/unreachable — the SSH port may not accept
	// connections for a few seconds after the session reports SSH-ready.
	sshDialReadyTimeout  = 2 * time.Minute
	sshDialRetryInterval = 2 * time.Second
)

// dialSSHWithRetry dials t, retrying on transient connection failures
// (connection refused, reset, unreachable, timeout, early EOF) until
// sshDialReadyTimeout elapses. Non-transient failures — notably authentication
// — return immediately so a real misconfiguration fails fast.
func dialSSHWithRetry(ctx context.Context, t sshTarget) (*sshClient, error) {
	dialCtx, cancel := context.WithTimeout(ctx, sshDialReadyTimeout)
	defer cancel()

	var lastErr error
	for {
		client, err := dialSSH(dialCtx, t)
		if err == nil {
			return client, nil
		}
		if !isRetryableDialErr(err) {
			return nil, err
		}
		lastErr = err
		select {
		case <-dialCtx.Done():
			return nil, fmt.Errorf("ssh not reachable after %s: %w", sshDialReadyTimeout, lastErr)
		case <-time.After(sshDialRetryInterval):
		}
	}
}

// isRetryableDialErr reports whether err is a transient connection-level
// failure worth retrying. Network errors (connection refused/reset, host
// unreachable, timeouts — all surfaced as *net.OpError / net.Error) and an
// early io.EOF qualify; SSH auth/handshake errors do not.
func isRetryableDialErr(err error) bool {
	if err == nil {
		return false
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	return errors.Is(err, io.EOF)
}

func dialSSH(ctx context.Context, t sshTarget) (*sshClient, error) {
	if t.Host == "" {
		return nil, fmt.Errorf("ssh target: host is empty")
	}
	if t.User == "" {
		return nil, fmt.Errorf("ssh target: user is empty")
	}
	if t.Port == 0 {
		t.Port = 22
	}

	localAgent, agentSocket := dialLocalAgent()

	methods := sshAuthMethods(t.Password, localAgent)
	if len(methods) == 0 {
		if agentSocket != nil {
			_ = agentSocket.Close()
		}
		return nil, fmt.Errorf("ssh target: no auth methods available (no agent, no default keys, no password)")
	}

	cfg := &ssh.ClientConfig{
		User:            t.User,
		Auth:            methods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // sessions are ephemeral; matches UI terminal behavior
		Timeout:         sshHandshakeTimeout,
	}

	addr := net.JoinHostPort(t.Host, strconv.Itoa(t.Port))
	d := &net.Dialer{Timeout: sshHandshakeTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		if agentSocket != nil {
			_ = agentSocket.Close()
		}
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	handshakeDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-handshakeDone:
		}
	}()
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	close(handshakeDone)
	if err != nil {
		if agentSocket != nil {
			_ = agentSocket.Close()
		}
		return nil, fmt.Errorf("ssh handshake %s: %w", addr, err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)

	if localAgent != nil {
		if err := agent.ForwardToAgent(client, localAgent); err != nil {
			_ = client.Close()
			if agentSocket != nil {
				_ = agentSocket.Close()
			}
			return nil, fmt.Errorf("ssh install agent forwarding: %w", err)
		}
	}

	return &sshClient{
		client:      client,
		localAgent:  localAgent,
		agentSocket: agentSocket,
	}, nil
}

func (c *sshClient) Close() error {
	if c == nil {
		return nil
	}
	var errs []error
	if c.client != nil {
		if err := c.client.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.agentSocket != nil {
		if err := c.agentSocket.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// listenRemote asks the remote sshd to open a listener on addr and returns a
// net.Listener whose Accept yields connections initiated on the remote side and
// tunneled back over this SSH connection (the equivalent of `ssh -R`). Pass an
// explicit loopback bind address (e.g. "127.0.0.1:0") so the remote listener is
// reachable only from the session itself, never the session's network.
func (c *sshClient) listenRemote(addr string) (net.Listener, error) {
	return c.client.Listen("tcp", addr)
}

// run executes userCmd in a forced-interactive login bash shell on a fresh
// SSH session, without allocating a PTY. stdout and stderr are captured
// separately. Context cancellation propagates by closing the session.
//
// `-i -l` is required because RDE warmup scripts commonly write PATH to
// ~/.bashrc, and .bashrc short-circuits on its `case $- in *i*)` guard
// when the shell is non-interactive.
//
// ExitCode is 0 on success, the command's exit status on a clean non-zero
// exit, or -1 if the session was terminated by a signal or cancellation.
func (c *sshClient) run(ctx context.Context, userCmd string) (ExecResult, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return ExecResult{}, fmt.Errorf("ssh new session: %w", err)
	}
	defer session.Close() //nolint:errcheck // run errors take precedence; nothing actionable on close failure

	if c.localAgent != nil {
		// Best-effort. If remote sshd refuses (AllowAgentForwarding=no), the
		// user's command runs without a forwarded agent — any git-over-SSH
		// step then fails with an auth error, surfaced through stderr.
		_ = agent.RequestAgentForwarding(session)
	}

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = session.Close()
		case <-done:
		}
	}()
	defer close(done)

	runErr := session.Run(buildLoginShellCmd(userCmd))

	result := ExecResult{
		Stdout: stdout.String(),
		Stderr: stripInteractiveBashStartupNoise(stderr.String()),
	}

	if runErr == nil {
		return result, nil
	}

	if ctx.Err() != nil {
		result.ExitCode = -1
		return result, fmt.Errorf("ssh run cancelled: %w", ctx.Err())
	}

	var exitErr *ssh.ExitError
	if errors.As(runErr, &exitErr) {
		result.ExitCode = exitErr.ExitStatus()
		return result, nil
	}

	result.ExitCode = -1
	return result, fmt.Errorf("ssh run: %w", runErr)
}

func buildLoginShellCmd(userCmd string) string {
	return "bash -i -l -c '" + strings.ReplaceAll(userCmd, "'", `'\''`) + "'"
}

// defaultTermType is used when $TERM is unset (e.g. some CI shells); a sane
// 256-color default keeps the remote program's rendering correct.
const defaultTermType = "xterm-256color"

// runInteractive runs userCmd in a forced-interactive login bash shell on a
// fresh SSH session, streaming the caller's stdin/stdout/stderr live and
// blocking until the remote program exits. When stdin is a terminal, it
// allocates a PTY, puts the local terminal into raw mode, and forwards
// SIGWINCH so the remote program reflows on resize.
//
// Unlike run, output is NOT captured and there is no timeout — this is meant
// for long-lived interactive programs. The `-i -l` shell is used for the same
// reason as run (RDE warmup writes PATH to ~/.bashrc); callers typically pass
// `exec <program>` so bash replaces itself with the program rather than
// leaving an interactive shell in front of it.
//
// Returns the remote exit code (0 on clean exit), or -1 with an error on
// dial/session failure or context cancellation.
func (c *sshClient) runInteractive(ctx context.Context, userCmd string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return -1, fmt.Errorf("ssh new session: %w", err)
	}
	defer session.Close() //nolint:errcheck // run errors take precedence; nothing actionable on close failure

	if c.localAgent != nil {
		// Best-effort, same posture as run: a refusing remote sshd just means
		// agent-backed auth inside the session is unavailable.
		_ = agent.RequestAgentForwarding(session)
	}

	session.Stdin = stdin
	session.Stdout = stdout
	session.Stderr = stderr

	// Allocate a PTY only when the local stdin is a real terminal. In a pipe
	// or CI context there's no TTY to mirror, so we run without one (the
	// remote program then sees a non-interactive stdin, which is correct).
	if fd, ok := ttyFd(stdin); ok {
		oldState, rawErr := term.MakeRaw(fd)
		if rawErr != nil {
			return -1, fmt.Errorf("set terminal raw mode: %w", rawErr)
		}
		defer func() { _ = term.Restore(fd, oldState) }()

		w, h, sizeErr := term.GetSize(fd)
		if sizeErr != nil {
			// Fall back to a conventional 80x24 if the size can't be read; the
			// SIGWINCH watcher will correct it on the first resize.
			w, h = 80, 24
		}

		termType := os.Getenv("TERM")
		if termType == "" {
			termType = defaultTermType
		}
		modes := ssh.TerminalModes{
			ssh.ECHO:          1,
			ssh.TTY_OP_ISPEED: 14400,
			ssh.TTY_OP_OSPEED: 14400,
		}
		if ptyErr := session.RequestPty(termType, h, w, modes); ptyErr != nil {
			return -1, fmt.Errorf("request pty: %w", ptyErr)
		}

		stop := watchWindowResize(session, fd)
		defer stop()
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = session.Close()
		case <-done:
		}
	}()
	defer close(done)

	// Detect a dropped connection quickly instead of waiting for the OS TCP
	// timeout (which can take minutes): keepAlive closes the client on the
	// first unanswered probe, which unblocks the Run below.
	go c.keepAlive(done)

	runErr := session.Run(buildLoginShellCmd(userCmd))
	if runErr == nil {
		return 0, nil
	}

	if ctx.Err() != nil {
		return -1, fmt.Errorf("ssh run cancelled: %w", ctx.Err())
	}

	var exitErr *ssh.ExitError
	if errors.As(runErr, &exitErr) {
		return exitErr.ExitStatus(), nil
	}

	// No exit status/signal (e.g. *ssh.ExitMissingError) or any other non-exit
	// failure means the channel dropped under us — the connection was lost.
	return -1, fmt.Errorf("%w: %v", ErrConnectionLost, runErr)
}

const (
	// sshKeepAliveInterval is how often a keepalive probe is sent;
	// sshKeepAliveTimeout is how long to wait for its reply before declaring
	// the connection dead. Together they bound drop detection to roughly
	// interval+timeout (≈5–10s).
	sshKeepAliveInterval = 5 * time.Second
	sshKeepAliveTimeout  = 5 * time.Second
)

// keepAlive sends periodic keepalive requests over the SSH transport. On the
// first probe that errors or goes unanswered within sshKeepAliveTimeout it
// assumes the connection is dead and closes the client, which unblocks an
// in-flight session.Run (surfaced as ErrConnectionLost). It stops when done is
// closed (normal end of the run).
func (c *sshClient) keepAlive(done <-chan struct{}) {
	ticker := time.NewTicker(sshKeepAliveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
		}

		// SendRequest blocks on the reply; the buffered channel lets the
		// goroutine exit even if we stop waiting on a timeout.
		reply := make(chan error, 1)
		go func() {
			_, _, err := c.client.SendRequest("keepalive@openssh.com", true, nil)
			reply <- err
		}()

		select {
		case <-done:
			return
		case err := <-reply:
			if err == nil {
				continue
			}
		case <-time.After(sshKeepAliveTimeout):
		}

		_ = c.client.Close() // unblocks session.Run; deferred Close is idempotent
		return
	}
}

// ttyFd returns the file descriptor of r when it is an *os.File backed by a
// terminal. Kept local to internal/rde so the SSH layer doesn't take a
// dependency on the cmd packages.
func ttyFd(r io.Reader) (int, bool) {
	f, ok := r.(*os.File)
	if !ok {
		return 0, false
	}
	fd := int(f.Fd()) //nolint:gosec // file descriptors are small ints, no overflow risk
	if !term.IsTerminal(fd) {
		return 0, false
	}
	return fd, true
}

// interactiveBashStartupNoise are the diagnostics `bash -i` writes to stderr
// when it has no controlling TTY — which is always the case here, since exec
// dials without allocating a PTY. They are emitted at shell startup, before
// the user's command runs, and carry no signal. We force interactive mode on
// purpose (see buildLoginShellCmd) so ~/.bashrc is sourced, and these lines
// are the unavoidable side effect. For a CLI a human reads, printing them on
// every invocation is pure noise, so we strip them from captured stderr.
var interactiveBashStartupNoise = map[string]bool{
	"bash: cannot set terminal process group (-1): Inappropriate ioctl for device": true,
	"bash: no job control in this shell":                                           true,
}

// stripInteractiveBashStartupNoise removes the leading run of bash interactive
// startup diagnostics from stderr, leaving everything after the user command's
// first real output byte-for-byte intact. Only leading lines are dropped: the
// noise is emitted before the command runs, and matching mid-stream would risk
// eating a legitimate identical line the command itself printed.
func stripInteractiveBashStartupNoise(stderr string) string {
	if stderr == "" {
		return ""
	}
	lines := strings.Split(stderr, "\n")
	i := 0
	for i < len(lines) && interactiveBashStartupNoise[lines[i]] {
		i++
	}
	if i == 0 {
		return stderr
	}
	return strings.Join(lines[i:], "\n")
}

// sshAuthMethods builds the auth-method chain. Password is tried FIRST:
// session sshd is password-authenticated by default. Trying publickey
// methods first would risk exhausting sshd's MaxAuthTries when the user's
// local agent holds many keys that aren't authorized on the session.
func sshAuthMethods(password string, a agent.ExtendedAgent) []ssh.AuthMethod {
	var methods []ssh.AuthMethod
	if password != "" {
		methods = append(methods, ssh.Password(password))
	}
	if a != nil {
		methods = append(methods, ssh.PublicKeysCallback(a.Signers))
	}
	if m := defaultKeyFilesAuthMethod(); m != nil {
		methods = append(methods, m)
	}
	return methods
}

func dialLocalAgent() (agent.ExtendedAgent, io.Closer) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, nil
	}
	conn, err := net.Dial("unix", sock) //nolint:gosec // sock is the path of the local SSH agent (SSH_AUTH_SOCK)
	if err != nil {
		return nil, nil
	}
	return agent.NewClient(conn), conn
}

func defaultKeyFilesAuthMethod() ssh.AuthMethod {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var signers []ssh.Signer
	for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
		data, err := os.ReadFile(filepath.Join(home, ".ssh", name)) //nolint:gosec // local SSH key files under $HOME
		if err != nil {
			continue
		}
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			// Skip encrypted or malformed keys silently — we don't prompt
			// for passphrases in CLI context.
			continue
		}
		signers = append(signers, signer)
	}
	if len(signers) == 0 {
		return nil
	}
	return ssh.PublicKeys(signers...)
}

var (
	userHostRegex     = regexp.MustCompile(`([\w.\-]+)@([\w.\-]+)`)
	sshAddrPortRegex  = regexp.MustCompile(`-p\s+(\d+)`)
	bareHostPortRegex = regexp.MustCompile(`^([\w.\-]+)(?::(\d+))?$`)
)

// sshTargetForSession runs the shared "reachable over SSH?" pre-flight and
// returns the dial target. Execute, ExecuteInteractive, and the host bridge all
// gate on the same conditions — session running and SSH credentials populated —
// so the checks live here once. The caller is responsible for the GetSession
// fetch (so it can decide how to classify that call's failures).
func sshTargetForSession(sess Session) (sshTarget, error) {
	if sess.Status != "running" {
		return sshTarget{}, fmt.Errorf(
			"session is not running (status: %q); start the session before running commands",
			sess.Status,
		)
	}
	if !sess.SSHConnectionOpen || sess.SSHAddress == "" || sess.SSHPassword == "" {
		return sshTarget{}, fmt.Errorf(
			"session SSH is not ready yet (credentials not populated); the session may still be provisioning — wait a few seconds and retry",
		)
	}
	target, err := parseSSHAddress(sess.SSHAddress)
	if err != nil {
		return sshTarget{}, fmt.Errorf("parse session ssh address: %w", err)
	}
	target.Password = sess.SSHPassword
	return target, nil
}

// parseSSHAddress extracts user, host, and port from a backend-provided
// ssh_address (which may be a full ssh command or "host:port"). Returns
// an error if no user is present — macOS sessions run as `vagrant` and
// Linux as `ubuntu`, so a silent fallback would misroute half the
// platforms.
func parseSSHAddress(addr string) (sshTarget, error) {
	if matches := userHostRegex.FindAllStringSubmatch(addr, -1); len(matches) > 0 {
		// Take the LAST user@host match. In OpenSSH command syntax the
		// target hostname comes after all options, so the rightmost match
		// is the intended target.
		m := matches[len(matches)-1]
		t := sshTarget{User: m[1], Host: m[2], Port: 22}
		if pm := sshAddrPortRegex.FindStringSubmatch(addr); pm != nil {
			p, err := strconv.Atoi(pm[1])
			if err != nil {
				return sshTarget{}, fmt.Errorf("ssh address %q: invalid port %q: %w", addr, pm[1], err)
			}
			t.Port = p
		}
		return t, nil
	}
	if bareHostPortRegex.MatchString(addr) {
		return sshTarget{}, fmt.Errorf("ssh address %q has no user component; cannot determine remote account", addr)
	}
	return sshTarget{}, fmt.Errorf("unable to parse ssh address: %q", addr)
}
