package bitriseapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuilds_PathAndQueryParams(t *testing.T) {
	fs := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[],"paging":{"next":""}}`))
	})

	successFilter := 1
	pipelineFilter := true
	_, err := fs.client("t").Builds(context.Background(), "my-app", BuildsListOptions{
		Branch:          "main",
		Workflow:        "deploy",
		Status:          &successFilter,
		IsPipelineBuild: &pipelineFilter,
		Limit:           20,
		Next:            "cur",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := fs.lastReq.URL.Path; got != "/apps/my-app/builds" {
		t.Errorf("path = %q", got)
	}
	q := fs.lastReq.URL.Query()
	checks := map[string]string{
		"branch":            "main",
		"workflow":          "deploy",
		"status":            "1",
		"is_pipeline_build": "true",
		"limit":             "20",
		"next":              "cur",
	}
	for k, want := range checks {
		if got := q.Get(k); got != want {
			t.Errorf("query %q = %q, want %q", k, got, want)
		}
	}
}

func TestBuilds_StatusZeroIsValidFilter(t *testing.T) {
	// Status=0 means "in-progress" filter. With *int we can encode it
	// distinctly from "no filter" (nil pointer).
	fs := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	zero := 0
	_, err := fs.client("t").Builds(context.Background(), "my-app", BuildsListOptions{Status: &zero})
	if err != nil {
		t.Fatal(err)
	}
	if got := fs.lastReq.URL.Query().Get("status"); got != "0" {
		t.Errorf("status = %q, want 0", got)
	}
}

func TestBuilds_NoStatusOmitsParam(t *testing.T) {
	fs := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	_, err := fs.client("t").Builds(context.Background(), "my-app", BuildsListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if fs.lastReq.URL.Query().Has("status") {
		t.Error("status query param should be omitted when filter is nil")
	}
}

func TestBuilds_ParsesResponse(t *testing.T) {
	fs := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
  "data": [
    {
      "slug": "build-1",
      "build_number": 42,
      "status": 1,
      "status_text": "success",
      "branch": "main",
      "triggered_workflow": "primary",
      "commit_hash": "deadbeef",
      "triggered_at": "2026-05-06T10:00:00Z",
      "finished_at": "2026-05-06T10:05:00Z"
    }
  ],
  "paging": {"next": "cursor-abc"}
}`))
	})
	page, err := fs.client("t").Builds(context.Background(), "my-app", BuildsListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page.Items))
	}
	b := page.Items[0]
	if b.Slug != "build-1" || b.BuildNumber != 42 || b.Status != 1 {
		t.Errorf("got %+v", b)
	}
	if b.TriggeredAt.IsZero() || b.FinishedAt.IsZero() {
		t.Errorf("timestamps should be parsed: triggered=%v finished=%v", b.TriggeredAt, b.FinishedAt)
	}
	if page.Paging.Next != "cursor-abc" {
		t.Errorf("Paging.Next = %q", page.Paging.Next)
	}
}

func TestBuild_Single(t *testing.T) {
	fs := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/my-app/builds/build-xyz" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"slug":"build-xyz","build_number":7,"status":2,"status_text":"failed","triggered_workflow":"deploy"}}`))
	})

	b, err := fs.client("t").Build(context.Background(), "my-app", "build-xyz")
	if err != nil {
		t.Fatal(err)
	}
	if b.Slug != "build-xyz" || b.BuildNumber != 7 || b.Status != 2 || b.TriggeredWorkflow != "deploy" {
		t.Errorf("got %+v", b)
	}
}

func TestTriggerBuild_RequestAndResponse(t *testing.T) {
	var gotBody []byte
	fs := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/apps/my-app/builds" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
  "build_slug": "new-build-001",
  "build_number": 99,
  "build_url": "https://app.bitrise.io/build/new-build-001",
  "triggered_workflow": "deploy",
  "status": "ok",
  "message": "Build triggered"
}`))
	})

	resp, err := fs.client("t").TriggerBuild(context.Background(), "my-app", BuildTriggerParams{
		HookInfo: BuildTriggerHookInfo{Type: "bitrise"},
		BuildParams: BuildTriggerBuildParams{
			WorkflowID: "deploy",
			Branch:     "release/2.0",
			CommitHash: "abc123",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify request body shape.
	var sent map[string]any
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("request body invalid JSON: %v\n%s", err, gotBody)
	}
	hi, _ := sent["hook_info"].(map[string]any)
	if hi["type"] != "bitrise" {
		t.Errorf("hook_info.type = %v, want bitrise", hi["type"])
	}
	bp, _ := sent["build_params"].(map[string]any)
	if bp["workflow_id"] != "deploy" || bp["branch"] != "release/2.0" || bp["commit_hash"] != "abc123" {
		t.Errorf("build_params = %v", bp)
	}

	// Verify response decoding.
	if resp.BuildSlug != "new-build-001" || resp.BuildNumber != 99 || resp.BuildURL == "" {
		t.Errorf("got %+v", resp)
	}
}

func TestTriggerBuild_PropagatesAPIError(t *testing.T) {
	fs := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"workflow_id is required"}`))
	})

	_, err := fs.client("t").TriggerBuild(context.Background(), "my-app", BuildTriggerParams{})
	if err == nil {
		t.Fatal("expected error on 400")
	}
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("got %v", err)
	}
}

func TestBuildLog_StreamsArchivedURL(t *testing.T) {
	// Two stages: first the API returns a manifest pointing at a raw URL;
	// then we fetch that URL and the body should be streamed to the writer.
	rawSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should NOT receive our auth header on the presigned URL.
		if r.Header.Get("Authorization") != "" {
			t.Errorf("raw log URL got Authorization header: %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte("FULL ARCHIVED LOG CONTENT\n"))
	}))
	t.Cleanup(rawSrv.Close)

	apiSrv := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		body := map[string]any{
			"is_archived":              true,
			"expiring_raw_log_url":     rawSrv.URL,
			"generated_log_chunks_num": 0,
		}
		_ = json.NewEncoder(w).Encode(body)
	})

	var buf bytes.Buffer
	manifest, err := apiSrv.client("t").BuildLog(context.Background(), "my-app", "my-build", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if !manifest.IsArchived {
		t.Error("manifest.IsArchived should be true")
	}
	if !strings.Contains(buf.String(), "FULL ARCHIVED LOG CONTENT") {
		t.Errorf("expected archived log content in output, got %q", buf.String())
	}
}

func TestBuildLog_StreamsChunksWhenNotArchived(t *testing.T) {
	apiSrv := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
  "is_archived": false,
  "log_chunks": [
    {"chunk": "first chunk\n", "position": 0},
    {"chunk": "second chunk\n", "position": 1}
  ]
}`))
	})

	var buf bytes.Buffer
	manifest, err := apiSrv.client("t").BuildLog(context.Background(), "my-app", "my-build", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.IsArchived {
		t.Error("manifest.IsArchived should be false")
	}
	got := buf.String()
	if !strings.Contains(got, "first chunk") || !strings.Contains(got, "second chunk") {
		t.Errorf("expected both chunks in output, got %q", got)
	}
}

func TestBuildLog_PropagatesManifestError(t *testing.T) {
	apiSrv := newFakeServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"build not found"}`))
	})

	var buf bytes.Buffer
	_, err := apiSrv.client("t").BuildLog(context.Background(), "my-app", "missing", &buf)
	if err == nil {
		t.Fatal("expected error")
	}
	if buf.Len() != 0 {
		t.Errorf("buffer should be untouched on error, got %q", buf.String())
	}
}
