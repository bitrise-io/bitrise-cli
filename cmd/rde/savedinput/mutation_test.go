package savedinput

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func TestUpdateCmd_RequiresAField(t *testing.T) {
	_, _, err := run(t, newUpdateCmd(), "http://unused", []string{"sv-1"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "--value") {
		t.Errorf("error = %v, want at-least-one-field error", err)
	}
}

func TestUpdateCmd_ValueOnlyOmitsSecret(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/v1/saved-inputs/sv-1" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = io.WriteString(w, `{"savedInput":{"id":"sv-1","key":"repo","value":"new"}}`)
	}))
	defer srv.Close()

	_, _, err := run(t, newUpdateCmd(), srv.URL, []string{"sv-1", "--value", "new"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBody["value"] != "new" {
		t.Errorf("value = %v, want new", gotBody["value"])
	}
	// --secret wasn't passed, so isSecret must be omitted (not reset to false).
	if _, ok := gotBody["isSecret"]; ok {
		t.Errorf("isSecret should be omitted, body=%v", gotBody)
	}
}

func TestUpdateCmd_ValueStdin(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = io.WriteString(w, `{"savedInput":{"id":"sv-1","key":"repo","isSecret":true,"value":"***"}}`)
	}))
	defer srv.Close()

	_, _, err := runIn(t, newUpdateCmd(), srv.URL, "new-secret\n",
		[]string{"sv-1", "--value-stdin"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBody["value"] != "new-secret" {
		t.Errorf("value = %v, want new-secret (read from stdin)", gotBody["value"])
	}
}

func TestUpdateCmd_SecretFlagSendsBool(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = io.WriteString(w, `{"savedInput":{"id":"sv-1","key":"repo","isSecret":true,"value":"***"}}`)
	}))
	defer srv.Close()

	// --secret without --value: only the secret flag is patched.
	_, _, err := run(t, newUpdateCmd(), srv.URL, []string{"sv-1", "--secret"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBody["isSecret"] != true {
		t.Errorf("isSecret = %v, want true", gotBody["isSecret"])
	}
	if _, ok := gotBody["value"]; ok {
		t.Errorf("value should be omitted, body=%v", gotBody)
	}
}

func TestUpdateCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"savedInput":{"id":"sv-1","key":"repo","value":"new"}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newUpdateCmd(), srv.URL, []string{"sv-1", "--value", "new"}, output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	if got["id"] != "sv-1" || got["value"] != "new" {
		t.Errorf("unexpected JSON: %v", got)
	}
}

func TestUpdateCmd_RequiresArg(t *testing.T) {
	_, _, err := run(t, newUpdateCmd(), "http://unused", []string{"--value", "x"}, output.Human)
	if err == nil {
		t.Fatal("expected error when SAVED_INPUT_ID is missing")
	}
}

func TestDeleteCmd_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v1/saved-inputs/sv-1" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	stdout, stderr, err := run(t, newDeleteCmd(), srv.URL, []string{"sv-1"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stderr, "Deleted saved input sv-1") {
		t.Errorf("stderr missing confirmation: %q", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout should be empty for delete, got: %q", stdout)
	}
}

func TestDeleteCmd_RequiresArg(t *testing.T) {
	_, _, err := run(t, newDeleteCmd(), "http://unused", nil, output.Human)
	if err == nil {
		t.Fatal("expected error when SAVED_INPUT_ID is missing")
	}
}
