package rde

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// VNCCredentials is the credential bundle a session exposes for VNC. The
// JSON tags define the stable shape used by `rde session vnc --output json`.
// The fields mirror what the backend returns (address, username, password)
// plus a pre-built `vnc://` URL ready to hand to an OS handler.
type VNCCredentials struct {
	Address  string `json:"address"`
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
