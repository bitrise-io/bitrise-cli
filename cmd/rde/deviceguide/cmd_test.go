package deviceguide

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func TestDeviceGuide(t *testing.T) {
	for _, tc := range []struct{ arg, want string }{
		{"", "PREVIEW_DEVICE_STATE_READY"},
		{"ios", "serve-sim"},
		{"android", "uiautomator"},
	} {
		c := NewCmd()
		var out bytes.Buffer
		c.SetOut(&out)
		if tc.arg != "" {
			c.SetArgs([]string{tc.arg})
		} else {
			c.SetArgs([]string{})
		}
		if err := c.Execute(); err != nil {
			t.Fatalf("%q: %v", tc.arg, err)
		}
		if !strings.Contains(out.String(), tc.want) {
			t.Errorf("%q: output missing %q", tc.arg, tc.want)
		}
	}
	c := NewCmd()
	c.SetOut(&bytes.Buffer{})
	c.SetErr(&bytes.Buffer{})
	c.SetArgs([]string{"tvos"})
	if err := c.Execute(); err == nil {
		t.Errorf("unknown platform must error")
	}
}

// TestDeviceGuide_RejectsJSON: the inherited --output json has no shape for
// a Markdown guide, so the command must refuse it instead of printing raw
// Markdown where a caller expects a JSON object.
func TestDeviceGuide_RejectsJSON(t *testing.T) {
	for _, arg := range [][]string{{}, {"ios"}} {
		c := NewCmd()
		var out bytes.Buffer
		c.SetOut(&out)
		c.SetErr(&bytes.Buffer{})
		c.SetArgs(arg)
		c.SetContext(config.WithResolved(context.Background(), config.Resolved{Output: output.JSON}))
		err := c.Execute()
		if err == nil || !strings.Contains(err.Error(), "json") {
			t.Errorf("args %v: error = %v, want --output json rejection", arg, err)
		}
		// cobra echoes the usage on error; the guide body itself must not
		// have been printed.
		if strings.Contains(out.String(), "PREVIEW_DEVICE_STATE_READY") || strings.Contains(out.String(), "serve-sim") {
			t.Errorf("args %v: guide must not be printed when --output json is rejected:\n%s", arg, out.String())
		}
	}
}
