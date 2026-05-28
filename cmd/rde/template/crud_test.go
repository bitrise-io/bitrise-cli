package template

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func TestCreateCmd_HappyPath(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/workspaces/ws-1/templates" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = io.WriteString(w, `{"template":{"id":"t-new","name":"Dev","image":"ubuntu","machineType":"standard"}}`)
	}))
	defer srv.Close()

	c := newCreateCmd()
	c.SetIn(strings.NewReader(`{"name":"Dev","image":"ubuntu","machine_type":"standard","session_inputs":[{"key":"repo","required":true}]}`))
	stdout, _, err := run(t, c, srv.URL, "ws-1", []string{"--file", "-"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// The spec's snake_case fields must map onto the camelCase wire body.
	if gotBody["name"] != "Dev" || gotBody["image"] != "ubuntu" || gotBody["machineType"] != "standard" {
		t.Errorf("unexpected create body: %v", gotBody)
	}
	if !strings.Contains(stdout, "t-new") {
		t.Errorf("stdout missing new template ID:\n%s", stdout)
	}
}

func TestCreateCmd_RequiresFile(t *testing.T) {
	_, _, err := run(t, newCreateCmd(), "http://unused", "ws-1", nil, output.Human)
	if err == nil || !strings.Contains(err.Error(), "--file") {
		t.Errorf("error = %v, want --file required", err)
	}
}

func TestCreateCmd_MissingRequiredSpecField(t *testing.T) {
	// machine_type is required by the service; the spec omits it, so the
	// command must fail before any HTTP call.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("server should not be hit when the spec is invalid")
	}))
	defer srv.Close()

	c := newCreateCmd()
	c.SetIn(strings.NewReader(`{"name":"Dev","image":"ubuntu"}`))
	_, _, err := run(t, c, srv.URL, "ws-1", []string{"--file", "-"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "machine_type") {
		t.Errorf("error = %v, want machine_type-required", err)
	}
}

func TestCreateCmd_MalformedJSON(t *testing.T) {
	c := newCreateCmd()
	c.SetIn(strings.NewReader(`{not json`))
	_, _, err := run(t, c, "http://unused", "ws-1", []string{"--file", "-"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "parse template spec") {
		t.Errorf("error = %v, want parse error", err)
	}
}

func TestUpdateCmd_SendsReplaceFlagForArrays(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/v1/workspaces/ws-1/templates/t-1" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = io.WriteString(w, `{"template":{"id":"t-1","name":"Renamed"}}`)
	}))
	defer srv.Close()

	c := newUpdateCmd()
	c.SetIn(strings.NewReader(`{"name":"Renamed","session_inputs":[{"key":"repo"}]}`))
	_, _, err := run(t, c, srv.URL, "ws-1", []string{"t-1", "--file", "-"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBody["name"] != "Renamed" {
		t.Errorf("name = %v, want Renamed", gotBody["name"])
	}
	// A present array field must carry its updateXxx replace flag so the
	// server replaces (not merges) the list.
	if gotBody["updateSessionInputs"] != true {
		t.Errorf("updateSessionInputs = %v, want true (body=%v)", gotBody["updateSessionInputs"], gotBody)
	}
	// An absent array field must not trigger its flag.
	if _, ok := gotBody["updateFeatureFlags"]; ok {
		t.Errorf("updateFeatureFlags should be absent, body=%v", gotBody)
	}
}

func TestUpdateCmd_RequiresFile(t *testing.T) {
	_, _, err := run(t, newUpdateCmd(), "http://unused", "ws-1", []string{"t-1"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "--file") {
		t.Errorf("error = %v, want --file required", err)
	}
}

func TestUpdateCmd_RequiresArg(t *testing.T) {
	_, _, err := run(t, newUpdateCmd(), "http://unused", "ws-1", []string{"--file", "-"}, output.Human)
	if err == nil {
		t.Fatal("expected error when TEMPLATE_ID is missing")
	}
}

func TestDeleteCmd_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v1/workspaces/ws-1/templates/t-1" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	stdout, stderr, err := run(t, newDeleteCmd(), srv.URL, "ws-1", []string{"t-1"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// Confirmation goes to stderr, never stdout.
	if !strings.Contains(stderr, "Deleted template t-1") {
		t.Errorf("stderr missing confirmation: %q", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout should be empty for delete, got: %q", stdout)
	}
}

func TestDeleteCmd_RequiresArg(t *testing.T) {
	_, _, err := run(t, newDeleteCmd(), "http://unused", "ws-1", nil, output.Human)
	if err == nil {
		t.Fatal("expected error when TEMPLATE_ID is missing")
	}
}
