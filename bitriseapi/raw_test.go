package bitriseapi

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestRawRequest_MethodHeadersAndBody(t *testing.T) {
	var gotBody string
	fs := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"ok":true}}`))
	})

	hdr := http.Header{}
	hdr.Set("X-Custom", "yes")
	hdr.Set("Accept", "text/plain") // overrides the default Accept

	resp, err := fs.client("tok").RawRequest(context.Background(), http.MethodPost, "/apps",
		nil, hdr, strings.NewReader(`{"a":1}`))
	if err != nil {
		t.Fatalf("RawRequest: %v", err)
	}

	if fs.lastReq.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", fs.lastReq.Method)
	}
	if fs.lastReq.URL.Path != "/apps" {
		t.Errorf("path = %q, want /apps", fs.lastReq.URL.Path)
	}
	if got := fs.lastReq.Header.Get("Authorization"); got != "token tok" {
		t.Errorf("Authorization = %q", got)
	}
	if got := fs.lastReq.Header.Get("X-Custom"); got != "yes" {
		t.Errorf("X-Custom = %q", got)
	}
	if got := fs.lastReq.Header.Get("Accept"); got != "text/plain" {
		t.Errorf("Accept = %q, want overridden text/plain", got)
	}
	if gotBody != `{"a":1}` {
		t.Errorf("body = %q", gotBody)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("StatusCode = %d, want 201", resp.StatusCode)
	}
	if string(resp.Body) != `{"data":{"ok":true}}` {
		t.Errorf("Body = %q", resp.Body)
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("response Content-Type = %q", resp.Header.Get("Content-Type"))
	}
}

func TestRawRequest_PathJoiningAndQuery(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		query    url.Values
		wantPath string
		wantQ    map[string]string
	}{
		{"leading slash", "/apps", nil, "/apps", nil},
		{"slashless", "apps", nil, "/apps", nil},
		{"path with query merged", "/apps?a=1", url.Values{"b": {"2"}}, "/apps", map[string]string{"a": "1", "b": "2"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{}`))
			})
			_, err := fs.client("t").RawRequest(context.Background(), http.MethodGet, tc.path, tc.query, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if fs.lastReq.URL.Path != tc.wantPath {
				t.Errorf("path = %q, want %q", fs.lastReq.URL.Path, tc.wantPath)
			}
			for k, want := range tc.wantQ {
				if got := fs.lastReq.URL.Query().Get(k); got != want {
					t.Errorf("query %q = %q, want %q", k, got, want)
				}
			}
		})
	}
}

func TestRawRequest_AbsoluteURLPassthrough(t *testing.T) {
	fs := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})
	// Point the client at a different base, but pass an absolute URL — it must
	// be used verbatim, ignoring the base.
	c := New("https://example.invalid/v0.1", "t")
	_, err := c.RawRequest(context.Background(), http.MethodGet, fs.srv.URL+"/custom/path", nil, nil, nil)
	if err != nil {
		t.Fatalf("RawRequest: %v", err)
	}
	if fs.lastReq.URL.Path != "/custom/path" {
		t.Errorf("path = %q, want /custom/path", fs.lastReq.URL.Path)
	}
}

func TestRawRequest_NonSuccessIsNotError(t *testing.T) {
	fs := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	})
	resp, err := fs.client("t").RawRequest(context.Background(), http.MethodGet, "/nope", nil, nil, nil)
	if err != nil {
		t.Fatalf("RawRequest returned error for 404: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want 404", resp.StatusCode)
	}
	if string(resp.Body) != `{"message":"not found"}` {
		t.Errorf("Body = %q", resp.Body)
	}
}
