package yml

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

func TestValidateCmd_Valid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/validate-bitrise-yml" || r.Method != http.MethodPost {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"errors":[],"warnings":[]}`)
	}))
	defer srv.Close()

	c := newValidateCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetIn(strings.NewReader(sampleYML))
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "valid") {
		t.Errorf("stdout missing 'valid': %q", stdout.String())
	}
}

func TestValidateCmd_Invalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"errors":["workflow 'primary' not found"],"warnings":[]}`)
	}))
	defer srv.Close()

	c := newValidateCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetIn(strings.NewReader(sampleYML))
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
	}))

	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected 'invalid' error, got %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "invalid") {
		t.Errorf("stdout missing 'invalid': %q", out)
	}
	if !strings.Contains(out, "workflow 'primary' not found") {
		t.Errorf("stdout missing error detail: %q", out)
	}
}

func TestValidateCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"errors":[],"warnings":["unused step"]}`)
	}))
	defer srv.Close()

	c := newValidateCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetIn(strings.NewReader(sampleYML))
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.JSON,
	}))

	if err := c.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if got["valid"] != true {
		t.Errorf("expected valid=true, got %v", got["valid"])
	}
	warnings, _ := got["warnings"].([]any)
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warnings))
	}
}

func TestValidateCmd_422TreatedAsInvalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"message":"could not parse YAML"}`)
	}))
	defer srv.Close()

	c := newValidateCmd()
	stdout := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(io.Discard)
	c.SetIn(strings.NewReader("not: yaml: at: all:"))
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		APIBaseURL: srv.URL,
		Token:      "tok",
		Output:     output.Human,
	}))

	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected invalid error for 422, got %v", err)
	}
	if !strings.Contains(stdout.String(), "invalid") {
		t.Errorf("stdout missing 'invalid': %q", stdout.String())
	}
}
