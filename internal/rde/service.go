// Package rde holds the business-logic layer for Remote Dev Environments.
//
// CLI-stable types (snake_case json tags) live here. The fromAPI mappers
// convert wire-format DTOs from bitriseapi/rde — they're the only place
// where backend renames affect `--output json`.
package rde

import (
	"fmt"
	"strings"
	"time"

	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
)

// Service exposes RDE operations to the cmd layer.
type Service struct {
	client *rdeapi.Client
}

// NewService returns a Service backed by the given RDE client. The client
// must be non-nil — every method makes a network call.
func NewService(client *rdeapi.Client) *Service {
	return &Service{client: client}
}

// parseTime is the shared timestamp parser. Backend emits RFC3339 strings;
// empty input round-trips as a nil pointer so JSON output omits the field.
func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// errClient is the canned "client not configured" error every method
// guards against. Kept here to avoid copy-paste across files.
func errClient() error { return fmt.Errorf("RDE client not configured") }

// statusFromAPI converts the backend's SESSION_STATUS_* enum into a stable
// lowercase string. Falls back to the raw value (lowercased, prefix-stripped)
// for any status added after this code was written, so new statuses don't
// break callers — they just see a value that's still recognizable.
func statusFromAPI(s string) string {
	if s == "" {
		return ""
	}
	const prefix = "SESSION_STATUS_"
	v := strings.TrimPrefix(s, prefix)
	if v == "UNSPECIFIED" {
		return ""
	}
	return strings.ToLower(v)
}

// agentStatusFromAPI strips the AGENT_SESSION_STATUS_ prefix similarly.
func agentStatusFromAPI(s string) string {
	if s == "" {
		return ""
	}
	const prefix = "AGENT_SESSION_STATUS_"
	v := strings.TrimPrefix(s, prefix)
	if v == "UNSPECIFIED" {
		return ""
	}
	return strings.ToLower(v)
}

// diskStatusFromAPI strips the PERSISTENT_DISK_STATUS_ prefix.
func diskStatusFromAPI(s string) string {
	if s == "" {
		return ""
	}
	const prefix = "PERSISTENT_DISK_STATUS_"
	v := strings.TrimPrefix(s, prefix)
	if v == "UNSPECIFIED" {
		return ""
	}
	return strings.ToLower(v)
}
