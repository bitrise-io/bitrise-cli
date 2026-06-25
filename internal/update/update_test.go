package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// fixedNow is an arbitrary reference instant; tests offset from it so the
// 24h interval logic is deterministic.
var fixedNow = time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)

// releaseServer returns an httptest server that answers the latest-release
// endpoint for repo with the given tag, and counts how many times it was hit.
func releaseServer(t *testing.T, repo, tag string) (*httptest.Server, *int) {
	t.Helper()
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		want := "/repos/" + repo + "/releases/latest"
		if r.URL.Path != want {
			t.Errorf("unexpected path %q, want %q", r.URL.Path, want)
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("request missing User-Agent header (GitHub rejects these)")
		}
		_ = json.NewEncoder(w).Encode(release{
			TagName: tag,
			HTMLURL: "https://github.com/" + repo + "/releases/tag/" + tag,
		})
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func newTestChecker(t *testing.T, current, apiBaseURL string) *Checker {
	t.Helper()
	return &Checker{
		Current:    current,
		Repo:       DefaultRepo,
		APIBaseURL: apiBaseURL,
		StatePath:  filepath.Join(t.TempDir(), stateFileName),
		HTTP:       &http.Client{Timeout: 2 * time.Second},
		Now:        func() time.Time { return fixedNow },
		Interval:   24 * time.Hour,
	}
}

func TestCheck_NotifiesWhenNewer(t *testing.T) {
	srv, hits := releaseServer(t, DefaultRepo, "v2.1.0")
	c := newTestChecker(t, "1.0.0", srv.URL)

	notice, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if notice == nil {
		t.Fatal("expected a notice, got nil")
	}
	if notice.Current != "1.0.0" || notice.Latest != "2.1.0" {
		t.Fatalf("notice = %+v, want current 1.0.0 → latest 2.1.0", notice)
	}
	if notice.URL == "" {
		t.Error("notice has no release URL")
	}
	if *hits != 1 {
		t.Fatalf("server hit %d times, want 1", *hits)
	}
}

func TestCheck_NoNoticeWhenCurrent(t *testing.T) {
	srv, _ := releaseServer(t, DefaultRepo, "v1.0.0")
	c := newTestChecker(t, "1.0.0", srv.URL)

	notice, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if notice != nil {
		t.Fatalf("expected no notice, got %+v", notice)
	}
}

func TestCheck_NoNoticeWhenAhead(t *testing.T) {
	// A local build ahead of the latest tag must not be told to "upgrade".
	srv, _ := releaseServer(t, DefaultRepo, "v1.0.0")
	c := newTestChecker(t, "1.2.0", srv.URL)

	notice, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if notice != nil {
		t.Fatalf("expected no notice, got %+v", notice)
	}
}

func TestCheck_UsesFreshCacheWithoutNetwork(t *testing.T) {
	// The server must NOT be hit when the cache is younger than the interval.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("network call made despite a fresh cache: %s", r.URL.Path)
	}))
	t.Cleanup(srv.Close)

	statePath := filepath.Join(t.TempDir(), stateFileName)
	writeState(t, statePath, State{
		LastCheckedAt: fixedNow.Add(-1 * time.Hour), // well within 24h
		LatestVersion: "v3.0.0",
		ReleaseURL:    "https://example.test/v3.0.0",
	})

	c := &Checker{
		Current:    "1.0.0",
		APIBaseURL: srv.URL,
		StatePath:  statePath,
		Now:        func() time.Time { return fixedNow },
		Interval:   24 * time.Hour,
	}
	notice, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if notice == nil || notice.Latest != "3.0.0" {
		t.Fatalf("expected cached notice 3.0.0, got %+v", notice)
	}
}

func TestCheck_RefreshesStaleCacheAndPersists(t *testing.T) {
	srv, hits := releaseServer(t, DefaultRepo, "v4.0.0")
	statePath := filepath.Join(t.TempDir(), stateFileName)
	writeState(t, statePath, State{
		LastCheckedAt: fixedNow.Add(-48 * time.Hour), // older than the interval
		LatestVersion: "v2.0.0",
	})

	c := &Checker{
		Current:    "1.0.0",
		Repo:       DefaultRepo,
		APIBaseURL: srv.URL,
		StatePath:  statePath,
		Now:        func() time.Time { return fixedNow },
		Interval:   24 * time.Hour,
	}
	notice, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if notice == nil || notice.Latest != "4.0.0" {
		t.Fatalf("expected refreshed notice 4.0.0, got %+v", notice)
	}
	if *hits != 1 {
		t.Fatalf("server hit %d times, want 1", *hits)
	}

	got := readState(t, statePath)
	if got.LatestVersion != "v4.0.0" {
		t.Errorf("persisted LatestVersion = %q, want v4.0.0", got.LatestVersion)
	}
	if !got.LastCheckedAt.Equal(fixedNow) {
		t.Errorf("persisted LastCheckedAt = %v, want %v", got.LastCheckedAt, fixedNow)
	}
}

func TestCheck_NetworkErrorFallsBackToCacheAndBacksOff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	statePath := filepath.Join(t.TempDir(), stateFileName)
	writeState(t, statePath, State{
		LastCheckedAt: fixedNow.Add(-48 * time.Hour),
		LatestVersion: "v5.0.0",
		ReleaseURL:    "https://example.test/v5.0.0",
	})

	c := &Checker{
		Current:    "1.0.0",
		Repo:       DefaultRepo,
		APIBaseURL: srv.URL,
		StatePath:  statePath,
		Now:        func() time.Time { return fixedNow },
		Interval:   24 * time.Hour,
	}
	notice, err := c.Check(context.Background())
	if err == nil {
		t.Fatal("expected a fetch error to be reported")
	}
	// A usable cached value still produces a notice even though the refresh failed.
	if notice == nil || notice.Latest != "5.0.0" {
		t.Fatalf("expected cached notice 5.0.0, got %+v", notice)
	}
	// The failed attempt is recorded so we back off instead of re-fetching.
	if got := readState(t, statePath); !got.LastCheckedAt.Equal(fixedNow) {
		t.Errorf("LastCheckedAt = %v, want it advanced to %v (backoff)", got.LastCheckedAt, fixedNow)
	}
}

func TestCheck_FirstRunNetworkErrorIsQuiet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	c := newTestChecker(t, "1.0.0", srv.URL) // no pre-existing cache
	notice, err := c.Check(context.Background())
	if err == nil {
		t.Fatal("expected a fetch error")
	}
	if notice != nil {
		t.Fatalf("expected no notice on a failed first check, got %+v", notice)
	}
}

func TestCheck_StateFileIs0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes are not meaningful on Windows")
	}
	srv, _ := releaseServer(t, DefaultRepo, "v2.0.0")
	c := newTestChecker(t, "1.0.0", srv.URL)
	if _, err := c.Check(context.Background()); err != nil {
		t.Fatalf("Check: %v", err)
	}
	info, err := os.Stat(c.StatePath)
	if err != nil {
		t.Fatalf("stat state file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("state file perm = %o, want 600", perm)
	}
}

func TestCheck_EmptyStatePathDisablesCaching(t *testing.T) {
	srv, hits := releaseServer(t, DefaultRepo, "v2.0.0")
	c := newTestChecker(t, "1.0.0", srv.URL)
	c.StatePath = "" // caching off: every Check hits the network

	for i := range 2 {
		if _, err := c.Check(context.Background()); err != nil {
			t.Fatalf("Check #%d: %v", i, err)
		}
	}
	if *hits != 2 {
		t.Fatalf("server hit %d times, want 2 (no caching)", *hits)
	}
}

func writeState(t *testing.T, path string, st State) {
	t.Helper()
	if err := saveState(path, st); err != nil {
		t.Fatalf("seed state: %v", err)
	}
}

func readState(t *testing.T, path string) State {
	t.Helper()
	st, err := loadState(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	return st
}
