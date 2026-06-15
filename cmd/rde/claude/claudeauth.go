package claude

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

// Claude Code auth env-var names — also the saved-input keys the RDE control
// plane uses for a configured Subscription Token / API Key.
const (
	envOAuthToken = "CLAUDE_CODE_OAUTH_TOKEN" //nolint:gosec // G101: env-var / saved-input name, not a credential
	envAPIKey     = "ANTHROPIC_API_KEY"       //nolint:gosec // G101: env-var / saved-input name, not a credential
)

// sourceSetupToken labels a credential freshly minted by `claude setup-token`.
const sourceSetupToken = "claude setup-token"

// forwardCred is a local Claude Code credential to forward into the session.
type forwardCred struct {
	EnvVar  string // env var to set in the session (CLAUDE_CODE_OAUTH_TOKEN / ANTHROPIC_API_KEY)
	Value   string // the token/key value
	Source  string // short human-readable origin, for logging
	Persist bool   // whether to save it on the control plane for future sessions
}

// controlPlaneHasClaudeToken reports whether the user has a Claude Code token
// configured on the control plane — i.e. a saved input keyed
// CLAUDE_CODE_OAUTH_TOKEN or ANTHROPIC_API_KEY. Saved inputs are user-scoped,
// so no workspace is needed.
func controlPlaneHasClaudeToken(ctx context.Context, svc *internalrde.Service) (bool, error) {
	inputs, err := svc.ListSavedInputs(ctx)
	if err != nil {
		return false, err
	}
	for _, in := range inputs {
		if in.Key == envOAuthToken || in.Key == envAPIKey {
			return true, nil
		}
	}
	return false, nil
}

// existingLocalCredential resolves a Claude Code credential already present on
// the machine, without any interaction. Precedence mirrors how `claude` itself
// resolves auth: an explicit env var wins, then the credentials file. Returns
// ok=false when nothing is found (the caller may then mint one).
func existingLocalCredential() (forwardCred, bool) {
	if v := os.Getenv(envOAuthToken); v != "" {
		return forwardCred{EnvVar: envOAuthToken, Value: v, Source: "$" + envOAuthToken, Persist: true}, true
	}
	if v := os.Getenv(envAPIKey); v != "" {
		return forwardCred{EnvVar: envAPIKey, Value: v, Source: "$" + envAPIKey, Persist: true}, true
	}
	if v, ok := credentialsFileToken(); ok {
		// The credentials-file OAuth token is short-lived (subscription
		// access token), so don't persist it on the control plane.
		return forwardCred{EnvVar: envOAuthToken, Value: v, Source: credentialsFilePath()}, true
	}
	return forwardCred{}, false
}

// mintSetupToken runs `claude setup-token` to obtain a long-lived OAuth token.
// It's interactive (browser auth): stdin/stderr stay wired to the terminal so
// the user can complete the flow; the token is read from stdout.
func mintSetupToken(ctx context.Context) (forwardCred, bool) {
	c := exec.CommandContext(ctx, "claude", "setup-token")
	c.Stdin = os.Stdin
	c.Stderr = os.Stderr
	out, err := c.Output()
	if err != nil {
		return forwardCred{}, false
	}
	token := lastNonEmptyLine(string(out))
	if token == "" {
		return forwardCred{}, false
	}
	return forwardCred{EnvVar: envOAuthToken, Value: token, Source: sourceSetupToken, Persist: true}, true
}

// lastNonEmptyLine returns the last non-blank, trimmed line of s — `claude
// setup-token` prints the token as its final output line.
func lastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(lines[i]); t != "" {
			return t
		}
	}
	return ""
}

// credentialsFilePath returns the path to Claude Code's credentials file,
// honoring $CLAUDE_CONFIG_DIR and falling back to ~/.claude.
func credentialsFilePath() string {
	dir := os.Getenv("CLAUDE_CONFIG_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".claude")
	}
	return filepath.Join(dir, ".credentials.json")
}

func credentialsFileToken() (string, bool) {
	p := credentialsFilePath()
	if p == "" {
		return "", false
	}
	data, err := os.ReadFile(p) //nolint:gosec // p is the user's own Claude Code credentials file under $HOME/$CLAUDE_CONFIG_DIR
	if err != nil {
		return "", false
	}
	return parseClaudeAccessToken(data)
}

// parseClaudeAccessToken extracts the OAuth access token from a Claude Code
// credentials blob ({"claudeAiOauth":{"accessToken":…}}). If the blob isn't
// that JSON shape, the trimmed raw bytes are treated as a bare token.
func parseClaudeAccessToken(data []byte) (string, bool) {
	var creds struct {
		ClaudeAiOauth struct {
			AccessToken string `json:"accessToken"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal(data, &creds); err == nil && creds.ClaudeAiOauth.AccessToken != "" {
		return creds.ClaudeAiOauth.AccessToken, true
	}
	if raw := strings.TrimSpace(string(data)); raw != "" && !strings.HasPrefix(raw, "{") {
		return raw, true
	}
	return "", false
}
