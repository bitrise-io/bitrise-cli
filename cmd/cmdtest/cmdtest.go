// Package cmdtest provides shared test helpers for cmd sub-packages.
package cmdtest

import (
	"io"
	"net/http"
)

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
