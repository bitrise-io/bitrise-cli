// Package output formats command results as either human-readable text or
// machine-parseable JSON. JSON mode is the contract used by automation and
// AI agents; human mode is the default for interactive use.
package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// Format selects the rendering style for command output.
type Format string

const (
	// Human is the default human-friendly format (tables, key/value lines).
	// The wire value matches the conventional --output names used by gh,
	// kubectl, az, and the Bitrise CLI patterns guide.
	Human Format = "human"
	// JSON emits the response as indented JSON. The schema is part of the
	// CLI's stable contract — additive changes only.
	JSON Format = "json"
)

func (f Format) String() string { return string(f) }

// ParseFormat validates a user-supplied --output value. The empty string
// resolves to Human so callers can pass cmd flag values directly.
func ParseFormat(s string) (Format, error) {
	switch s {
	case "", "human":
		return Human, nil
	case "json":
		return JSON, nil
	default:
		return "", fmt.Errorf("unsupported output format %q (expected: human, json)", s)
	}
}

// Render writes v to w. In JSON mode v is marshaled directly; in Human mode
// the per-command renderHuman callback formats the value.
func Render[T any](w io.Writer, format Format, v T, renderHuman func(io.Writer, T) error) error {
	switch format {
	case JSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	case Human:
		if renderHuman == nil {
			return fmt.Errorf("no human renderer provided for value of type %T", v)
		}
		return renderHuman(w, v)
	default:
		return fmt.Errorf("unknown output format: %q", format)
	}
}
