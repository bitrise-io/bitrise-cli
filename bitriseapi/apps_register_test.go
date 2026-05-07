package bitriseapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestRegisterApp(t *testing.T) {
	fs := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/register" {
			t.Errorf("path = %q, want /apps/register", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}

		var body RegisterAppRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.RepoURL != "https://github.com/x/y.git" || body.OrganizationSlug != "acme" || body.Provider != "github" {
			t.Errorf("body = %+v", body)
		}
		_, _ = io.WriteString(w, `{"status":"ok","slug":"new-app-slug"}`)
	})

	got, err := fs.client("t").RegisterApp(context.Background(), RegisterAppRequest{
		RepoURL:           "https://github.com/x/y.git",
		OrganizationSlug:  "acme",
		Provider:          "github",
		Title:             "Y",
		DefaultBranchName: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Slug != "new-app-slug" || got.Status != "ok" {
		t.Errorf("got %+v", got)
	}
}

func TestFinishApp(t *testing.T) {
	fs := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/abc/finish" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body FinishAppRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.StackID != "linux-docker-android-22.04" || body.Mode != "manual" || body.ProjectType != "other" {
			t.Errorf("body = %+v", body)
		}
		_, _ = io.WriteString(w, `{"status":"ok","build_trigger_token":"btt","branch_name":"main"}`)
	})

	got, err := fs.client("t").FinishApp(context.Background(), "abc", FinishAppRequest{
		StackID:     "linux-docker-android-22.04",
		Mode:        "manual",
		ProjectType: "other",
		Config:      "default",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.BuildTriggerToken != "btt" || got.BranchName != "main" {
		t.Errorf("got %+v", got)
	}
}

func TestUploadAppConfig(t *testing.T) {
	fs := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/abc/bitrise.yml" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body uploadAppConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.AppConfigDatastoreYAML != "format_version: 11\n" {
			t.Errorf("YAML body = %q", body.AppConfigDatastoreYAML)
		}
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})

	if err := fs.client("t").UploadAppConfig(context.Background(), "abc", "format_version: 11\n"); err != nil {
		t.Fatal(err)
	}
}

func TestOrganizations(t *testing.T) {
	fs := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/organizations" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"data":[{"slug":"acme","name":"Acme"},{"slug":"globex","name":"Globex"}]}`)
	})

	got, err := fs.client("t").Organizations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Slug != "acme" || got[1].Name != "Globex" {
		t.Errorf("got %+v", got)
	}
}
