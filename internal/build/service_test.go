package build

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

func TestService_Trigger_RequiresAppSlug(t *testing.T) {
	svc := NewService(fakeAPI(t, func(http.ResponseWriter, *http.Request) {}))
	if _, err := svc.Trigger(context.Background(), TriggerRequest{Workflow: "x"}); err == nil {
		t.Fatal("missing app slug should fail")
	}
}

func TestService_Trigger_PipelineFields(t *testing.T) {
	var gotBody []byte
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"build_slug":"p-1","build_number":5,"build_url":"https://app.bitrise.io/build/p-1"}`))
	})
	svc := NewService(client)

	got, err := svc.Trigger(context.Background(), TriggerRequest{
		AppSlug:    "my-app",
		Pipeline:   "my-pipeline",
		Branch:     "main",
		BranchDest: "release",
		Tag:        "v1.0.0",
	})
	if err != nil {
		t.Fatal(err)
	}

	var sent map[string]any
	_ = json.Unmarshal(gotBody, &sent)
	bp, _ := sent["build_params"].(map[string]any)
	if bp["pipeline_id"] != "my-pipeline" {
		t.Errorf("pipeline_id = %v", bp["pipeline_id"])
	}
	if bp["branch_dest"] != "release" {
		t.Errorf("branch_dest = %v", bp["branch_dest"])
	}
	if bp["tag"] != "v1.0.0" {
		t.Errorf("tag = %v", bp["tag"])
	}
	if bp["workflow_id"] != nil {
		t.Errorf("workflow_id should be absent, got %v", bp["workflow_id"])
	}

	if got.Tag != "v1.0.0" || got.Branch != "main" {
		t.Errorf("result fields: tag=%q branch=%q", got.Tag, got.Branch)
	}
}

func TestService_Trigger_EnvsAndPriorityAndPR(t *testing.T) {
	var gotBody []byte
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"build_slug":"e-1","build_number":10}`))
	})
	svc := NewService(client)

	_, err := svc.Trigger(context.Background(), TriggerRequest{
		AppSlug:       "my-app",
		Workflow:      "primary",
		PullRequestID: 42,
		Priority:      1,
		Environments: []TriggerEnv{
			{Key: "MY_VAR", Value: "hello"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var sent map[string]any
	_ = json.Unmarshal(gotBody, &sent)
	bp, _ := sent["build_params"].(map[string]any)

	if bp["pull_request_id"] != float64(42) {
		t.Errorf("pull_request_id = %v", bp["pull_request_id"])
	}
	if bp["priority"] != float64(1) {
		t.Errorf("priority = %v", bp["priority"])
	}
	envs, _ := bp["environments"].([]any)
	if len(envs) != 1 {
		t.Fatalf("environments len = %d, want 1", len(envs))
	}
	env, _ := envs[0].(map[string]any)
	if env["mapped_to"] != "MY_VAR" || env["value"] != "hello" || env["is_expand"] != true {
		t.Errorf("env = %v", env)
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
	if _, err := svc.Trigger(context.Background(), TriggerRequest{AppSlug: "a"}); err == nil {
		t.Fatal("Trigger with nil client should fail")
	}
	if err := svc.Log(context.Background(), "a", "b", io.Discard); err == nil {
		t.Fatal("Log with nil client should fail")
	}
	if _, err := svc.Watch(context.Background(), "a", "b", io.Discard, nil, time.Second); err == nil {
		t.Fatal("Watch with nil client should fail")
	}
}

func TestService_Watch_DeltaStreaming(t *testing.T) {
	// Three log polls: chunk1 (ts1), chunk2 (ts2), chunk3 (empty next ts).
	// Build status returns in-progress until the log stream ends naturally,
	// then the final-flush call and a View call complete the sequence.
	var logCalls, buildCalls atomic.Int32
	var logTimestamps []string

	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apps/my-app/builds/b-1/log":
			n := int(logCalls.Add(1))
			logTimestamps = append(logTimestamps, r.URL.Query().Get("after_timestamp"))
			switch n {
			case 1:
				_, _ = w.Write([]byte(`{"is_archived":false,"log_chunks":[{"chunk":"chunk1\n","position":0}],"next_after_timestamp":"ts1"}`))
			case 2:
				_, _ = w.Write([]byte(`{"is_archived":false,"log_chunks":[{"chunk":"chunk2\n","position":1}],"next_after_timestamp":"ts2"}`))
			case 3:
				// Empty next_after_timestamp: loop exits after this poll.
				_, _ = w.Write([]byte(`{"is_archived":false,"log_chunks":[{"chunk":"chunk3\n","position":2}]}`))
			default: // final flush
				_, _ = w.Write([]byte(`{"is_archived":false,"log_chunks":[{"chunk":"final\n","position":3}]}`))
			}
		case "/apps/my-app/builds/b-1":
			n := int(buildCalls.Add(1))
			// Stay in-progress while the log stream is active so we can observe
			// all chunks; return success on the final View call.
			if n <= 2 {
				_, _ = w.Write([]byte(`{"data":{"slug":"b-1","build_number":1,"status":0}}`))
			} else {
				_, _ = w.Write([]byte(`{"data":{"slug":"b-1","build_number":1,"status":1,"triggered_workflow":"primary","branch":"main"}}`))
			}
		}
	})
	svc := NewService(client)

	var buf bytes.Buffer
	build, err := svc.Watch(context.Background(), "my-app", "b-1", &buf, nil, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	// All chunks must appear in order.
	got := buf.String()
	for _, want := range []string{"chunk1", "chunk2", "chunk3", "final"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q; got %q", want, got)
		}
	}

	// Verify after_timestamp progression for log calls: "", ts1, ts2, ts2 (final flush).
	wantTimestamps := []string{"", "ts1", "ts2", "ts2"}
	if len(logTimestamps) < len(wantTimestamps) {
		t.Fatalf("expected at least %d log calls, got %d", len(wantTimestamps), len(logTimestamps))
	}
	for i, want := range wantTimestamps {
		if logTimestamps[i] != want {
			t.Errorf("log call %d after_timestamp = %q, want %q", i+1, logTimestamps[i], want)
		}
	}

	if build.Status != "success" {
		t.Errorf("final status = %q, want success", build.Status)
	}
}

// recordingSink captures every OnUpdate snapshot for assertion. The sink
// stores a fresh copy of the map each call so later mutations from Watch
// don't leak into earlier snapshots.
type recordingSink struct {
	snapshots []map[int]string
}

func (s *recordingSink) OnUpdate(chunks map[int]string) error {
	c := make(map[int]string, len(chunks))
	for k, v := range chunks {
		c[k] = v
	}
	s.snapshots = append(s.snapshots, c)
	return nil
}

func TestService_Watch_SinkAccumulatesAcrossPolls(t *testing.T) {
	// Across two live polls the API delivers chunks at positions 1, 0, 2.
	// The sink must see the cumulative state on each call (not just the
	// new batch) so the cmd layer can render the full sorted log.
	var logCalls atomic.Int32
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apps/my-app/builds/b-1/log":
			n := int(logCalls.Add(1))
			switch n {
			case 1:
				// Out-of-order within a batch.
				_, _ = w.Write([]byte(`{"is_archived":false,"log_chunks":[{"chunk":"B\n","position":1},{"chunk":"A\n","position":0}],"next_after_timestamp":"ts1"}`))
			case 2:
				_, _ = w.Write([]byte(`{"is_archived":false,"log_chunks":[{"chunk":"C\n","position":2}]}`))
			default:
				_, _ = w.Write([]byte(`{"is_archived":false,"log_chunks":[]}`))
			}
		case "/apps/my-app/builds/b-1":
			_, _ = w.Write([]byte(`{"data":{"slug":"b-1","build_number":1,"status":1,"triggered_workflow":"primary","branch":"main"}}`))
		}
	})
	svc := NewService(client)

	sink := &recordingSink{}
	if _, err := svc.Watch(context.Background(), "my-app", "b-1", io.Discard, sink, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if len(sink.snapshots) < 2 {
		t.Fatalf("expected at least 2 sink updates, got %d", len(sink.snapshots))
	}
	// First snapshot: positions 0,1 from batch 1.
	first := sink.snapshots[0]
	if first[0] != "A\n" || first[1] != "B\n" || len(first) != 2 {
		t.Errorf("first snapshot = %v, want {0:A\\n, 1:B\\n}", first)
	}
	// Last snapshot: cumulative — position 2 from batch 2 plus carryover.
	last := sink.snapshots[len(sink.snapshots)-1]
	if last[0] != "A\n" || last[1] != "B\n" || last[2] != "C\n" {
		t.Errorf("last snapshot = %v, want {0:A\\n, 1:B\\n, 2:C\\n}", last)
	}
}

func TestService_Watch_SortsChunksByPosition(t *testing.T) {
	// API returns chunks shuffled within a single response — the watch
	// streamer must sort by position before printing.
	var logCalls atomic.Int32
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apps/my-app/builds/b-1/log":
			n := int(logCalls.Add(1))
			if n == 1 {
				_, _ = w.Write([]byte(`{"is_archived":false,"log_chunks":[{"chunk":"third\n","position":2},{"chunk":"first\n","position":0},{"chunk":"second\n","position":1}]}`))
			} else {
				_, _ = w.Write([]byte(`{"is_archived":false,"log_chunks":[]}`))
			}
		case "/apps/my-app/builds/b-1":
			_, _ = w.Write([]byte(`{"data":{"slug":"b-1","build_number":1,"status":1,"triggered_workflow":"primary","branch":"main"}}`))
		}
	})
	svc := NewService(client)

	var buf bytes.Buffer
	if _, err := svc.Watch(context.Background(), "my-app", "b-1", &buf, nil, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	want := "first\nsecond\nthird\n"
	if buf.String() != want {
		t.Errorf("output = %q, want %q (chunks must be sorted by position)", buf.String(), want)
	}
}

func TestService_Watch_AlreadyArchived(t *testing.T) {
	rawSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ARCHIVED LOG\n"))
	}))
	t.Cleanup(rawSrv.Close)

	var logCallCount atomic.Int32
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apps/my-app/builds/b-done/log":
			logCallCount.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"is_archived":          true,
				"expiring_raw_log_url": rawSrv.URL,
			})
		case "/apps/my-app/builds/b-done":
			_, _ = w.Write([]byte(`{"data":{"slug":"b-done","build_number":5,"status":1}}`))
		}
	})
	svc := NewService(client)

	var buf bytes.Buffer
	build, err := svc.Watch(context.Background(), "my-app", "b-done", &buf, nil, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "ARCHIVED LOG") {
		t.Errorf("expected archived log content, got %q", buf.String())
	}
	if build.Status != "success" {
		t.Errorf("final status = %q, want success", build.Status)
	}
}

func TestService_Watch_RetriesOn404(t *testing.T) {
	// First log call returns 404 (build not yet started); second succeeds.
	var callCount atomic.Int32
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		n := int(callCount.Add(1))
		switch r.URL.Path {
		case "/apps/my-app/builds/b-1/log":
			if n == 1 {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"message":"build not found"}`))
				return
			}
			// Second call: build has started, archived immediately.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"is_archived": true,
				"log_chunks":  []map[string]any{{"chunk": "log line\n", "position": 0}},
			})
		case "/apps/my-app/builds/b-1":
			_, _ = w.Write([]byte(`{"data":{"slug":"b-1","build_number":1,"status":1}}`))
		}
	})
	svc := NewService(client)

	var buf bytes.Buffer
	build, err := svc.Watch(context.Background(), "my-app", "b-1", &buf, nil, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if build.Status != "success" {
		t.Errorf("status = %q, want success", build.Status)
	}
}

func TestService_Watch_FailsOnSecond404(t *testing.T) {
	// Both log calls return 404; Watch should return an error.
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/apps/my-app/builds/b-1/log" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"build not found"}`))
		}
	})
	svc := NewService(client)

	_, err := svc.Watch(context.Background(), "my-app", "b-1", io.Discard, nil, time.Millisecond)
	if err == nil {
		t.Fatal("expected error on persistent 404")
	}
	var apiErr *bitriseapi.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 APIError, got %v", err)
	}
}

func TestService_Watch_StopsOnIsArchived(t *testing.T) {
	// Poll returns IsArchived=true with a non-empty NextAfterTimestamp; Watch
	// must stop and not loop indefinitely.
	var logCallCount atomic.Int32
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apps/my-app/builds/b-1/log":
			n := int(logCallCount.Add(1))
			if n == 1 {
				// First call: in-progress with a next timestamp
				_, _ = w.Write([]byte(`{"is_archived":false,"log_chunks":[{"chunk":"line1\n","position":0}],"next_after_timestamp":"ts1"}`))
				return
			}
			// Second call: archived but NextAfterTimestamp still set — this is the bug scenario
			_, _ = w.Write([]byte(`{"is_archived":true,"log_chunks":[{"chunk":"line2\n","position":1}],"next_after_timestamp":"ts2"}`))
		case "/apps/my-app/builds/b-1":
			_, _ = w.Write([]byte(`{"data":{"slug":"b-1","build_number":1,"status":1}}`))
		}
	})
	svc := NewService(client)

	var buf bytes.Buffer
	build, err := svc.Watch(context.Background(), "my-app", "b-1", &buf, nil, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "line1") || !strings.Contains(buf.String(), "line2") {
		t.Errorf("expected both chunks in output, got %q", buf.String())
	}
	if build.Status != "success" {
		t.Errorf("status = %q, want success", build.Status)
	}
}

func TestService_Watch_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	client := fakeAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		// Return a next timestamp so Watch will enter the sleep-poll loop,
		// then cancel the context so the select fires.
		_, _ = w.Write([]byte(`{"is_archived":false,"log_chunks":[{"chunk":"line\n","position":0}],"next_after_timestamp":"ts1"}`))
		cancel()
	})
	svc := NewService(client)

	var buf bytes.Buffer
	_, err := svc.Watch(ctx, "my-app", "b-1", &buf, nil, 10*time.Millisecond)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestService_Watch_StopsOnBuildStatus(t *testing.T) {
	// Watch should exit the poll loop when the build status is no longer
	// in-progress (0), without waiting for the log to be archived.
	var logCalls, buildCalls atomic.Int32
	client := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apps/my-app/builds/b-1/log":
			n := int(logCalls.Add(1))
			if n == 1 {
				// Initial call: in-progress, gives first chunks.
				_, _ = w.Write([]byte(`{"is_archived":false,"log_chunks":[{"chunk":"line1\n","position":0}],"next_after_timestamp":"ts1"}`))
			} else {
				// Poll: more chunks arrive but is_archived stays false.
				_, _ = w.Write([]byte(`{"is_archived":false,"log_chunks":[{"chunk":"line2\n","position":1}],"next_after_timestamp":"ts2"}`))
			}
		case "/apps/my-app/builds/b-1":
			buildCalls.Add(1)
			_, _ = w.Write([]byte(`{"data":{"slug":"b-1","build_number":5,"status":1,"status_text":"success"}}`))
		}
	})
	svc := NewService(client)

	var buf bytes.Buffer
	build, err := svc.Watch(context.Background(), "my-app", "b-1", &buf, nil, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "line1") || !strings.Contains(buf.String(), "line2") {
		t.Errorf("expected both log lines, got %q", buf.String())
	}
	if build.Status != "success" {
		t.Errorf("status = %q, want success", build.Status)
	}
	if n := int(buildCalls.Load()); n < 1 {
		t.Errorf("expected at least 1 build status call, got %d", n)
	}
	// Should have stopped after a single poll cycle, not looped many times.
	if n := int(logCalls.Load()); n > 3 {
		t.Errorf("too many log calls (%d), Watch did not stop on build status", n)
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
