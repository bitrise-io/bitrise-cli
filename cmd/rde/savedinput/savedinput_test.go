package savedinput

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

// run drives c against a Resolved context pointing at srvURL. Saved inputs are
// user-scoped, so no workspace ID is required.
func run(t *testing.T, c *cobra.Command, srvURL string, args []string, format output.Format) (string, string, error) {
	t.Helper()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(stderr)
	c.SetArgs(args)
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		RDEAPIBaseURL: srvURL,
		Token:         "tok",
		Output:        format,
	}))
	err := c.Execute()
	return stdout.String(), stderr.String(), err
}

// runIn is run with a fixed stdin, for exercising --value-stdin and the
// interactive-prompt fallback. The injected reader is not a terminal, so
// ReadSecretInput reads it as a plain line rather than a masked prompt.
func runIn(t *testing.T, c *cobra.Command, srvURL, stdin string, args []string, format output.Format) (string, string, error) {
	t.Helper()
	c.SetIn(strings.NewReader(stdin))
	return run(t, c, srvURL, args, format)
}

func TestListCmd_HappyPath_MasksSecrets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/saved-inputs" {
			t.Errorf("unexpected path: %s (saved inputs are user-scoped)", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"savedInputs":[
			{"id":"sv-1","key":"repo","value":"my-app"},
			{"id":"sv-2","key":"gh-token","isSecret":true,"value":"ghp_LEAK"}
		]}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newListCmd(), srv.URL, nil, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"repo", "my-app", "gh-token", "(hidden)", "sv-1", "sv-2"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
	// The masked secret's plaintext must never reach human output.
	if strings.Contains(stdout, "ghp_LEAK") {
		t.Errorf("secret value leaked into human output:\n%s", stdout)
	}
}

func TestListCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"savedInputs":[{"id":"sv-1","key":"repo","value":"my-app"}]}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newListCmd(), srv.URL, nil, output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got struct {
		Items []struct {
			ID    string `json:"id"`
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	if len(got.Items) != 1 || got.Items[0].Key != "repo" || got.Items[0].Value != "my-app" {
		t.Errorf("unexpected JSON items: %+v", got.Items)
	}
}

// TestJSONOutput_MasksSecrets is the regression guard for the secret leak:
// the backend returns secret values in cleartext (and echoes the
// just-submitted value back on create/update), so the CLI must blank them
// before --output json marshals the record. Covers both list and view.
func TestJSONOutput_MasksSecrets(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"savedInputs":[
				{"id":"sv-1","key":"repo","value":"my-app"},
				{"id":"sv-2","key":"gh-token","isSecret":true,"value":"ghp_LEAK"}
			]}`)
		}))
		defer srv.Close()

		stdout, _, err := run(t, newListCmd(), srv.URL, nil, output.JSON)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if strings.Contains(stdout, "ghp_LEAK") {
			t.Errorf("secret value leaked into JSON output:\n%s", stdout)
		}
	})

	t.Run("view", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"savedInput":{"id":"sv-2","key":"gh-token","isSecret":true,"value":"ghp_LEAK"}}`)
		}))
		defer srv.Close()

		stdout, _, err := run(t, newViewCmd(), srv.URL, []string{"sv-2"}, output.JSON)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if strings.Contains(stdout, "ghp_LEAK") {
			t.Errorf("secret value leaked into JSON output:\n%s", stdout)
		}
	})
}

func TestViewCmd_SecretHuman(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/saved-inputs/sv-2" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"savedInput":{"id":"sv-2","key":"gh-token","isSecret":true,"value":"***"}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, []string{"sv-2"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"gh-token", "sv-2", "(hidden)", "yes"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestCreateCmd_RequiresKey(t *testing.T) {
	_, _, err := run(t, newCreateCmd(), "http://unused", []string{"--value", "x"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "--key") {
		t.Errorf("error = %v, want --key required", err)
	}
}

func TestCreateCmd_EmptyValueRejected(t *testing.T) {
	// Neither --value nor --value-stdin, and empty stdin (the prompt fallback
	// reads it as a line): there is nothing to store, so it must error rather
	// than create an empty value.
	_, _, err := runIn(t, newCreateCmd(), "http://unused", "", []string{"--key", "repo"}, output.Human)
	if err == nil || !strings.Contains(err.Error(), "value is empty") {
		t.Errorf("error = %v, want value-is-empty", err)
	}
}

func TestCreateCmd_ValueStdin(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = io.WriteString(w, `{"savedInput":{"id":"sv-new","key":"gh-token","isSecret":true,"value":"***"}}`)
	}))
	defer srv.Close()

	_, _, err := runIn(t, newCreateCmd(), srv.URL, "ghp_secret\n",
		[]string{"--key", "gh-token", "--value-stdin", "--secret"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if body["value"] != "ghp_secret" {
		t.Errorf("value = %v, want ghp_secret (read from stdin, trailing newline trimmed)", body["value"])
	}
}

// TestCreateCmd_LiteralDashValue guards the original question that started this:
// with the "-" sentinel removed, --value - now stores a literal dash.
func TestCreateCmd_LiteralDashValue(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = io.WriteString(w, `{"savedInput":{"id":"sv-new","key":"dash","value":"-"}}`)
	}))
	defer srv.Close()

	_, _, err := run(t, newCreateCmd(), srv.URL, []string{"--key", "dash", "--value", "-"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if body["value"] != "-" {
		t.Errorf("value = %v, want literal -", body["value"])
	}
}

func TestCreateCmd_ValueAndStdinMutuallyExclusive(t *testing.T) {
	_, _, err := run(t, newCreateCmd(), "http://unused",
		[]string{"--key", "repo", "--value", "x", "--value-stdin"}, output.Human)
	if err == nil {
		t.Fatal("expected error when --value and --value-stdin are both set")
	}
}

func TestCreateCmd_HappyPathJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/saved-inputs" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["key"] != "gh-token" || body["isSecret"] != true {
			t.Errorf("unexpected create body: %v", body)
		}
		_, _ = io.WriteString(w, `{"savedInput":{"id":"sv-new","key":"gh-token","isSecret":true,"value":"***"}}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newCreateCmd(), srv.URL,
		[]string{"--key", "gh-token", "--value", "ghp_x", "--secret"}, output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	if got["id"] != "sv-new" || got["is_secret"] != true {
		t.Errorf("unexpected JSON: %v", got)
	}
}
