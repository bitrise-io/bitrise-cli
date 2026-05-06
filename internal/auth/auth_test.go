package auth

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPath_HonorsXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/xdg")
	got, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	want := filepath.Join("/custom/xdg", "bitrise", "auth.yaml")
	if got != want {
		t.Fatalf("Path = %q, want %q", got, want)
	}
}

func TestPath_FallsBackToHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	got, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if !strings.HasSuffix(got, filepath.Join(".config", "bitrise", "auth.yaml")) {
		t.Fatalf("expected fallback to ~/.config/bitrise/auth.yaml, got %q", got)
	}
}

func TestSaveLoadClear_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// Initially empty.
	got, err := Load()
	if err != nil {
		t.Fatalf("Load empty: %v", err)
	}
	if got != (Auth{}) {
		t.Fatalf("expected zero Auth, got %+v", got)
	}

	// Save then Load round-trip.
	want := Auth{Token: "secret-pat-123"}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Fatalf("round-trip: got %+v, want %+v", got, want)
	}

	// Verify file perms (skip on Windows).
	if runtime.GOOS != "windows" {
		p := filepath.Join(dir, "bitrise", "auth.yaml")
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("file perm = %o, want 0600", info.Mode().Perm())
		}
		// Parent dir should be 0700.
		dirInfo, err := os.Stat(filepath.Dir(p))
		if err != nil {
			t.Fatal(err)
		}
		if dirInfo.Mode().Perm() != 0o700 {
			t.Fatalf("dir perm = %o, want 0700", dirInfo.Mode().Perm())
		}
	}

	// Clear removes the file.
	if err := Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	got, err = Load()
	if err != nil {
		t.Fatalf("Load after Clear: %v", err)
	}
	if got != (Auth{}) {
		t.Fatalf("after Clear, got %+v", got)
	}

	// Clear is idempotent.
	if err := Clear(); err != nil {
		t.Fatalf("second Clear: %v", err)
	}
}

func TestSave_RejectsEmptyToken(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := Save(Auth{Token: ""}); err == nil {
		t.Fatal("Save with empty token should fail")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "bitrise"), 0o700); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bitrise", "auth.yaml")
	if err := os.WriteFile(bad, []byte("this: is :: bad yaml\n: oops"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected error on malformed YAML")
	}
}

// Save survives an existing file (atomic replace).
func TestSave_OverwritesExisting(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := Save(Auth{Token: "first"}); err != nil {
		t.Fatal(err)
	}
	if err := Save(Auth{Token: "second"}); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Token != "second" {
		t.Fatalf("expected overwrite, got %q", got.Token)
	}
}
