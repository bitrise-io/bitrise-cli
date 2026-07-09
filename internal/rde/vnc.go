package rde

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// VNCCredentials is the credential bundle a session exposes for VNC. The
// JSON tags define the stable shape used by `rde session vnc --output json`.
// The fields mirror what the backend returns (address, username, password)
// plus a pre-built `vnc://` URL ready to hand to an OS handler.
//
// Host and Port are the address decomposed into discrete fields, so callers
// that need to build their own connection (a bridge, a native client) never
// have to parse `address` or the URL — the endpoint is always fully qualified.
type VNCCredentials struct {
	Address  string `json:"address"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	URL      string `json:"url"`
}

// GetSessionVNC fetches the session and returns its VNC credentials,
// erroring clearly when the session has no VNC endpoint yet (still
// provisioning, terminated, or a Linux template that doesn't expose VNC).
func (s *Service) GetSessionVNC(ctx context.Context, workspaceID, sessionID string) (VNCCredentials, error) {
	if s.client == nil {
		return VNCCredentials{}, errClient()
	}
	sess, err := s.GetSession(ctx, workspaceID, sessionID)
	if err != nil {
		return VNCCredentials{}, err
	}
	return VNCCredentialsFromSession(sess)
}

// SessionExposesVNC reports whether the session currently has a VNC endpoint.
// VNC is exposed by macOS sessions once they are running; Linux sessions have
// none. Callers use it to decide whether to offer VNC-related features for a
// session at all, rather than letting GetSessionVNC fail later.
func (s *Service) SessionExposesVNC(ctx context.Context, workspaceID, sessionID string) (bool, error) {
	if s.client == nil {
		return false, errClient()
	}
	sess, err := s.GetSession(ctx, workspaceID, sessionID)
	if err != nil {
		return false, err
	}
	return sess.VNCAddress != "", nil
}

// VNCCredentialsFromSession assembles a credentials bundle from an already
// loaded Session. Split from GetSessionVNC so callers that already hold a
// Session (e.g. `session create --wait`) can reuse it without a second GET.
func VNCCredentialsFromSession(sess Session) (VNCCredentials, error) {
	if sess.VNCAddress == "" {
		if sess.Status != "running" && sess.Status != "" {
			return VNCCredentials{}, fmt.Errorf(
				"session VNC is not available (status: %q); VNC is exposed once the session is running",
				sess.Status,
			)
		}
		return VNCCredentials{}, fmt.Errorf(
			"session has no VNC endpoint (the template may not expose VNC, or the session is still provisioning)",
		)
	}
	host, port, err := parseVNCHostPort(sess.VNCAddress)
	if err != nil {
		return VNCCredentials{}, err
	}
	creds := VNCCredentials{
		Address:  sess.VNCAddress,
		Host:     host,
		Port:     port,
		Username: sess.VNCUsername,
		Password: sess.VNCPassword,
	}
	creds.URL = buildVNCURL(host, port, sess.VNCUsername, sess.VNCPassword)
	return creds, nil
}

// parseVNCHostPort accepts the address shapes the backend has used:
// "vnc://host:port", "host:port", or bare "host". A missing port defaults
// to 5900 (standard VNC).
func parseVNCHostPort(addr string) (string, int, error) {
	s := strings.TrimSpace(addr)
	s = strings.TrimPrefix(s, "vnc://")
	if s == "" {
		return "", 0, fmt.Errorf("empty VNC address")
	}
	host := s
	port := 5900
	if idx := strings.LastIndex(s, ":"); idx > 0 {
		host = s[:idx]
		p, err := parsePort(s[idx+1:])
		if err != nil {
			return "", 0, fmt.Errorf("VNC address %q: %w", addr, err)
		}
		port = p
	}
	if host == "" {
		return "", 0, fmt.Errorf("VNC address %q has no host", addr)
	}
	return host, port, nil
}

func parsePort(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty port")
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid port %q", s)
		}
		n = n*10 + int(c-'0')
		if n > 65535 {
			return 0, fmt.Errorf("port %q out of range", s)
		}
	}
	return n, nil
}

// FormatVNCURL builds a `vnc://[user[:pass]@]host:port` URL with URL-escaped
// credentials — the same shape as VNCCredentials.URL. Exposed so a caller that
// forwards the endpoint to a local port (see ForwardVNC) can present a
// ready-to-use URL pointing at the local address.
func FormatVNCURL(host string, port int, user, pass string) string {
	return buildVNCURL(host, port, user, pass)
}

// ForwardVNC opens an SSH tunnel to the session and forwards its VNC endpoint
// to a local TCP port, blocking until ctx is cancelled. localPort 0 auto-picks
// a free port; the chosen "127.0.0.1:port" is reported via onReady once the
// listener is accepting, so a caller can print connection details. A native
// VNC client (e.g. macOS Screen Sharing) then connects to that local address —
// no credentials embedded in a URL and no direct network route to the session
// required, since the traffic rides the SSH connection the CLI already trusts.
//
// The tunnel targets the session VM's loopback VNC port — the standard
// `ssh -L LOCAL:localhost:5900` recipe. macOS Screen Sharing listens locally on
// the VM and the SSH connection terminates there, so dialing the VM's
// 127.0.0.1:<vnc-port> reaches it regardless of how the endpoint is exposed
// externally. (The port is taken from the backend's vnc_address, defaulting to
// the standard 5900.)
func (s *Service) ForwardVNC(ctx context.Context, workspaceID, sessionID string, localPort int, onReady func(localAddr string)) error {
	if s.client == nil {
		return errClient()
	}
	sess, err := s.GetSession(ctx, workspaceID, sessionID)
	if err != nil {
		return err
	}
	if sess.VNCAddress == "" {
		return fmt.Errorf("session has no VNC endpoint (the template may not expose VNC, or the session is still provisioning)")
	}
	_, vncPort, err := parseVNCHostPort(sess.VNCAddress)
	if err != nil {
		return err
	}
	target, err := sshTargetForSession(sess)
	if err != nil {
		return err
	}
	client, err := dialSSHWithRetry(ctx, target)
	if err != nil {
		return err
	}
	defer client.Close() //nolint:errcheck // forward errors take precedence; nothing actionable on close failure

	remoteAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(vncPort))
	return client.forwardLocal(ctx, localPort, remoteAddr, onReady)
}

// buildVNCURL emits a `vnc://[user[:pass]@]host:port` URL. URL-escapes
// credentials so a `@` or `:` in the password doesn't desync the parser
// on the receiving side.
func buildVNCURL(host string, port int, user, pass string) string {
	var b strings.Builder
	b.WriteString("vnc://")
	if user != "" || pass != "" {
		if user != "" {
			b.WriteString(url.QueryEscape(user))
		}
		if pass != "" {
			b.WriteByte(':')
			b.WriteString(url.QueryEscape(pass))
		}
		b.WriteByte('@')
	}
	fmt.Fprintf(&b, "%s:%d", host, port)
	return b.String()
}
