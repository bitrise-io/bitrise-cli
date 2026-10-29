package rdeflags

import (
	"strings"
	"testing"
)

func TestParseSessionInputs(t *testing.T) {
	got, err := ParseSessionInputs(
		[]string{"repo=my-app"},
		[]string{"token=ghp_x"},
		[]string{"key=saved-id"},
	)
	if err != nil {
		t.Fatalf("ParseSessionInputs: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d inputs, want 3", len(got))
	}
	if got[0].Key != "repo" || got[0].Value != "my-app" || got[0].IsSecret {
		t.Errorf("plain input wrong: %+v", got[0])
	}
	if !got[1].IsSecret || got[1].Value != "ghp_x" {
		t.Errorf("secret input wrong: %+v", got[1])
	}
	if got[2].SavedInputID != "saved-id" || got[2].Value != "" {
		t.Errorf("saved input wrong: %+v", got[2])
	}

	if _, err := ParseSessionInputs([]string{"bad"}, nil, nil); err == nil {
		t.Error("expected error for malformed --input")
	}
	if _, err := ParseSessionInputs(nil, nil, []string{"key="}); err == nil {
		t.Error("expected error for --saved-input with empty ID")
	}
}

// An empty value is a legal plain input (key= clears a default); only the
// saved-input form insists on a non-empty right-hand side.
func TestParseSessionInputs_EmptyValueAllowed(t *testing.T) {
	got, err := ParseSessionInputs([]string{"flag="}, nil, nil)
	if err != nil {
		t.Fatalf("ParseSessionInputs: %v", err)
	}
	if len(got) != 1 || got[0].Key != "flag" || got[0].Value != "" {
		t.Errorf("got %+v, want key flag with empty value", got)
	}
}

// A scope prefix (scope:key=value) scopes the value to that scope on every
// flag kind; an unknown scope or an empty key behind the prefix is an error.
// A key without a prefix stays unscoped (Hook empty).
func TestParseSessionInputs_ScopePrefix(t *testing.T) {
	got, err := ParseSessionInputs(
		[]string{"on_claim:GH_TOKEN=a", "GH_TOKEN=b"},
		[]string{"on_restore:GH_TOKEN=c"},
		[]string{"shared:key=saved-id"},
	)
	if err != nil {
		t.Fatalf("ParseSessionInputs: %v", err)
	}
	want := []struct{ key, value, hook string }{
		{"GH_TOKEN", "a", "on_claim"},
		{"GH_TOKEN", "b", ""},
		{"GH_TOKEN", "c", "on_restore"},
		{"key", "", "shared"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d inputs, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Key != w.key || got[i].Value != w.value || got[i].Hook != w.hook {
			t.Errorf("input %d = %+v, want key %q value %q hook %q", i, got[i], w.key, w.value, w.hook)
		}
	}
	if got[3].SavedInputID != "saved-id" {
		t.Errorf("scoped saved input lost its ID: %+v", got[3])
	}

	for _, bad := range []string{"later:GH_TOKEN=a", "on_claim:=a", "on_claim:"} {
		if _, err := ParseSessionInputs([]string{bad}, nil, nil); err == nil {
			t.Errorf("expected error for --input %q", bad)
		}
	}
	if _, err := ParseSessionInputs(nil, []string{"nope:K=v"}, nil); err == nil {
		t.Error("expected error for --secret-input with an unknown scope")
	}
	if _, err := ParseSessionInputs(nil, nil, []string{"on_claim:key="}); err == nil {
		t.Error("expected error for a scoped --saved-input with empty ID")
	}
}

// A malformed --secret-input must not print its value.
func TestParseSessionInputs_ErrorsNeverEchoTheValue(t *testing.T) {
	const secret = "s3cr3t-value"
	for _, bad := range []string{"on-claim:TOKEN=" + secret, ":TOKEN=" + secret, "on_claim:=" + secret, secret} {
		_, err := ParseSessionInputs(nil, []string{bad}, nil)
		if err == nil {
			t.Fatalf("expected error for --secret-input %q", bad)
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("error for a malformed --secret-input prints its value: %v", err)
		}
	}
}
