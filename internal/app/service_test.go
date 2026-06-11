package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/bitrise-io/bitrise-cli/bitriseapi"
	"github.com/bitrise-io/bitrise-cli/internal/resolve"
)

// fakeAPI returns an httptest server backing a real bitriseapi.Client. We
// don't introduce a fake-client interface; testing through the real client
// also exercises the wire-format mapping.
func fakeAPI(t *testing.T, handler http.HandlerFunc) *bitriseapi.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return bitriseapi.New(srv.URL, "test-token")
}

func TestService_List_MapsAPIShape(t *testing.T) {
	client := fakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
  "data": [
    {
      "slug": "app-1",
      "title": "First",
      "provider": "github",
      "repo_url": "https://github.com/x/y",
      "project_type": "android",
      "is_disabled": false,
      "owner": {"account_type": "Organization", "slug": "acme"}
    },
    {
      "slug": "app-2",
      "title": "Second",
      "provider": "gitlab",
      "repo_url": "https://gitlab.com/x/y",
      "project_type": "ios",
      "is_disabled": true,
      "owner": {"account_type": "User", "slug": "bob"}
    }
  ],
  "paging": {"next": "page-2"}
}`))
	})
	svc := NewService(client)

	res, err := svc.List(context.Background(), ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(res.Items))
	}
	want := App{
		Slug:        "app-1",
		Title:       "First",
		Provider:    "github",
		RepoURL:     "https://github.com/x/y",
		OwnerSlug:   "acme",
		ProjectType: "android",
		IsDisabled:  false,
	}
	if res.Items[0] != want {
		t.Errorf("item[0] = %+v, want %+v", res.Items[0], want)
	}
	if !res.Items[1].IsDisabled {
		t.Errorf("item[1].IsDisabled should be true")
	}
	if res.NextCursor != "page-2" {
		t.Errorf("NextCursor = %q, want page-2", res.NextCursor)
	}
}

func TestService_List_PassesOptionsAsQueryParams(t *testing.T) {
	var gotQuery url.Values
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	svc := NewService(client)

	_, err := svc.List(context.Background(), ListOptions{
		Limit:       10,
		Cursor:      "cur",
		SortBy:      "created_at",
		Title:       "exact-title",
		ProjectType: "android",
	})
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{
		"limit":        "10",
		"next":         "cur",
		"sort_by":      "created_at",
		"title":        "exact-title",
		"project_type": "android",
	}
	for k, want := range checks {
		if got := gotQuery.Get(k); got != want {
			t.Errorf("query %q = %q, want %q", k, got, want)
		}
	}
}

func TestService_List_NilClientFails(t *testing.T) {
	svc := NewService(nil)
	if _, err := svc.List(context.Background(), ListOptions{}); err == nil {
		t.Fatal("expected error when client is nil")
	}
}

func TestService_List_PropagatesAPIError(t *testing.T) {
	client := fakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Unauthorized"}`))
	})
	svc := NewService(client)

	_, err := svc.List(context.Background(), ListOptions{})
	if err == nil {
		t.Fatal("expected error on 401")
	}
}

func TestService_View_HitsCorrectPath(t *testing.T) {
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/my-app" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"slug":"my-app","title":"App","provider":"github","owner":{"slug":"acme","account_type":"User"}}}`))
	})
	svc := NewService(client)

	got, err := svc.View(context.Background(), "my-app")
	if err != nil {
		t.Fatal(err)
	}
	want := App{Slug: "my-app", Title: "App", Provider: "github", OwnerSlug: "acme"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestService_ViewByNameOrSlug_NameMatch_SingleCall(t *testing.T) {
	var paths []string
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/apps":
			_, _ = w.Write([]byte(`{"data":[{"slug":"my-app","title":"My App","provider":"github","repo_url":"https://github.com/x/y","owner":{"slug":"acme","account_type":"Organization"}}],"paging":{}}`))
		default:
			http.NotFound(w, r)
		}
	})
	svc := NewService(client)
	r := resolve.New(client, nil)

	got, err := svc.ViewByNameOrSlug(context.Background(), r, "My App")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Slug != "my-app" || got.Title != "My App" {
		t.Errorf("unexpected app: %+v", got)
	}
	if len(paths) != 1 || paths[0] != "/apps" {
		t.Errorf("expected exactly one GET /apps call, got %v", paths)
	}
}

func TestService_ViewByNameOrSlug_SlugPassthrough_TwoCalls(t *testing.T) {
	var paths []string
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/apps":
			_, _ = w.Write([]byte(`{"data":[],"paging":{}}`))
		case "/apps/my-app":
			_, _ = w.Write([]byte(`{"data":{"slug":"my-app","title":"My App","provider":"github","owner":{"slug":"acme","account_type":"Organization"}}}`))
		default:
			http.NotFound(w, r)
		}
	})
	svc := NewService(client)
	r := resolve.New(client, nil)

	got, err := svc.ViewByNameOrSlug(context.Background(), r, "my-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Slug != "my-app" {
		t.Errorf("unexpected slug: %q", got.Slug)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 API calls (resolve + view), got %v", paths)
	}
}

func TestService_View_RequiresSlug(t *testing.T) {
	svc := NewService(fakeAPI(t, func(http.ResponseWriter, *http.Request) {}))
	if _, err := svc.View(context.Background(), ""); err == nil {
		t.Fatal("View with empty slug should fail")
	}
}

func TestService_View_NilClientFails(t *testing.T) {
	svc := NewService(nil)
	if _, err := svc.View(context.Background(), "x"); err == nil {
		t.Fatal("expected error when client is nil")
	}
}
