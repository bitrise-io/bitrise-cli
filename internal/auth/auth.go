// Package auth persists and reads the Bitrise access token.
//
// Storage: YAML at $XDG_CONFIG_HOME/bitrise/auth.yaml, falling back to
// ~/.config/bitrise/auth.yaml. Per the patterns guide, credentials live
// in their own file (separate from preferences in config.yaml) and at
// 0600 permissions. OS-keychain integration is intentionally deferred.
//
// The Bitrise API accepts both Personal Access Tokens (user-scoped) and
// Workspace API Tokens (workspace-scoped); they have identical wire format
// and authenticate the same way, so this package treats them as a single
// opaque token. If/when cross-workspace warnings become useful, a "type"
// field can be added back without breaking existing auth.yaml files.
package auth

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Auth is the on-disk shape of auth.yaml.
type Auth struct {
	Token string `yaml:"token,omitempty"`
}

// TokenType returns "PAT", "WAT", or "unknown" based on the token prefix.
func TokenType(token string) string {
	switch {
	case strings.HasPrefix(token, "bitpat_"):
		return "PAT"
	case strings.HasPrefix(token, "bitwat_"):
		return "WAT"
	default:
		return "unknown"
	}
}

// Path returns the absolute path to the auth file (whether or not it exists).
func Path() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("locate user home dir: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "bitrise", "auth.yaml"), nil
}

// Load reads the auth file. A missing file returns the zero Auth so
// first-time users don't see failures.
func Load() (Auth, error) {
	p, err := Path()
	if err != nil {
		return Auth{}, err
	}
	data, err := os.ReadFile(p) //nolint:gosec // p is derived from XDG_CONFIG_HOME / user home, not user input
	if errors.Is(err, fs.ErrNotExist) {
		return Auth{}, nil
	}
	if err != nil {
		return Auth{}, fmt.Errorf("read %s: %w", p, err)
	}
	var a Auth
	if err := yaml.Unmarshal(data, &a); err != nil {
		return Auth{}, fmt.Errorf("parse %s: %w", p, err)
	}
	return a, nil
}

// Save atomically writes a to disk with 0600 permissions, creating the
// parent directory (0700) if needed.
func Save(a Auth) error {
	if a.Token == "" {
		return fmt.Errorf("refusing to save auth with empty token")
	}
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(&a)
	if err != nil {
		return fmt.Errorf("marshal auth: %w", err)
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, p); err != nil {
		return fmt.Errorf("install %s: %w", p, err)
	}
	return nil
}

// Clear removes the auth file. A non-existent file is not an error.
func Clear() error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", p, err)
	}
	return nil
}
