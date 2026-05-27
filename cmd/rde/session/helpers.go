package session

import (
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/bitrise-io/bitrise-cli/internal/output/style"
)

// formatTime is the canonical timestamp format for session human output.
// Empty pointer renders as "" so renderers can shove it directly into a
// table cell without conditionals.
func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04 UTC")
}

// statusStyle picks the right semantic color for a session status string.
// The status comes from internal/rde — already lowercased, prefix-stripped.
func statusStyle(s style.Styles, status string) lipgloss.Style {
	switch status {
	case "running":
		return s.BuildStatus("success")
	case "pending", "starting", "terminating", "draining":
		return s.BuildStatus("in-progress")
	case "terminated", "drained":
		return s.Dim
	case "failed":
		return s.BuildStatus("failed")
	}
	return s.Dim
}
