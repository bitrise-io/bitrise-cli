package config

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
	want := filepath.Join("/custom/xdg", "bitrise", "config.yaml")
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
	if !strings.HasSuffix(got, filepath.Join(".config", "bitrise", "config.yaml")) {
		t.Fatalf("expected fallback to ~/.config/bitrise/config.yaml, got %q", got)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		c       Config
		wantErr bool
	}{
		{"empty", Config{}, false},
		{"valid output", Config{Output: "json"}, false},
		{"valid url", Config{APIBaseURL: "https://api.example.com"}, false},
		{"all set", Config{Output: "human", APIBaseURL: "https://x", AppSlug: "s"}, false},
		{"bad output", Config{Output: "yaml"}, true},
		{"bad url no scheme", Config{APIBaseURL: "api.example.com"}, true},
		{"bad url empty host", Config{APIBaseURL: "https://"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.c.Validate()
			if c.wantErr && err == nil {
				t.Fatalf("expected error for %+v", c.c)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("unexpected error for %+v: %v", c.c, err)
			}
		})
	}
}

func TestGetSetUnset(t *testing.T) {
	c := Config{}

	// Unknown keys reject.
	if err := c.Set("frobnicate", "x"); err == nil {
		t.Fatal("Set with unknown key should fail")
	}
	if _, err := c.Get("frobnicate"); err == nil {
		t.Fatal("Get with unknown key should fail")
	}

	// Set + Get round-trip on every known key.
	values := map[string]string{
		KeyOutput:     "json",
		KeyAppSlug:    "stub-slug",
		KeyAPIBaseURL: "https://api.example.com",
	}
	for k, v := range values {
		if err := c.Set(k, v); err != nil {
			t.Fatalf("Set(%q, %q): %v", k, v, err)
		}
		got, err := c.Get(k)
		if err != nil {
			t.Fatalf("Get(%q): %v", k, err)
		}
		if got != v {
			t.Fatalf("Get(%q) = %q, want %q", k, got, v)
		}
	}

	// Set with invalid value rolls back: Set is all-or-nothing.
	prevOutput := c.Output
	if err := c.Set(KeyOutput, "yaml"); err == nil {
		t.Fatal("Set(output, yaml) should fail validation")
	}
	if c.Output != prevOutput {
		t.Fatalf("after failed Set, Output = %q, want %q (rollback)", c.Output, prevOutput)
	}

	// Unset clears the field.
	if err := c.Unset(KeyAppSlug); err != nil {
		t.Fatalf("Unset: %v", err)
	}
	if v, _ := c.Get(KeyAppSlug); v != "" {
		t.Fatalf("after Unset, AppSlug = %q, want empty", v)
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	want := Config{
		Output:     "json",
		AppSlug:    "my-app",
		APIBaseURL: "https://api.staging.example.com",
	}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", got, want)
	}

	// File should be 0600 (skip on Windows where modes don't match).
	if runtime.GOOS != "windows" {
		p := filepath.Join(dir, "bitrise", "config.yaml")
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("file perm = %o, want 0600", info.Mode().Perm())
		}
	}
}

func TestLoad_MissingFileIsZeroValue(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != (Config{}) {
		t.Fatalf("expected zero Config, got %+v", got)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "bitrise"), 0o700); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bitrise", "config.yaml")
	if err := os.WriteFile(bad, []byte("this: is :: not yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected error on malformed YAML")
	}
}

func TestLoad_RejectsInvalidContent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "bitrise"), 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "bitrise", "config.yaml")
	if err := os.WriteFile(p, []byte("output: yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load()
	if err == nil {
		t.Fatal("expected validation error on bad output")
	}
}

func TestLoadDir_FindsAncestorFile(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil { //nolint:gosec // test-only tempdir, perms don't matter
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "a", DirFileName)
	if err := os.WriteFile(cfgPath, []byte("app_slug: project-app\n"), 0o644); err != nil { //nolint:gosec // test-only tempfile
		t.Fatal(err)
	}

	got, found, err := loadDirFrom(deep)
	if err != nil {
		t.Fatalf("loadDirFrom: %v", err)
	}
	if found != cfgPath {
		t.Fatalf("found path = %q, want %q", found, cfgPath)
	}
	if got.AppSlug != "project-app" {
		t.Fatalf("AppSlug = %q, want %q", got.AppSlug, "project-app")
	}
}

func TestLoadDir_NoFileReturnsZero(t *testing.T) {
	got, found, err := loadDirFrom(t.TempDir())
	if err != nil {
		t.Fatalf("loadDirFrom: %v", err)
	}
	if found != "" {
		t.Fatalf("found = %q, want empty", found)
	}
	if got != (Config{}) {
		t.Fatalf("got %+v, want zero", got)
	}
}
