package build

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bitrise-io/bitrise-cli/bitriseapi"
)

func fakeAPI(t *testing.T, handler http.HandlerFunc) *bitriseapi.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return bitriseapi.New(srv.URL, "test-token")
}

func TestService_List_PathAndMapping(t *testing.T) {
	var gotQuery string
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/my-app/builds" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{
  "data": [
    {"slug":"b-1","build_number":42,"status":1,"status_text":"success","branch":"main","triggered_workflow":"primary",
     "triggered_at":"2026-05-06T10:00:00Z","finished_at":"2026-05-06T10:05:00Z"},
    {"slug":"b-2","build_number":41,"status":0,"branch":"feature","triggered_workflow":"primary","triggered_at":"2026-05-06T09:00:00Z"}
  ],
  "paging": {"next": "next-cur"}
}`))
	})
	svc := NewService(client)

	res, err := svc.List(context.Background(), ListOptions{
		AppSlug:  "my-app",
		Branch:   "main",
		Workflow: "primary",
		Status:   "success",
		Limit:    50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("got %d items, want 2", len(res.Items))
	}
	if res.Items[0].Status != "success" || res.Items[1].Status != "in-progress" {
		t.Errorf("status mapping: %+v / %+v", res.Items[0], res.Items[1])
	}
	if res.Items[0].FinishedAt == nil || res.Items[1].FinishedAt != nil {
		t.Errorf("finished_at: item0=%v item1=%v", res.Items[0].FinishedAt, res.Items[1].FinishedAt)
	}
	if res.Items[0].AppSlug != "my-app" {
		t.Errorf("AppSlug should be propagated from request, got %q", res.Items[0].AppSlug)
	}
	if res.NextCursor != "next-cur" {
		t.Errorf("NextCursor = %q", res.NextCursor)
	}
	// Verify status string was translated to int 1 in the query.
	if !strings.Contains(gotQuery, "status=1") {
		t.Errorf("expected status=1 in query, got %q", gotQuery)
	}
}

func TestService_List_StatusInProgressMapsToZero(t *testing.T) {
	var gotQuery string
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	svc := NewService(client)

	_, err := svc.List(context.Background(), ListOptions{AppSlug: "my-app", Status: "in-progress"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "status=0") {
		t.Errorf("expected status=0 in query for in-progress, got %q", gotQuery)
	}
}

func TestService_List_NewFiltersPassedThrough(t *testing.T) {
	var gotQuery string
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	svc := NewService(client)

	after := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	before := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	isPipeline := true

	_, err := svc.List(context.Background(), ListOptions{
		AppSlug:          "my-app",
		SortBy:           "running_first",
		CommitMessage:    "fix bug",
		TriggerEventType: "push",
		PullRequestID:    42,
		BuildNumber:      99,
		After:            &after,
		Before:           &before,
		IsPipelineBuild:  &isPipeline,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"sort_by=running_first",
		"commit_message=fix+bug",
		"trigger_event_type=push",
		"pull_request_id=42",
		"build_number=99",
		"after=" + fmt.Sprintf("%d", after.Unix()),
		"before=" + fmt.Sprintf("%d", before.Unix()),
		"is_pipeline_build=true",
	} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("expected %q in query, got %q", want, gotQuery)
		}
	}
}

func TestService_List_RejectsUnknownStatus(t *testing.T) {
	svc := NewService(fakeAPI(t, func(http.ResponseWriter, *http.Request) {}))
	_, err := svc.List(context.Background(), ListOptions{AppSlug: "my-app", Status: "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown status")
	}
	if !strings.Contains(err.Error(), "unknown build status") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestService_View_PathAndMapping(t *testing.T) {
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/my-app/builds/b-xyz" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"slug":"b-xyz","build_number":7,"status":2,"status_text":"failed","triggered_workflow":"deploy","branch":"feature/x"}}`))
	})
	svc := NewService(client)

	got, err := svc.View(context.Background(), "my-app", "b-xyz")
	if err != nil {
		t.Fatal(err)
	}
	if got.Slug != "b-xyz" || got.Status != "failed" || got.Workflow != "deploy" || got.AppSlug != "my-app" {
		t.Errorf("got %+v", got)
	}
}

func TestService_Trigger_BodyAndResponse(t *testing.T) {
	var gotBody []byte
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q", r.Method)
		}
		if r.URL.Path != "/apps/my-app/builds" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
  "build_slug": "new-1",
  "build_number": 100,
  "build_url": "https://app.bitrise.io/build/new-1",
  "triggered_workflow": "deploy",
  "message": "Build triggered"
}`))
	})
	svc := NewService(client)

	got, err := svc.Trigger(context.Background(), TriggerRequest{
		AppSlug:    "my-app",
		Workflow:   "deploy",
		Branch:     "main",
		CommitHash: "abc",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify wire shape.
	var sent map[string]any
	_ = json.Unmarshal(gotBody, &sent)
	hi, _ := sent["hook_info"].(map[string]any)
	if hi["type"] != "bitrise" {
		t.Errorf("hook_info.type = %v", hi["type"])
	}
	bp, _ := sent["build_params"].(map[string]any)
	if bp["workflow_id"] != "deploy" || bp["branch"] != "main" || bp["commit_hash"] != "abc" {
		t.Errorf("build_params = %v", bp)
	}

	// Verify response → Build.
	if got.Slug != "new-1" || got.BuildNumber != 100 || got.BuildURL == "" {
		t.Errorf("got %+v", got)
	}
	if got.Status != "in-progress" {
		t.Errorf("triggered build should report in-progress status, got %q", got.Status)
	}
	if got.AppSlug != "my-app" || got.Workflow != "deploy" || got.Branch != "main" {
		t.Errorf("missing request-derived fields in result: %+v", got)
	}
}

func TestService_Trigger_PrefersResultsArray(t *testing.T) {
	client := fakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		// Modern API returns "results"; deprecated top-level fields differ.
		_, _ = w.Write([]byte(`{
  "build_slug": "deprecated-slug",
  "build_url": "deprecated-url",
  "results": [
    {"build_slug": "result-slug", "build_number": 99, "build_url": "result-url", "triggered_workflow": "deploy"}
  ]
}`))
	})
	svc := NewService(client)

	got, err := svc.Trigger(context.Background(), TriggerRequest{AppSlug: "a", Workflow: "deploy"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Slug != "result-slug" || got.BuildURL != "result-url" || got.BuildNumber != 99 {
		t.Errorf("expected results[0] to win, got %+v", got)
	}
}

func TestService_Trigger_RequiresAppAndWorkflow(t *testing.T) {
	svc := NewService(fakeAPI(t, func(http.ResponseWriter, *http.Request) {}))
	if _, err := svc.Trigger(context.Background(), TriggerRequest{Workflow: "x"}); err == nil {
		t.Fatal("missing app slug should fail")
	}
	if _, err := svc.Trigger(context.Background(), TriggerRequest{AppSlug: "x"}); err == nil {
		t.Fatal("missing workflow should fail")
	}
}

func TestService_Log_Streams(t *testing.T) {
	rawSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("LOG OUTPUT"))
	}))
	t.Cleanup(rawSrv.Close)

	client := fakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"is_archived":          true,
			"expiring_raw_log_url": rawSrv.URL,
		})
	})
	svc := NewService(client)

	var buf bytes.Buffer
	if err := svc.Log(context.Background(), "my-app", "b-1", &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "LOG OUTPUT") {
		t.Errorf("got %q", buf.String())
	}
}

func TestService_Log_RequiresSlugs(t *testing.T) {
	svc := NewService(fakeAPI(t, func(http.ResponseWriter, *http.Request) {}))
	if err := svc.Log(context.Background(), "", "b", io.Discard); err == nil {
		t.Fatal("missing app slug should fail")
	}
	if err := svc.Log(context.Background(), "a", "", io.Discard); err == nil {
		t.Fatal("missing build slug should fail")
	}
}

func TestService_NilClientFails(t *testing.T) {
	svc := NewService(nil)
	if _, err := svc.List(context.Background(), ListOptions{AppSlug: "x"}); err == nil {
		t.Fatal("List with nil client should fail")
	}
	if _, err := svc.View(context.Background(), "a", "b"); err == nil {
		t.Fatal("View with nil client should fail")
	}
	if _, err := svc.Trigger(context.Background(), TriggerRequest{AppSlug: "a", Workflow: "w"}); err == nil {
		t.Fatal("Trigger with nil client should fail")
	}
	if err := svc.Log(context.Background(), "a", "b", io.Discard); err == nil {
		t.Fatal("Log with nil client should fail")
	}
}

func TestStatusString(t *testing.T) {
	cases := map[int]string{
		0: "in-progress",
		1: "success",
		2: "failed",
		3: "aborted",
		4: "aborted-with-success",
		9: "9", // unknown — passthrough as integer string
	}
	for in, want := range cases {
		if got := statusString(in); got != want {
			t.Errorf("statusString(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestParseStatusFilter(t *testing.T) {
	if got, err := parseStatusFilter(""); err != nil || got != nil {
		t.Errorf("empty: got %v %v, want nil nil", got, err)
	}
	if got, err := parseStatusFilter("success"); err != nil || got == nil || *got != 1 {
		t.Errorf("success: got %v %v", got, err)
	}
	if got, err := parseStatusFilter("in-progress"); err != nil || got == nil || *got != 0 {
		t.Errorf("in-progress: got %v %v", got, err)
	}
	if _, err := parseStatusFilter("BOGUS"); err == nil {
		t.Error("expected error on unknown status")
	}
}
