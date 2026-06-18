package claude

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func TestNewHostBridgeRegistersOpenVNC(t *testing.T) {
	b := newHostBridge(nil, "ws-1", "sess-1")
	if b.WorkspaceID != "ws-1" || b.SessionID != "sess-1" {
		t.Errorf("bridge ids = %q/%q, want ws-1/sess-1", b.WorkspaceID, b.SessionID)
	}
	if _, ok := b.Actions[internalrde.ActionOpenVNC]; !ok {
		t.Fatal("open-vnc action not registered")
	}
}

func TestOpenVNCActionOpensViewer(t *testing.T) {
	// The action fetches the session's VNC credentials and hands the vnc:// URL
	// to the opener. Stub the opener to capture the URL without launching a
	// viewer, and back the service with a session that exposes VNC.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"session":{
			"id":"s-1","name":"dev","status":"SESSION_STATUS_RUNNING",
			"vncAddress":"vnc://host.example:5900","vncUsername":"vagrant","vncPassword":"hunter2"
		}}`)
	}))
	defer srv.Close()

	svc := internalrde.NewService(rdeapi.New(srv.URL, "tok"))

	var gotURL string
	prev := openVNCURL
	openVNCURL = func(_ context.Context, url string) error { gotURL = url; return nil }
	defer func() { openVNCURL = prev }()

	b := newHostBridge(svc, "ws-1", "s-1")
	action := b.Actions[internalrde.ActionOpenVNC]
	res, err := action.Handle(context.Background(), httptest.NewRequest(http.MethodPost, "/open-vnc", nil))
	if err != nil {
		t.Fatalf("action: %v", err)
	}

	want := "vnc://vagrant:hunter2@host.example:5900" // #nosec G101 -- test fixture
	if gotURL != want {
		t.Errorf("opened URL = %q, want %q", gotURL, want)
	}
	m, ok := res.(map[string]any)
	if !ok || m["opened"] != true || m["address"] != "vnc://host.example:5900" {
		t.Errorf("result = %v, want opened=true with address", res)
	}
}
