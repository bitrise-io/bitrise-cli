package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestParseFormat(t *testing.T) {
	cases := []struct {
		in      string
		want    Format
		wantErr bool
	}{
		{"", Human, false}, // empty defaults to human
		{"human", Human, false},
		{"json", JSON, false},
		{"text", "", true},  // legacy alias is intentionally rejected
		{"yaml", "", true},  // not yet supported
		{"HUMAN", "", true}, // case-sensitive on purpose
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := ParseFormat(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParseFormat(%q): expected error, got %v", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFormat(%q): unexpected error: %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("ParseFormat(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

type sample struct {
	Name string `json:"name"`
	N    int    `json:"n"`
}

func TestRender_JSON(t *testing.T) {
	var buf bytes.Buffer
	err := Render(&buf, JSON, sample{Name: "x", N: 7}, nil)
	if err != nil {
		t.Fatalf("Render JSON: %v", err)
	}
	var got sample
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, buf.String())
	}
	if got != (sample{Name: "x", N: 7}) {
		t.Fatalf("decoded JSON differs: got %+v", got)
	}
	// Confirm indented output (we use SetIndent in Render).
	if !strings.Contains(buf.String(), "\n  ") {
		t.Errorf("expected indented JSON, got %q", buf.String())
	}
}

func TestRender_Human(t *testing.T) {
	var buf bytes.Buffer
	render := func(w io.Writer, v sample) error {
		_, err := w.Write([]byte("name=" + v.Name))
		return err
	}
	if err := Render(&buf, Human, sample{Name: "y"}, render); err != nil {
		t.Fatalf("Render Human: %v", err)
	}
	if buf.String() != "name=y" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestRender_HumanRequiresRenderer(t *testing.T) {
	var buf bytes.Buffer
	err := Render[sample](&buf, Human, sample{}, nil)
	if err == nil {
		t.Fatal("expected error when human renderer is nil")
	}
}

func TestRender_UnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	err := Render[sample](&buf, Format("yaml"), sample{}, nil)
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

// Render bubbles up errors from the human renderer.
func TestRender_PropagatesHumanError(t *testing.T) {
	wantErr := errors.New("boom")
	render := func(_ io.Writer, _ sample) error { return wantErr }
	err := Render(io.Discard, Human, sample{}, render)
	if !errors.Is(err, wantErr) {
		t.Fatalf("got %v, want %v", err, wantErr)
	}
}
