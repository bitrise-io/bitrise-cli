// Package style holds the lipgloss-backed semantic styles used by the
// human renderers. JSON output never goes through this package — ANSI
// codes must not leak into machine-readable output.
//
// Each renderer constructs Styles via New(writer). The returned bundle is
// scoped to that writer's color profile, so non-TTY writers (pipes,
// tests using *bytes.Buffer) automatically produce ANSI-free output.
// NO_COLOR and FORCE_COLOR are honored automatically by the underlying
// termenv detection. The Configure function adds an explicit override
// for the --no-color flag.
package style

import (
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// 256-color palette. Lipgloss falls back to the nearest ANSI-16 color
// when the terminal can't render 256-color, so these values are safe to
// use even on legacy terminals.
const (
	colorGrey   = "245" // dim text
	colorGreen  = "42"  // success
	colorRed    = "196" // failed
	colorBlue   = "33"  // in-progress (running, neutral, stands out)
	colorAmber  = "214" // aborted
	colorYellow = "220" // warnings
)

// forceNoColor is set by Configure when the --no-color flag is passed.
// It overrides termenv's auto-detection on every renderer constructed
// after Configure runs.
var forceNoColor bool

// Configure applies process-wide style settings. Call once from the cmd
// layer's persistentPreRun, after parsing the --no-color flag.
func Configure(noColor bool) {
	forceNoColor = noColor
}

// Styles bundles the semantic styles used across human renderers. It is
// constructed per-writer so the writer's terminal capabilities (or lack
// thereof) control whether ANSI is emitted.
type Styles struct {
	r *lipgloss.Renderer

	Header  lipgloss.Style // table header rows
	Dim     lipgloss.Style // de-emphasized text
	Bold    lipgloss.Style // generic emphasis
	Label   lipgloss.Style // key in a key/value detail line
	Slug    lipgloss.Style // technical identifiers (dimmed)
	URL     lipgloss.Style // URLs (underlined)
	Success lipgloss.Style // success indicators / "✓"
	Failure lipgloss.Style // failure indicators / "✗"
	Warn    lipgloss.Style // warnings / "!"

	statusSuccess    lipgloss.Style
	statusFailed     lipgloss.Style
	statusAborted    lipgloss.Style
	statusInProgress lipgloss.Style
	statusUnknown    lipgloss.Style
}

// New returns a Styles bundle for the given writer. ANSI escape codes
// are emitted only if the writer is detected as a color-capable TTY and
// --no-color is not in effect.
func New(w io.Writer) Styles {
	r := lipgloss.NewRenderer(w)
	if forceNoColor {
		r.SetColorProfile(termenv.Ascii)
	}
	return Styles{
		r:                r,
		Header:           r.NewStyle().Bold(true).Foreground(lipgloss.Color(colorGrey)),
		Dim:              r.NewStyle().Foreground(lipgloss.Color(colorGrey)),
		Bold:             r.NewStyle().Bold(true),
		Label:            r.NewStyle().Bold(true),
		Slug:             r.NewStyle().Foreground(lipgloss.Color(colorGrey)),
		URL:              r.NewStyle().Underline(true),
		Success:          r.NewStyle().Foreground(lipgloss.Color(colorGreen)),
		Failure:          r.NewStyle().Foreground(lipgloss.Color(colorRed)),
		Warn:             r.NewStyle().Foreground(lipgloss.Color(colorYellow)),
		statusSuccess:    r.NewStyle().Foreground(lipgloss.Color(colorGreen)),
		statusFailed:     r.NewStyle().Foreground(lipgloss.Color(colorRed)),
		statusAborted:    r.NewStyle().Foreground(lipgloss.Color(colorAmber)),
		statusInProgress: r.NewStyle().Foreground(lipgloss.Color(colorBlue)),
		statusUnknown:    r.NewStyle().Foreground(lipgloss.Color(colorGrey)),
	}
}

// BuildStatus picks the right semantic style for a build's string status.
// Unknown values fall through to a dim grey so future API statuses don't
// look broken.
func (s Styles) BuildStatus(status string) lipgloss.Style {
	switch status {
	case "success":
		return s.statusSuccess
	case "failed":
		return s.statusFailed
	case "aborted", "aborted-with-success":
		return s.statusAborted
	case "in-progress":
		return s.statusInProgress
	default:
		return s.statusUnknown
	}
}

// HasColor reports whether the styles will emit ANSI codes. Useful for
// tests and for skipping iconography on legacy terminals.
func (s Styles) HasColor() bool {
	return s.r.ColorProfile() != termenv.Ascii
}

// CellStyler renders a single cell content string. row==-1 means the
// header row. Return the styled string; do not change the visible width.
type CellStyler func(row, col int, content string) string

// Table renders a borderless table with two-space gutters. Column widths
// are computed using lipgloss.Width so ANSI codes don't break alignment.
//
// hdrStyle is applied to header cells uniformly. cellStyler may be nil to
// emit unstyled cells.
func Table(w io.Writer, headers []string, rows [][]string, hdrStyle lipgloss.Style, cellStyler CellStyler) error {
	cols := len(headers)
	if cols == 0 {
		return nil
	}
	widths := make([]int, cols)
	for i, h := range headers {
		widths[i] = lipgloss.Width(h)
	}
	for _, row := range rows {
		for i := 0; i < cols && i < len(row); i++ {
			if w := lipgloss.Width(row[i]); w > widths[i] {
				widths[i] = w
			}
		}
	}

	var sb strings.Builder
	for i, h := range headers {
		sb.WriteString(hdrStyle.Render(h))
		if i < cols-1 {
			sb.WriteString(strings.Repeat(" ", widths[i]-lipgloss.Width(h)+2))
		}
	}
	sb.WriteRune('\n')

	for r, row := range rows {
		for i := 0; i < cols; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			styled := cell
			if cellStyler != nil {
				styled = cellStyler(r, i, cell)
			}
			sb.WriteString(styled)
			if i < cols-1 {
				sb.WriteString(strings.Repeat(" ", widths[i]-lipgloss.Width(cell)+2))
			}
		}
		sb.WriteRune('\n')
	}

	_, err := io.WriteString(w, sb.String())
	return err
}
