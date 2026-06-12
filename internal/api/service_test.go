package api

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

func TestDo_GETFieldsBecomeQuery(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	svc := NewService(bitriseapi.New(srv.URL, "tok"))

	_, err := svc.Do(context.Background(), Request{
		Method: http.MethodGet,
		Path:   "/apps",
		Fields: []KeyValue{{"sort_by", "created_at"}, {"limit", "10"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "sort_by=created_at") || !strings.Contains(gotQuery, "limit=10") {
		t.Errorf("query = %q, want fields as query params", gotQuery)
	}
}

func TestDo_WriteFieldsBecomeJSONBody(t *testing.T) {
	var gotBody []byte
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotCT = r.Header.Get("Content-Type")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	svc := NewService(bitriseapi.New(srv.URL, "tok"))

	_, err := svc.Do(context.Background(), Request{
		Method: http.MethodPost,
		Path:   "/apps",
		Fields: []KeyValue{{"branch", "main"}, {"workflow_id", "primary"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotCT)
	}
	var obj map[string]string
	if err := json.Unmarshal(gotBody, &obj); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, gotBody)
	}
	if obj["branch"] != "main" || obj["workflow_id"] != "primary" {
		t.Errorf("body = %v", obj)
	}
}

func TestDo_ContentTypeHeaderWins(t *testing.T) {
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	svc := NewService(bitriseapi.New(srv.URL, "tok"))

	hdr := http.Header{}
	hdr.Set("Content-Type", "application/vnd.custom+json")
	_, err := svc.Do(context.Background(), Request{
		Method:  http.MethodPost,
		Path:    "/apps",
		Fields:  []KeyValue{{"k", "v"}},
		Headers: hdr,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotCT != "application/vnd.custom+json" {
		t.Errorf("Content-Type = %q, want the user-supplied value", gotCT)
	}
}

func TestDo_InputBodySent(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	svc := NewService(bitriseapi.New(srv.URL, "tok"))

	_, err := svc.Do(context.Background(), Request{
		Method: http.MethodPut,
		Path:   "/apps/x",
		Body:   strings.NewReader(`{"raw":"payload"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotBody != `{"raw":"payload"}` {
		t.Errorf("body = %q", gotBody)
	}
}

func TestDo_FieldsAndBodyConflict(t *testing.T) {
	svc := NewService(bitriseapi.New("https://example.invalid", "tok"))
	_, err := svc.Do(context.Background(), Request{
		Method: http.MethodPost,
		Path:   "/apps",
		Fields: []KeyValue{{"k", "v"}},
		Body:   strings.NewReader("x"),
	})
	if err == nil || !strings.Contains(err.Error(), "--field") {
		t.Errorf("expected --field/--input conflict error, got %v", err)
	}
}

func TestDo_PaginateNonGETRejected(t *testing.T) {
	svc := NewService(bitriseapi.New("https://example.invalid", "tok"))
	_, err := svc.Do(context.Background(), Request{
		Method:   http.MethodPost,
		Path:     "/apps",
		Paginate: true,
	})
	if err == nil || !strings.Contains(err.Error(), "--all") {
		t.Errorf("expected --all non-GET error, got %v", err)
	}
}

func TestDo_PaginateMergesPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("next") {
		case "":
			_, _ = w.Write([]byte(`{"data":[{"id":"a"},{"id":"b"}],"paging":{"next":"c2"}}`))
		case "c2":
			_, _ = w.Write([]byte(`{"data":[{"id":"c"}],"paging":{"next":""}}`))
		default:
			t.Errorf("unexpected cursor %q", r.URL.Query().Get("next"))
		}
	}))
	t.Cleanup(srv.Close)
	svc := NewService(bitriseapi.New(srv.URL, "tok"))

	resp, err := svc.Do(context.Background(), Request{
		Method:   http.MethodGet,
		Path:     "/apps",
		Paginate: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &env); err != nil {
		t.Fatalf("merged body not JSON: %v (%s)", err, resp.Body)
	}
	if len(env.Data) != 3 {
		t.Fatalf("merged %d items, want 3: %s", len(env.Data), resp.Body)
	}
	if env.Data[0].ID != "a" || env.Data[2].ID != "c" {
		t.Errorf("merged order wrong: %s", resp.Body)
	}
}

func TestDo_PaginateNoOpOnNonList(t *testing.T) {
	const single = `{"data":{"username":"alice"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(single))
	}))
	t.Cleanup(srv.Close)
	svc := NewService(bitriseapi.New(srv.URL, "tok"))

	resp, err := svc.Do(context.Background(), Request{
		Method:   http.MethodGet,
		Path:     "/me",
		Paginate: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Body) != single {
		t.Errorf("body = %q, want unchanged single response", resp.Body)
	}
}

func TestDo_PaginateStopsOnErrorPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"nope"}`))
	}))
	t.Cleanup(srv.Close)
	svc := NewService(bitriseapi.New(srv.URL, "tok"))

	resp, err := svc.Do(context.Background(), Request{
		Method:   http.MethodGet,
		Path:     "/apps",
		Paginate: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("StatusCode = %d, want 403", resp.StatusCode)
	}
}
