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
// entries are key=value, saved entries are key=savedInputID. Returns an error
// on the first malformed entry; later iterations don't run.
func ParseSessionInputs(plain, secret, saved []string) ([]internalrde.SessionInputValue, error) {
	out := make([]internalrde.SessionInputValue, 0, len(plain)+len(secret)+len(saved))
	for _, kv := range plain {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("--input %q: expected key=value", kv)
		}
		out = append(out, internalrde.SessionInputValue{Key: k, Value: v})
	}
	for _, kv := range secret {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("--secret-input %q: expected key=value", kv)
		}
		out = append(out, internalrde.SessionInputValue{Key: k, Value: v, IsSecret: true})
	}
	for _, kv := range saved {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" || v == "" {
			return nil, fmt.Errorf("--saved-input %q: expected key=savedInputID", kv)
		}
		out = append(out, internalrde.SessionInputValue{Key: k, SavedInputID: v})
	}
	return out, nil
}
