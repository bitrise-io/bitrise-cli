package rde

import (
	"context"
	"time"

	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
)

// PreviewLink is a minted device preview link: a shareable URL that opens an
// app build on a live iOS simulator or Android emulator in a browser, with no
// Bitrise login.
//
// Nothing is stored server-side, so this is the only record of the link that
// will ever exist — it cannot be listed, read back, or revoked, only left to
// expire. Treat URL and Token as bearer credentials.
type PreviewLink struct {
	// URL is the shareable link.
	URL string `json:"url,omitempty"`
	// Token is the signed link on its own.
	Token string `json:"token"`
	// JTI is the link's id, recorded on every session the link spawns — the
	// handle for finding and stopping them.
	JTI string `json:"jti"`
	// ExpiresAt is when the link stops opening. Devices already open are
	// unaffected and end on their own idle window.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// CreatePreviewLinkRequest is the CLI-stable mint request. DeviceSpec and
// Artifact are both required: a link always names the device to boot and the
// app to install on it.
type CreatePreviewLinkRequest struct {
	DeviceSpec *DeviceSpec
	Artifact   *DeviceArtifact
	// TTL is the link's lifetime; zero uses the default of 24 hours.
	TTL time.Duration
	// StackID and MachineType override the platform defaults; empty uses them.
	StackID     string
	MachineType string
	// SessionAutoTerminateMinutes is the idle window of the sessions the link
	// spawns; zero uses the default of 60 minutes.
	SessionAutoTerminateMinutes int
	// WarmPoolID serves the link's opens from a warm pool: each open claims
	// a warm session when one is available and creates one from the pool's
	// configuration otherwise. The pool must be workspace-owned and boot a
	// device; it fixes the machine and the device, so StackID, MachineType
	// and DeviceSpec must be empty.
	WarmPoolID string
}

// CreatePreviewLink mints a device preview link for an app build.
func (s *Service) CreatePreviewLink(ctx context.Context, workspaceID string, req CreatePreviewLinkRequest) (PreviewLink, error) {
	if s.client == nil {
		return PreviewLink{}, errClient()
	}
	w, err := s.client.CreatePreviewLink(ctx, workspaceID, rdeapi.CreatePreviewLinkRequest{
		DeviceSpec:                  deviceSpecToAPI(req.DeviceSpec),
		Artifact:                    artifactToAPI(req.Artifact),
		TTLSeconds:                  int64(req.TTL.Seconds()),
		StackID:                     req.StackID,
		MachineType:                 req.MachineType,
		SessionAutoTerminateMinutes: req.SessionAutoTerminateMinutes,
		WarmPoolID:                  req.WarmPoolID,
	})
	if err != nil {
		return PreviewLink{}, err
	}
	return previewLinkFromAPI(w), nil
}

func previewLinkFromAPI(w rdeapi.PreviewLink) PreviewLink {
	return PreviewLink{
		URL:       w.URL,
		Token:     w.Token,
		JTI:       w.JTI,
		ExpiresAt: parseTime(w.ExpiresAt),
	}
}
