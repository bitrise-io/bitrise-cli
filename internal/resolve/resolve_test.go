package resolve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/bitrise-io/bitrise-cli/bitriseapi"
	"github.com/bitrise-io/bitrise-cli/internal/cache"
)

func fakeAPI(t *testing.T, handler http.HandlerFunc) *bitriseapi.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return bitriseapi.New(srv.URL, "test-token")
}

func appsBody(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/apps" {
			_, _ = w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}
}

func orgsBody(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/organizations" {
			_, _ = w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}
}

func TestAppSlug_LiteralSlugPassthrough(t *testing.T) {
	var called bool
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{"data":[],"paging":{}}`))
	})
	r := New(client, nil)
	slug, err := r.AppSlug(context.Background(), "abc12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slug != "abc12345" {
		t.Errorf("expected passthrough slug abc12345, got %q", slug)
	}
	if !called {
		t.Error("expected API to be called even for slug-like input")
	}
}

func TestAppSlug_NameResolution(t *testing.T) {
	var gotTitle string
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotTitle, _ = url.QueryUnescape(r.URL.Query().Get("title"))
		_, _ = w.Write([]byte(`{"data":[{"slug":"abc12345","title":"My App","owner":{}}],"paging":{}}`))
	})
	r := New(client, nil)
	slug, err := r.AppSlug(context.Background(), "My App")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slug != "abc12345" {
		t.Errorf("expected abc12345, got %q", slug)
	}
	if gotTitle != "My App" {
		t.Errorf("expected title filter 'My App', got %q", gotTitle)
	}
}

func TestAppSlug_CaseInsensitive(t *testing.T) {
	client := fakeAPI(t, appsBody(`{"data":[{"slug":"abc12345","title":"My App","owner":{}}],"paging":{}}`))
	r := New(client, nil)
	slug, err := r.AppSlug(context.Background(), "my app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slug != "abc12345" {
		t.Errorf("expected abc12345, got %q", slug)
	}
}

func TestAppSlug_AmbiguousError(t *testing.T) {
	client := fakeAPI(t, appsBody(`{"data":[
		{"slug":"slug-1","title":"My App","owner":{}},
		{"slug":"slug-2","title":"My App","owner":{}}
	],"paging":{}}`))
	r := New(client, nil)
	_, err := r.AppSlug(context.Background(), "My App")
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
}

func TestAppSlug_CacheHit(t *testing.T) {
	var apiCalled bool
	client := fakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		apiCalled = true
		_, _ = w.Write([]byte(`{"data":[],"paging":{}}`))
	})
	c := cache.New()
	c.SetApp("My App", "cached-slug")

	r := New(client, c)
	slug, err := r.AppSlug(context.Background(), "My App")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slug != "cached-slug" {
		t.Errorf("expected cached-slug, got %q", slug)
	}
	if apiCalled {
		t.Error("API should not be called on cache hit")
	}
}

func TestAppSlug_PopulatesCache(t *testing.T) {
	client := fakeAPI(t, appsBody(`{"data":[{"slug":"abc12345","title":"My App","owner":{}}],"paging":{}}`))
	c := cache.New()
	r := New(client, c)
	_, err := r.AppSlug(context.Background(), "My App")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slug, ok := c.LookupApp("My App"); !ok || slug != "abc12345" {
		t.Errorf("cache not populated: slug=%q ok=%v", slug, ok)
	}
}

func TestWorkspaceID_NameResolution(t *testing.T) {
	client := fakeAPI(t, orgsBody(`{"data":[{"slug":"acme-corp","name":"Acme Corp"}],"paging":{}}`))
	r := New(client, nil)
	slug, err := r.WorkspaceSlug(context.Background(), "Acme Corp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slug != "acme-corp" {
		t.Errorf("expected acme-corp, got %q", slug)
	}
}

func TestWorkspaceID_LiteralSlugPassthrough(t *testing.T) {
	client := fakeAPI(t, orgsBody(`{"data":[],"paging":{}}`))
	r := New(client, nil)
	slug, err := r.WorkspaceSlug(context.Background(), "acme-corp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slug != "acme-corp" {
		t.Errorf("expected passthrough acme-corp, got %q", slug)
	}
}

func TestWorkspaceID_CacheHit(t *testing.T) {
	var apiCalled bool
	client := fakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		apiCalled = true
		_, _ = w.Write([]byte(`{"data":[],"paging":{}}`))
	})
	c := cache.New()
	c.SetWorkspace("Acme Corp", "acme-corp")

	r := New(client, c)
	slug, err := r.WorkspaceSlug(context.Background(), "Acme Corp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slug != "acme-corp" {
		t.Errorf("expected acme-corp, got %q", slug)
	}
	if apiCalled {
		t.Error("API should not be called on cache hit")
	}
}

func TestResolveApp_NameMatch_ReturnsFull(t *testing.T) {
	var calls int
	client := fakeAPI(t, appsBody(`{"data":[{"slug":"abc12345","title":"My App","provider":"github","repo_url":"https://github.com/x/y","owner":{"slug":"acme","account_type":"Organization"}}],"paging":{}}`))
	_ = calls
	r := New(client, nil)
	app, fetched, err := r.ResolveApp(context.Background(), "My App")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fetched {
		t.Error("expected fetched=true on name match")
	}
	if app.Slug != "abc12345" || app.Title != "My App" || app.Provider != "github" {
		t.Errorf("unexpected app: %+v", app)
	}
}

func TestResolveApp_NoMatch_Passthrough(t *testing.T) {
	client := fakeAPI(t, appsBody(`{"data":[],"paging":{}}`))
	r := New(client, nil)
	app, fetched, err := r.ResolveApp(context.Background(), "abc12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fetched {
		t.Error("expected fetched=false on passthrough")
	}
	if app.Slug != "abc12345" {
		t.Errorf("expected slug=abc12345, got %q", app.Slug)
	}
}

func TestResolveApp_CacheHit_NotFetched(t *testing.T) {
	var apiCalled bool
	client := fakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		apiCalled = true
		_, _ = w.Write([]byte(`{"data":[],"paging":{}}`))
	})
	c := cache.New()
	c.SetApp("My App", "cached-slug")

	r := New(client, c)
	app, fetched, err := r.ResolveApp(context.Background(), "My App")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fetched {
		t.Error("expected fetched=false on cache hit (full data not cached)")
	}
	if app.Slug != "cached-slug" {
		t.Errorf("expected slug=cached-slug, got %q", app.Slug)
	}
	if apiCalled {
		t.Error("API should not be called on cache hit")
	}
}

func TestResolveApp_AmbiguousError(t *testing.T) {
	client := fakeAPI(t, appsBody(`{"data":[
		{"slug":"slug-1","title":"My App","owner":{}},
		{"slug":"slug-2","title":"My App","owner":{}}
	],"paging":{}}`))
	r := New(client, nil)
	_, _, err := r.ResolveApp(context.Background(), "My App")
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
}

func TestWorkspaceID_AmbiguousError(t *testing.T) {
	client := fakeAPI(t, orgsBody(`{"data":[
		{"slug":"ws-1","name":"Acme Corp"},
		{"slug":"ws-2","name":"Acme Corp"}
	],"paging":{}}`))
	r := New(client, nil)
	_, err := r.WorkspaceSlug(context.Background(), "Acme Corp")
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
}
