package bitriseapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeServer spins up an httptest server, captures the last request, and
// responds with whatever handler the test installs.
type fakeServer struct {
	t       *testing.T
	srv     *httptest.Server
	lastReq *http.Request
}

func newFakeServer(t *testing.T, handler http.HandlerFunc) *fakeServer {
	t.Helper()
	fs := &fakeServer{t: t}
	fs.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.lastReq = r
		handler(w, r)
	}))
	t.Cleanup(fs.srv.Close)
	return fs
}

func (fs *fakeServer) client(token string) *Client {
	return New(token, WithBaseURL(fs.srv.URL))
}

func TestApps_PassesAuthHeaderAndPath(t *testing.T) {
	fs := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"paging":{"total_item_count":0,"page_item_limit":50,"next":""}}`))
	})

	_, err := fs.client("my-token").Apps(context.Background(), AppsListOptions{})
	if err != nil {
		t.Fatalf("Apps: %v", err)
	}
	if got := fs.lastReq.URL.Path; got != "/apps" {
		t.Errorf("path = %q, want /apps", got)
	}
	if got := fs.lastReq.Header.Get("Authorization"); got != "token my-token" {
		t.Errorf("Authorization = %q, want %q", got, "token my-token")
	}
	if got := fs.lastReq.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q", got)
	}
}

func TestApps_EncodesQueryParams(t *testing.T) {
	fs := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})

	_, err := fs.client("t").Apps(context.Background(), AppsListOptions{
		SortBy:      "created_at",
		Next:        "abc-cursor",
		Limit:       25,
		Title:       "my app",
		ProjectType: "ios",
	})
	if err != nil {
		t.Fatal(err)
	}
	q := fs.lastReq.URL.Query()
	checks := map[string]string{
		"sort_by":      "created_at",
		"next":         "abc-cursor",
		"limit":        "25",
		"title":        "my app",
		"project_type": "ios",
	}
	for k, want := range checks {
		if got := q.Get(k); got != want {
			t.Errorf("query %q = %q, want %q", k, got, want)
		}
	}
}

func TestApps_OmitsUnsetParams(t *testing.T) {
	fs := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})

	_, err := fs.client("t").Apps(context.Background(), AppsListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if fs.lastReq.URL.RawQuery != "" {
		t.Errorf("expected no query params, got %q", fs.lastReq.URL.RawQuery)
	}
}

func TestApps_ParsesResponse(t *testing.T) {
	fs := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
  "data": [
    {
      "slug": "app-1",
      "title": "First",
      "provider": "github",
      "repo_url": "https://github.com/x/y",
      "project_type": "android",
      "is_disabled": false,
      "owner": {"account_type": "Organization", "name": "Acme", "slug": "acme"}
    },
    {
      "slug": "app-2",
      "title": "Second",
      "is_disabled": true,
      "owner": {"account_type": "User", "slug": "alice"}
    }
  ],
  "paging": {"total_item_count": 2, "page_item_limit": 50, "next": "next-cursor"}
}`))
	})

	page, err := fs.client("t").Apps(context.Background(), AppsListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("got %d items, want 2", len(page.Items))
	}
	first := page.Items[0]
	if first.Slug != "app-1" || first.Title != "First" || first.Provider != "github" {
		t.Errorf("first item: %+v", first)
	}
	if first.Owner.AccountType != "Organization" || first.Owner.Slug != "acme" {
		t.Errorf("first owner: %+v", first.Owner)
	}
	second := page.Items[1]
	if !second.IsDisabled {
		t.Error("second.IsDisabled should be true")
	}
	if page.Paging.Next != "next-cursor" || !page.Paging.HasMore() {
		t.Errorf("paging: %+v", page.Paging)
	}
}

func TestApps_PropagatesAPIError(t *testing.T) {
	fs := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Unauthorized"})
	})

	_, err := fs.client("bad-token").Apps(context.Background(), AppsListOptions{})
	if err == nil {
		t.Fatal("expected error on 401")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("StatusCode = %d, want 401", apiErr.StatusCode)
	}
	if apiErr.Message != "Unauthorized" {
		t.Errorf("Message = %q, want Unauthorized", apiErr.Message)
	}
}

func TestApps_NonJSONErrorBody(t *testing.T) {
	fs := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("upstream exploded"))
	})

	_, err := fs.client("t").Apps(context.Background(), AppsListOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApps_ContextCancellation(t *testing.T) {
	fs := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		// We never reach here because the context is already cancelled.
		w.WriteHeader(http.StatusOK)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fs.client("t").Apps(ctx, AppsListOptions{})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestMe(t *testing.T) {
	fs := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/me" {
			t.Errorf("path = %q, want /me", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"username":"alice","email":"alice@example.com","avatar_url":"https://x"}}`))
	})

	u, err := fs.client("t").Me(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if u.Username != "alice" || u.Email != "alice@example.com" {
		t.Errorf("got %+v", u)
	}
}
