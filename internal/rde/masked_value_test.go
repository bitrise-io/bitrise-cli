package rde

import (
	"encoding/json"
	"strings"
	"testing"

	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
)

const (
	mask       = "****"
	maskSuffix = "****5678"
)

// The backend returns secrets write-only, with a masked_value. The CLI keeps
// that mask, never a value, and never anything that is not a mask.
func TestMaskedValue(t *testing.T) {
	cases := map[string]struct {
		isSecret bool
		wire     string
		want     string
	}{
		"plain value has no mask":            {false, maskSuffix, ""},
		"secret keeps the backend mask":      {true, maskSuffix, maskSuffix},
		"short secret keeps the bare mask":   {true, mask, mask},
		"secret without a mask gets one":     {true, "", secretMask},
		"anything not a mask is never shown": {true, "ghp_plaintext", secretMask},
		"a mask hiding more is refused":      {true, "****plaintext", secretMask},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := maskedValue(tc.isSecret, tc.wire); got != tc.want {
				t.Errorf("maskedValue(%v, %q) = %q, want %q", tc.isSecret, tc.wire, got, tc.want)
			}
		})
	}
}

func TestSavedInputFromAPI_MaskedValue(t *testing.T) {
	secret := savedInputFromAPI(rdeapi.SavedInput{ID: "1", Key: "GH", Value: "leak", IsSecret: true, MaskedValue: maskSuffix})
	if secret.Value != "" || secret.MaskedValue != maskSuffix {
		t.Errorf("secret = %+v", secret)
	}
	plain := savedInputFromAPI(rdeapi.SavedInput{ID: "2", Key: "REPO", Value: "app"})
	if plain.Value != "app" || plain.MaskedValue != "" {
		t.Errorf("plain = %+v", plain)
	}
	data, err := json.Marshal(secret)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"masked_value":"`+maskSuffix+`"`) || strings.Contains(string(data), "leak") {
		t.Errorf("json = %s", data)
	}
}

func TestTemplateFromAPI_MaskedValue(t *testing.T) {
	tpl := templateFromAPI(rdeapi.Template{TemplateVariables: []rdeapi.TemplateVariable{
		{Key: "TOKEN", Value: "leak", IsSecret: true, Protected: true, MaskedValue: maskSuffix},
		{Key: "APP", Value: "demo"},
	}})
	if v := tpl.TemplateVariables[0]; v.Value != "" || v.MaskedValue != maskSuffix || !v.IsSecret {
		t.Errorf("secret = %+v", v)
	}
	if v := tpl.TemplateVariables[1]; v.Value != "demo" || v.MaskedValue != "" {
		t.Errorf("plain = %+v", v)
	}
}

func TestSnapshotFromAPI_MaskedValue(t *testing.T) {
	snap := snapshotFromAPI(rdeapi.SessionTemplateSnapshot{SessionInputs: []rdeapi.SnapshotInput{
		{Key: "GH", Value: "leak", IsSecret: true, MaskedValue: maskSuffix},
		{Key: "SEED", IsSecret: true, Hook: "on_create", SuppliedByPool: true},
		{Key: "REPO", Value: "app"},
	}})
	if in := snap.SessionInputs[0]; in.Value != "" || in.MaskedValue != maskSuffix {
		t.Errorf("secret = %+v", in)
	}
	if in := snap.SessionInputs[1]; in.Value != "" || in.MaskedValue != "" {
		t.Errorf("a pool-supplied value has no mask either: %+v", in)
	}
	if in := snap.SessionInputs[2]; in.Value != "app" || in.MaskedValue != "" {
		t.Errorf("plain = %+v", in)
	}
}

func TestWarmPoolFromAPI_MaskedValue(t *testing.T) {
	pool := warmPoolFromAPI(rdeapi.WarmPool{SessionInputs: []rdeapi.SessionInputValue{
		{Key: "GH", Value: "leak", IsSecret: true, MaskedValue: maskSuffix},
		{Key: "SSH_KEY", IsSecret: true, SavedInputID: "si-1"},
		{Key: "REPO", Value: "app"},
	}})
	if in := pool.SessionInputs[0]; in.Value != "" || in.MaskedValue != maskSuffix {
		t.Errorf("secret = %+v", in)
	}
	if in := pool.SessionInputs[1]; in.MaskedValue != "" || in.SavedInputID != "si-1" {
		t.Errorf("a saved-input reference has no mask: %+v", in)
	}
	if in := pool.SessionInputs[2]; in.Value != "app" || in.MaskedValue != "" {
		t.Errorf("plain = %+v", in)
	}
}
