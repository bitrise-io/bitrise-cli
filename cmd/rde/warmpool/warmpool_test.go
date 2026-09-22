package warmpool

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

// uuidPool is a UUID-shaped warm pool arg. Real pool IDs are UUIDs, so
// passing one exercises the ResolveWarmPoolID short-circuit (no extra
// ListWarmPools call) — the path production hits when a user pastes an ID.
const uuidPool = "33333333-4444-4444-8444-555555555555"

// uuidTemplate is a UUID-shaped template arg so ResolveTemplateID
// short-circuits without an extra ListTemplates call.
const uuidTemplate = "11111111-2222-3333-4444-555555555555"

// run drives c against a Resolved context pointing at srvURL, with
// workspaceID seeded so commands resolve it without a --workspace flag.
func run(t *testing.T, c *cobra.Command, srvURL, workspaceID string, args []string, format output.Format) (string, string, error) {
	t.Helper()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(stderr)
	c.SetArgs(args)
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		RDEAPIBaseURL: srvURL,
		Token:         "tok",
		Output:        format,
		WorkspaceID:   workspaceID,
	}))
	err := c.Execute()
	return stdout.String(), stderr.String(), err
}

const listBody = `{"warmPools":[
	{"id":"p-1","name":"ios-devs","templateId":"t-1","templateName":"iOS Dev","ownerType":"workspace","ownerId":"my-ws","poolSize":2,
	 "status":{"ready":1,"warming":1,"claimedTotal":12,"coldTotal":3}},
	{"id":"p-2","name":"mine","templateId":"t-1","templateName":"iOS Dev","ownerType":"user","createdByEmail":"a@b.io","poolSize":0,
	 "status":{"configError":"stack retired"}}
]}`

func TestListCmd_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workspaces/ws-1/warm-pools" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.RawQuery != "" {
			t.Errorf("unexpected query for the default listing: %q", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, listBody)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newListCmd(), srv.URL, "ws-1", nil, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"NAME", "DESIRED", "READY", "STATUS", "ios-devs", "iOS Dev", "workspace", "a@b.io", "config error", "p-1", "p-2"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

// --template narrows by template (a UUID passes through unresolved) and
// --all switches to the every-pool scope; both ride in the query string.
func TestListCmd_TemplateFilterAndAll(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"warmPools":[]}`)
	}))
	defer srv.Close()

	if _, _, err := run(t, newListCmd(), srv.URL, "ws-1", []string{"--template", uuidTemplate, "--all"}, output.Human); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(gotQuery, "templateId="+uuidTemplate) || !strings.Contains(gotQuery, "scope=WARM_POOL_LIST_SCOPE_ALL") {
		t.Errorf("query = %q", gotQuery)
	}
}

// A template NAME is resolved through ListTemplates before the pools are
// listed, so the filter reaches the backend as an ID.
func TestListCmd_ResolvesTemplateName(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/workspaces/ws-1/templates":
			_, _ = io.WriteString(w, `{"templates":[{"id":"t-9","name":"iOS Dev"}]}`)
		case "/v1/workspaces/ws-1/warm-pools":
			gotQuery = r.URL.RawQuery
			_, _ = io.WriteString(w, `{"warmPools":[]}`)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	if _, _, err := run(t, newListCmd(), srv.URL, "ws-1", []string{"--template", "iOS Dev"}, output.Human); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotQuery != "templateId=t-9" {
		t.Errorf("query = %q, want the resolved template id", gotQuery)
	}
}

func TestListCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, listBody)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newListCmd(), srv.URL, "ws-1", nil, output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got struct {
		Items []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			PoolSize  int    `json:"pool_size"`
			OwnerType string `json:"owner_type"`
			Status    struct {
				Ready        int    `json:"ready"`
				ClaimedTotal int    `json:"claimed_total"`
				ConfigError  string `json:"config_error"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	if len(got.Items) != 2 || got.Items[0].ID != "p-1" || got.Items[0].PoolSize != 2 || got.Items[0].Status.Ready != 1 || got.Items[0].Status.ClaimedTotal != 12 {
		t.Errorf("unexpected JSON items: %+v", got.Items)
	}
	if got.Items[1].OwnerType != "user" || got.Items[1].Status.ConfigError != "stack retired" {
		t.Errorf("unexpected second item: %+v", got.Items[1])
	}
}

func TestListCmd_EmptyHuman(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"warmPools":[]}`)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newListCmd(), srv.URL, "ws-1", nil, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "No warm pools found.") {
		t.Errorf("expected empty-state message, got: %q", stdout)
	}
}

func TestListCmd_MissingWorkspace(t *testing.T) {
	c := newListCmd()
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	c.SetContext(config.WithResolved(context.Background(), config.Resolved{
		RDEAPIBaseURL: "http://unused",
		Token:         "tok",
		Output:        output.Human,
	}))
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "workspace") {
		t.Errorf("error = %v, want it to mention workspace", err)
	}
}

const viewBody = `{"warmPool":{
	"id":"p-1","name":"ios-devs","templateId":"t-1","templateName":"iOS Dev","ownerType":"workspace","ownerId":"my-ws","poolSize":2,
	"machineType":"g2.mac.m2pro.6c-14g",
	"sessionInputs":[{"key":"GITHUB_TOKEN","value":"leaked","isSecret":true},{"key":"REPO","value":"my-app"},{"key":"SSH_KEY","savedInputId":"si-1"}],
	"enabledFeatureFlagNames":["enable_beta_simulator"],
	"deviceSpec":{"platform":"ios","deviceModel":"iPhone 16","osVersion":"18.2"},
	"status":{"ready":1,"warming":1,"claimedTotal":12,"coldTotal":3,"lastError":"quota exceeded",
	  "sessions":[{"sessionId":"s-ready","state":"ready","createdAt":"2026-09-17T08:00:00Z","readyAt":"2026-09-17T08:04:00Z"},
	              {"sessionId":"s-warm","state":"warming","createdAt":"2026-09-17T08:10:00Z"}]},
	"createdAt":"2026-09-01T00:00:00Z"}}`

// The detail view shows the status, the configuration with the secret
// hidden and the saved input referenced by ID, and the inventory table.
func TestViewCmd_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workspaces/ws-1/warm-pools/"+uuidPool {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, viewBody)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidPool}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{
		"ios-devs", "p-1", "iOS Dev", "t-1", "workspace",
		"Pool size:", "2", "1 ready, 1 warming", "12 claimed, 3 cold", "quota exceeded",
		"g2.mac.m2pro.6c-14g", "iOS simulator · iPhone 16 · 18.2",
		"GITHUB_TOKEN", "(hidden)", "REPO", "my-app", "SSH_KEY", "saved input", "si-1",
		"enable_beta_simulator",
		"Warm sessions", "SESSION_ID", "s-ready", "ready", "s-warm", "warming", "2026-09-17 08:04 UTC",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "leaked") {
		t.Errorf("stdout leaks the secret value:\n%s", stdout)
	}
}

func TestViewCmd_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, viewBody)
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{uuidPool}, output.JSON)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, stdout)
	}
	if got["id"] != "p-1" || got["machine_type"] != "g2.mac.m2pro.6c-14g" || got["pool_size"] != float64(2) {
		t.Errorf("unexpected JSON: %v", got)
	}
	status, _ := got["status"].(map[string]any)
	sessions, _ := status["sessions"].([]any)
	if status["ready"] != float64(1) || len(sessions) != 2 {
		t.Errorf("unexpected status in JSON: %v", got["status"])
	}
	if strings.Contains(stdout, "leaked") {
		t.Errorf("JSON leaks the secret value:\n%s", stdout)
	}
}

// A pool NAME resolves through the visible listing to its ID.
func TestViewCmd_ResolvesName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/workspaces/ws-1/warm-pools":
			_, _ = io.WriteString(w, `{"warmPools":[{"id":"p-9","name":"ios-devs"},{"id":"p-7","name":"other"}]}`)
		case "/v1/workspaces/ws-1/warm-pools/p-9":
			_, _ = io.WriteString(w, `{"warmPool":{"id":"p-9","name":"ios-devs","poolSize":1}}`)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	stdout, _, err := run(t, newViewCmd(), srv.URL, "ws-1", []string{"ios-devs"}, output.Human)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout, "p-9") {
		t.Errorf("stdout missing the resolved pool:\n%s", stdout)
	}
}

func TestViewCmd_RequiresArg(t *testing.T) {
	_, _, err := run(t, newViewCmd(), "http://unused", "ws-1", nil, output.Human)
	if err == nil {
		t.Fatal("expected error when WARM_POOL_ID is missing")
	}
}
