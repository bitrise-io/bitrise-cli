// Package cmdtest provides shared test helpers for cmd sub-packages.
package cmdtest

import (
	"io"
	"net/http"
	"os"
	"testing"
)

// RunIsolated runs the package's tests with a sandboxed config home so the
// token-refresh path (auth.Load) can never read the developer's real
// ~/.config/bitrise/auth.yaml. Without this, any test that builds an API
// client refreshes the dev's expired OAuth token against an unset endpoint
// and fails — while CI (no auth.yaml) stays green. Use from a package TestMain:
//
//	func TestMain(m *testing.M) { os.Exit(cmdtest.RunIsolated(m)) }
//
// XDG_CONFIG_HOME and BITRISE_TOKEN are the only env inputs that reach these
// tests (config is otherwise injected via config.WithResolved); sandboxing
// both makes the run hermetic regardless of the developer's machine.
func RunIsolated(m *testing.M) int {
	dir, err := os.MkdirTemp("", "bcli-test")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := os.Setenv("XDG_CONFIG_HOME", dir); err != nil {
		panic(err)
	}
	if err := os.Unsetenv("BITRISE_TOKEN"); err != nil { // don't let a dev's exported token mask behavior
		panic(err)
	}
	return m.Run()
}

// AppPassthrough wraps a handler so that GET /apps returns an empty app list,
// letting name resolution fall through to slug-passthrough mode. Use this when
// a test server only handles a specific endpoint and name-resolution would
// otherwise hit an unexpected path.
func AppPassthrough(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/apps" {
			_, _ = io.WriteString(w, `{"data":[],"paging":{}}`)
			return
		}
		h(w, r)
	})
}
