package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bitrise-io/bitrise-cli/bitriseapi"
)

// stubAPI is the multiplexer used by Create's tests. It maps URL paths to
// canned responses and records the request bodies that arrived.
type stubAPI struct {
	t        *testing.T
	srv      *httptest.Server
	registry map[string]http.HandlerFunc
	bodies   map[string][]byte
}

func newStubAPI(t *testing.T) *stubAPI {
	t.Helper()
	s := &stubAPI{t: t, registry: map[string]http.HandlerFunc{}, bodies: map[string][]byte{}}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.bodies[r.URL.Path] = body
		h, ok := s.registry[r.URL.Path]
		if !ok {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		// Restore the body so handlers that want to decode see it.
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		h(w, r)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *stubAPI) handle(path string, h http.HandlerFunc) { s.registry[path] = h }

func (s *stubAPI) client() *bitriseapi.Client { return bitriseapi.New(s.srv.URL, "tok") }

func TestCreate_RegisterFinishUpload_WithExplicitOrg(t *testing.T) {
	api := newStubAPI(t)
	api.handle("/apps/register", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"status":"ok","slug":"new-app"}`)
	})
	api.handle("/apps/new-app/finish", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"status":"ok","build_trigger_token":"btt","branch_name":"main"}`)
	})
	api.handle("/apps/new-app/bitrise.yml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})

	svc := NewService(api.client())
	got, err := svc.Create(context.Background(), CreateOptions{
		RepoURL:    "https://github.com/acme/widget.git",
		Branch:     "main",
		Title:      "Widget",
		Provider:   "auto",
		OrgSlug:    "acme",
		BitriseYML: "format_version: 11\n",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Result fields.
	want := CreateResult{
		Slug: "new-app", Title: "Widget", RepoURL: "https://github.com/acme/widget.git",
		DefaultBranch: "main", BuildTriggerToken: "btt",
		OrgSlug: "acme", StackID: DefaultStackID, ProjectType: DefaultProjectType,
		BitriseYMLUploaded: true,
	}
	if got != want {
		t.Errorf("got %+v\nwant %+v", got, want)
	}

	// Provider defaults to "custom" — matches the website's add-new-app flow.
	var reg bitriseapi.RegisterAppRequest
	if err := json.Unmarshal(api.bodies["/apps/register"], &reg); err != nil {
		t.Fatal(err)
	}
	if reg.Provider != DefaultProvider || reg.OrganizationSlug != "acme" || reg.DefaultBranchName != "main" {
		t.Errorf("register body = %+v", reg)
	}
	if reg.FlowType != FlowTypeCLI {
		t.Errorf("FlowType = %q, want %q", reg.FlowType, FlowTypeCLI)
	}

	// Finish defaults applied; config maps to project_type.
	var fin bitriseapi.FinishAppRequest
	if err := json.Unmarshal(api.bodies["/apps/new-app/finish"], &fin); err != nil {
		t.Fatal(err)
	}
	if fin.StackID != DefaultStackID || fin.Mode != "manual" || fin.ProjectType != DefaultProjectType || fin.Config != "other-config" {
		t.Errorf("finish body = %+v", fin)
	}
	if fin.FlowType != FlowTypeCLI {
		t.Errorf("FlowType = %q, want %q", fin.FlowType, FlowTypeCLI)
	}

	// Upload happened with the YAML payload.
	if !strings.Contains(string(api.bodies["/apps/new-app/bitrise.yml"]), "format_version: 11") {
		t.Errorf("upload body = %q", api.bodies["/apps/new-app/bitrise.yml"])
	}
}

func TestCreate_SkipsUpload_WhenNoYML(t *testing.T) {
	api := newStubAPI(t)
	api.handle("/apps/register", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"status":"ok","slug":"x"}`)
	})
	api.handle("/apps/x/finish", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"status":"ok","build_trigger_token":"t","branch_name":"main"}`)
	})

	svc := NewService(api.client())
	got, err := svc.Create(context.Background(), CreateOptions{
		RepoURL: "https://github.com/a/b.git",
		OrgSlug: "acme",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.BitriseYMLUploaded {
		t.Error("expected BitriseYMLUploaded=false when no YAML supplied")
	}
}

func TestCreate_AutoDetectOrg_SingleOrgWins(t *testing.T) {
	api := newStubAPI(t)
	api.handle("/organizations", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":[{"slug":"only-org","name":"Only"}]}`)
	})
	api.handle("/apps/register", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"status":"ok","slug":"x"}`)
	})
	api.handle("/apps/x/finish", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"status":"ok","build_trigger_token":"t","branch_name":"main"}`)
	})

	svc := NewService(api.client())
	got, err := svc.Create(context.Background(), CreateOptions{
		RepoURL: "https://github.com/a/b.git",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.OrgSlug != "only-org" {
		t.Errorf("OrgSlug = %q, want only-org", got.OrgSlug)
	}
	var reg bitriseapi.RegisterAppRequest
	_ = json.Unmarshal(api.bodies["/apps/register"], &reg)
	if reg.OrganizationSlug != "only-org" {
		t.Errorf("register sent organization_slug=%q", reg.OrganizationSlug)
	}
}

func TestCreate_AutoDetectOrg_NoneFails(t *testing.T) {
	api := newStubAPI(t)
	api.handle("/organizations", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":[]}`)
	})
	svc := NewService(api.client())
	_, err := svc.Create(context.Background(), CreateOptions{RepoURL: "https://github.com/a/b.git"})
	if err == nil || !strings.Contains(err.Error(), "no workspaces") {
		t.Fatalf("expected no-workspaces error, got %v", err)
	}
}

func TestCreate_AutoDetectOrg_MultipleFails(t *testing.T) {
	api := newStubAPI(t)
	api.handle("/organizations", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":[{"slug":"a"},{"slug":"b"}]}`)
	})
	svc := NewService(api.client())
	_, err := svc.Create(context.Background(), CreateOptions{RepoURL: "https://github.com/a/b.git"})
	if err == nil || !strings.Contains(err.Error(), "multiple workspaces") {
		t.Fatalf("expected multiple-workspaces error, got %v", err)
	}
}

func TestCreate_RequiresRepoURL(t *testing.T) {
	svc := NewService(bitriseapi.New("http://unused", "t"))
	_, err := svc.Create(context.Background(), CreateOptions{OrgSlug: "acme"})
	if err == nil || !strings.Contains(err.Error(), "repo URL is required") {
		t.Fatalf("expected repo-url error, got %v", err)
	}
}

func TestCreate_NilClientFails(t *testing.T) {
	svc := NewService(nil)
	if _, err := svc.Create(context.Background(), CreateOptions{RepoURL: "x", OrgSlug: "y"}); err == nil {
		t.Fatal("expected error when client is nil")
	}
}

func TestResolveProvider_DefaultsToCustom(t *testing.T) {
	cases := map[string]string{
		"":     DefaultProvider,
		"auto": DefaultProvider,
	}
	for input, want := range cases {
		got, err := resolveProvider(input, "https://github.com/x/y.git")
		if err != nil || got != want {
			t.Errorf("resolveProvider(%q) = (%q, %v), want (%q, nil)", input, got, err, want)
		}
	}
}

func TestConfigIDForProjectType(t *testing.T) {
	// Pins the full project-type → config_id mapping. The expected values are
	// the server presets from bitrise-website's custom_config.yml (see the
	// doc comment on configIDForProjectType); any change here should be a
	// reviewed, intentional diff checked against that source.
	cases := map[string]string{
		"android":              "default-android-config",
		"cordova":              "default-cordova-config",
		"fastlane":             "default-fastlane-ios-config",
		"flutter":              "flutter-config-test-ios-android-web-0",
		"ionic":                "default-ionic-config",
		"ios":                  "default-ios-config",
		"java":                 "default-java-gradle-config",
		"kotlin-multiplatform": "default-kotlin-multiplatform-config",
		"macos":                "default-macos-config",
		"node-js":              "default-node-js-npm-config",
		"python":               "default-python-pip-config",
		"react-native":         "default-react-native-config",
		"ruby":                 "default-ruby-config",
		// Fallbacks: empty, unmapped, and unknown values all yield the
		// server's omitted-field default. "xamarin" is advertised by the
		// --project-type completion list but has no preset, so it falls
		// through here too — kept explicit so that gap stays visible.
		"":        "other-config",
		"other":   "other-config",
		"xamarin": "other-config",
		"unknown": "other-config",
	}
	for input, want := range cases {
		if got := configIDForProjectType(input); got != want {
			t.Errorf("configIDForProjectType(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestDeriveTitle(t *testing.T) {
	cases := map[string]string{
		"https://github.com/acme/widget.git": "widget",
		"https://github.com/acme/widget":     "widget",
		"git@github.com:acme/widget.git":     "widget",
		"":                                   "",
	}
	for url, want := range cases {
		if got := deriveTitle(url); got != want {
			t.Errorf("deriveTitle(%q) = %q, want %q", url, got, want)
		}
	}
}

func TestResolveProvider_RejectsUnknown(t *testing.T) {
	if _, err := resolveProvider("hg", "https://example.com/x/y"); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
