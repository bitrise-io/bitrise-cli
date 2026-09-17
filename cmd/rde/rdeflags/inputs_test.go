package rdeflags

import "testing"

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
