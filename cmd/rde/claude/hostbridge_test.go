package claude

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

// sessionResponse is an httptest handler returning a running session whose VNC
// address is vncAddress (empty mimics a Linux session with no VNC endpoint).
func sessionResponse(vncAddress string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"session":{
			"id":"s-1","name":"dev","status":"SESSION_STATUS_RUNNING",
			"vncAddress":"`+vncAddress+`","vncUsername":"vagrant","vncPassword":"hunter2"
		}}`)
	}
}

func newTestService(t *testing.T, h http.Handler) *internalrde.Service {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return internalrde.NewService(rdeapi.New(srv.URL, "tok"))
}

func TestLocalHostActions_OffersOpenVNCWhenSessionHasVNC(t *testing.T) {
	svc := newTestService(t, sessionResponse("vnc://host.example:5900"))
	actions := localHostActions(context.Background(), svc, "ws-1", "s-1")
	action, ok := actions[internalrde.ActionOpenVNC]
	if !ok {
		t.Fatal("expected open-vnc action for a VNC-capable session")
	}
	// The action must ship a skill section, and it must reference its own route —
	// otherwise the composed skill would document an endpoint that doesn't exist
	// (or omit one that does).
	if !strings.Contains(action.SkillSection, internalrde.ActionOpenVNC) {
		t.Errorf("open-vnc skill section does not mention the %q route", internalrde.ActionOpenVNC)
	}
}

func TestLocalHostActions_SkipsOpenVNCWhenNoVNC(t *testing.T) {
	// A Linux session has no VNC endpoint; open-vnc must not be offered, so the
	// bridge ends up with no actions and the caller skips it (no skill written).
	svc := newTestService(t, sessionResponse(""))
	actions := localHostActions(context.Background(), svc, "ws-1", "s-1")
	if len(actions) != 0 {
		t.Fatalf("expected no actions for a session without VNC, got %d", len(actions))
	}
}

func TestOpenVNCActionOpensViewer(t *testing.T) {
	// The action fetches the session's VNC credentials and hands the vnc:// URL
	// to the opener. Stub the opener to capture the URL without launching a
	// viewer, and back the service with a session that exposes VNC.
	svc := newTestService(t, sessionResponse("vnc://host.example:5900"))

	var gotURL string
	prev := openVNCURL
	openVNCURL = func(_ context.Context, url string) error { gotURL = url; return nil }
	defer func() { openVNCURL = prev }()

	action := openVNCAction(svc, "ws-1", "s-1")
	res, err := action(context.Background(), httptest.NewRequest(http.MethodPost, "/open-vnc", nil))
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
