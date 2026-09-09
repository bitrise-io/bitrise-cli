package deviceguide

import (
	"bytes"
	"strings"
	"testing"
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
