// Package rdeflags holds flag-parsing helpers shared by the rde command
// groups — `session create` and `warm-pool create/update` take the same
// session-input flags and must read them identically.
package rdeflags

import (
	"fmt"
	"strings"

	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

// ParseSessionInputs converts the user-friendly --input / --secret-input /
// --saved-input flag values into SessionInputValue entries: plain and secret
// entries are key=value, saved entries are key=savedInputID. The key may
// carry a scope prefix, scope:key (on_claim:GH_TOKEN=…): the value is then
// for that scope only, and wins there over an unscoped one; without a
// prefix the value applies to every scope that defines the key. Returns an
// error on the first malformed entry; later iterations don't run.
func ParseSessionInputs(plain, secret, saved []string) ([]internalrde.SessionInputValue, error) {
	out := make([]internalrde.SessionInputValue, 0, len(plain)+len(secret)+len(saved))
	for _, kv := range plain {
		hook, k, v, err := splitInput("--input", kv, "key=value")
		if err != nil {
			return nil, err
		}
		out = append(out, internalrde.SessionInputValue{Key: k, Value: v, Hook: hook})
	}
	for _, kv := range secret {
		hook, k, v, err := splitInput("--secret-input", kv, "key=value")
		if err != nil {
			return nil, err
		}
		out = append(out, internalrde.SessionInputValue{Key: k, Value: v, IsSecret: true, Hook: hook})
	}
	for _, kv := range saved {
		hook, k, v, err := splitInput("--saved-input", kv, "key=savedInputID")
		if err != nil {
			return nil, err
		}
		if v == "" {
			return nil, fmt.Errorf("--saved-input %q: expected key=savedInputID", kv)
		}
		out = append(out, internalrde.SessionInputValue{Key: k, SavedInputID: v, Hook: hook})
	}
	return out, nil
}

// splitInput parses one [scope:]key=value flag value (form names the
// unprefixed shape for the error). hook is "" without a scope prefix; a
// prefix must be one of the known scopes (shared, on_create, on_claim,
// on_restore).
func splitInput(flag, kv, form string) (hook, key, value string, err error) {
	k, v, ok := strings.Cut(kv, "=")
	if !ok || k == "" {
		return "", "", "", fmt.Errorf("%s %q: expected %s (the key may carry a scope prefix, scope:key)", flag, kv, form)
	}
	if scope, bare, scoped := strings.Cut(k, ":"); scoped {
		if !isScope(scope) {
			return "", "", "", fmt.Errorf("%s %q: unknown scope %q (one of %s)", flag, kv, scope, strings.Join(internalrde.Scopes, ", "))
		}
		if bare == "" {
			return "", "", "", fmt.Errorf("%s %q: expected %s after the scope prefix", flag, kv, form)
		}
		hook, k = scope, bare
	}
	return hook, k, v, nil
}

func isScope(s string) bool {
	for _, scope := range internalrde.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}
