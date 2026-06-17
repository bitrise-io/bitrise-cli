package localsession

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestProjectKey(t *testing.T) {
	for _, tc := range []struct {
		in, want string
	}{
		{"/Users/me/repo", "-Users-me-repo"},
		{"/Users/me/my.repo_x", "-Users-me-my.repo_x"},
		{"/a/b c/d", "-a-b-c-d"},
		{"", "-"},
		{"already-safe", "already-safe"},
	} {
		if got := projectKey(tc.in); got != tc.want {
			t.Errorf("projectKey(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	repoPath := "/work/repo"

	rec := Record{
		RDESessionID:    "sess-1",
		WorkspaceID:     "ws-1",
		Name:            "claude-abcd",
		ClaudeSessionID: "uuid-1",
		Repo:            "git@github.com:org/repo.git",
		RepoPath:        repoPath,
		Branch:          "feature",
		RemoteRepoDir:   "repo",
	}
	if err := Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(repoPath, "sess-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.RDESessionID != rec.RDESessionID || got.ClaudeSessionID != rec.ClaudeSessionID ||
		got.Branch != rec.Branch || got.RemoteRepoDir != rec.RemoteRepoDir {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Errorf("timestamps not stamped: created=%v updated=%v", got.CreatedAt, got.UpdatedAt)
	}
}

func TestSaveValidation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := Save(Record{RepoPath: "/x"}); err == nil {
		t.Error("expected error for missing RDE session ID")
	}
	if err := Save(Record{RDESessionID: "s"}); err == nil {
		t.Error("expected error for missing repo path")
	}
}

func TestFilePerms(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix perms")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	repoPath := "/work/repo"
	if err := Save(Record{RDESessionID: "s", RepoPath: repoPath}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	dir, _ := sessionsDir(repoPath)
	di, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if di.Mode().Perm() != 0o700 {
		t.Errorf("dir perm = %v, want 0700", di.Mode().Perm())
	}
	fi, err := os.Stat(filepath.Join(dir, "s.json"))
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("file perm = %v, want 0600", fi.Mode().Perm())
	}
}

func TestListAndLatestOrdering(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	repoPath := "/work/repo"

	if err := Save(Record{RDESessionID: "old", RepoPath: repoPath}); err != nil {
		t.Fatalf("Save old: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := Save(Record{RDESessionID: "new", RepoPath: repoPath}); err != nil {
		t.Fatalf("Save new: %v", err)
	}

	recs, err := ListByProject(repoPath)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
	if recs[0].RDESessionID != "new" {
		t.Errorf("newest-first ordering broken: %q first", recs[0].RDESessionID)
	}

	latest, ok, err := Latest(repoPath)
	if err != nil || !ok {
		t.Fatalf("Latest: ok=%v err=%v", ok, err)
	}
	if latest.RDESessionID != "new" {
		t.Errorf("Latest = %q, want new", latest.RDESessionID)
	}
}

func TestListMissingProjectIsEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	recs, err := ListByProject("/never/saved")
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("got %d records, want 0", len(recs))
	}
	if _, ok, err := Latest("/never/saved"); ok || err != nil {
		t.Errorf("Latest on empty: ok=%v err=%v", ok, err)
	}
}

func TestRemove(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	repoPath := "/work/repo"
	if err := Save(Record{RDESessionID: "s", RepoPath: repoPath}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := Remove(repoPath, "s"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := Load(repoPath, "s"); err == nil {
		t.Error("expected Load to fail after Remove")
	}
	// Removing a missing record is not an error.
	if err := Remove(repoPath, "s"); err != nil {
		t.Errorf("Remove missing: %v", err)
	}
}

func TestDisplayName(t *testing.T) {
	if got := (Record{Name: "claude-x"}).DisplayName(); got != "claude-x" {
		t.Errorf("DisplayName without title = %q", got)
	}
	if got := (Record{Name: "claude-x", AITitle: "Fix the bug"}).DisplayName(); got != "Fix the bug" {
		t.Errorf("DisplayName with title = %q", got)
	}
}
