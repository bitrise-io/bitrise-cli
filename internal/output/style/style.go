// Package style holds the lipgloss-backed semantic styles used by the
// human renderers. JSON output never goes through this package — ANSI
// codes must not leak into machine-readable output.
//
// Each renderer constructs Styles via New(writer). The returned bundle is
// scoped to that writer's color profile, so non-TTY writers (pipes,
// tests using *bytes.Buffer) automatically produce ANSI-free output.
// NO_COLOR and FORCE_COLOR are honored automatically by the underlying
// termenv detection. The Configure function adds explicit overrides for
// the --no-color and --theme flags.
package style

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/termenv"
)

// Theme controls which side of the AdaptiveColor pairs is selected when
// the renderer is asked for a color. Auto leaves the choice to lipgloss
// (which queries the terminal background via OSC 11). Dark/Light force
// the corresponding side. None disables ANSI altogether — handy for
// terminals where neither preset looks right.
type Theme string

const (
	ThemeAuto  Theme = "auto"
	ThemeDark  Theme = "dark"
	ThemeLight Theme = "light"
	ThemeNone  Theme = "none"
)

// Themes is the registered list of theme values, used for validation,
// help text, and shell completion.
var Themes = []string{string(ThemeAuto), string(ThemeDark), string(ThemeLight), string(ThemeNone)}

// ParseTheme validates a user-supplied --theme value. Empty resolves to
// Auto so callers can pass cmd flag values directly.
func ParseTheme(s string) (Theme, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", string(ThemeAuto):
		return ThemeAuto, nil
	case string(ThemeDark):
		return ThemeDark, nil
	case string(ThemeLight):
		return ThemeLight, nil
	case string(ThemeNone):
		return ThemeNone, nil
	default:
		return "", fmt.Errorf("unknown theme %q (expected: %s)", s, strings.Join(Themes, ", "))
	}
}

// 256-color palette, paired by terminal background brightness.
//
// Each AdaptiveColor pairs a color tuned for dark terminals with one
// tuned for light terminals; lipgloss/termenv detect the terminal's
// background via the OSC 11 query and pick the appropriate side. On
// terminals that don't answer OSC 11 lipgloss falls back to "Dark",
// which works for the common case (most dev terminals are dark) and
// can be overridden later by an explicit --theme flag if we add one.
//
// Lipgloss falls back to the nearest ANSI-16 color when the terminal
// can't render 256-color, so these values are safe even on legacy
// terminals.
//
// Picking values: the dark side is bright/saturated to pop on a black
// background; the light side is darker/muted to stay readable on white
// without going to pure black (which fights the body text).
var (
	dimColor     = lipgloss.AdaptiveColor{Light: "240", Dark: "245"} // grey
	successColor = lipgloss.AdaptiveColor{Light: "28", Dark: "42"}   // green
	failedColor  = lipgloss.AdaptiveColor{Light: "124", Dark: "196"} // red
	runningColor = lipgloss.AdaptiveColor{Light: "27", Dark: "33"}   // blue
	abortedColor = lipgloss.AdaptiveColor{Light: "166", Dark: "214"} // amber
	warnColor    = lipgloss.AdaptiveColor{Light: "136", Dark: "220"} // yellow / olive
)

// BrandColor is the Bitrise brand purple. Unlike the AdaptiveColor pairs above
// it's a single truecolor value (lipgloss downsamples to 256/16-color when the
// terminal can't render it), used for accents like the build-watch spinner and
// the interactive picker's cursor.
const BrandColor = lipgloss.Color("#7B61FF")

// forceNoColor and forcedTheme are set by Configure. They override the
// renderer's auto-detection on every Styles bundle constructed after
// Configure runs. Concurrent New() calls during a single command run see
// a stable value because Configure is invoked once in persistentPreRun
// before any subcommand's RunE is called.
var (
	forceNoColor bool
	forcedTheme  = ThemeAuto
)

// Configure applies process-wide style settings. Call once from the cmd
// layer's persistentPreRun, after parsing the --no-color and --theme flags.
func Configure(noColor bool, theme Theme) {
	forceNoColor = noColor
	forcedTheme = theme
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
	Brand   lipgloss.Style // brand-purple accent (picker cursor, etc.)

	statusSuccess    lipgloss.Style
	statusFailed     lipgloss.Style
	statusAborted    lipgloss.Style
	statusInProgress lipgloss.Style
	statusUnknown    lipgloss.Style
}

// New returns a Styles bundle for the given writer. ANSI escape codes
// are emitted only if the writer is detected as a color-capable TTY,
// --no-color is not in effect, and --theme is not "none". The theme
// (when not Auto) overrides the renderer's auto-detected light/dark
// background, forcing AdaptiveColor pairs to pick the requested side.
func New(w io.Writer) Styles {
	r := lipgloss.NewRenderer(w)
	if forceNoColor || forcedTheme == ThemeNone {
		r.SetColorProfile(termenv.Ascii)
	} else {
		switch forcedTheme {
		case ThemeDark:
			r.SetHasDarkBackground(true)
		case ThemeLight:
			r.SetHasDarkBackground(false)
		}
	}
	return Styles{
		r:                r,
		Header:           r.NewStyle().Bold(true).Foreground(dimColor),
		Dim:              r.NewStyle().Foreground(dimColor),
		Bold:             r.NewStyle().Bold(true),
		Label:            r.NewStyle().Bold(true),
		Slug:             r.NewStyle().Foreground(dimColor),
		URL:              r.NewStyle().Underline(true),
		Success:          r.NewStyle().Foreground(successColor),
		Failure:          r.NewStyle().Foreground(failedColor),
		Warn:             r.NewStyle().Foreground(warnColor),
		Brand:            r.NewStyle().Foreground(BrandColor),
		statusSuccess:    r.NewStyle().Foreground(successColor),
		statusFailed:     r.NewStyle().Foreground(failedColor),
		statusAborted:    r.NewStyle().Foreground(abortedColor),
		statusInProgress: r.NewStyle().Foreground(runningColor),
		statusUnknown:    r.NewStyle().Foreground(dimColor),
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

// Rainbow returns msg with each visible rune colored along the HSV
// spectrum. Whitespace stays unstyled. hueOffsetDeg shifts the start of
// the spectrum (0-360); incrementing it across animation frames produces
// a smooth shimmer. When color is disabled (--no-color, --theme=none, or
// the writer isn't a TTY) the plain string is returned unchanged.
func (s Styles) Rainbow(msg string, hueOffsetDeg float64) string {
	if !s.HasColor() {
		return msg
	}
	runes := []rune(msg)
	visibleCount := 0
	for _, r := range runes {
		if !isWhitespace(r) {
			visibleCount++
		}
	}
	if visibleCount == 0 {
		return msg
	}

	// Each non-whitespace rune gets a slice of the spectrum. Saturation
	// 0.85 / value 0.95 keeps the colors bright on dark terminals while
	// staying just-barely-readable on light ones; the rainbow theme
	// trades perfect contrast for visible variety.
	const sat, val = 0.85, 0.95

	var b strings.Builder
	b.Grow(len(msg) * 12) // ANSI per-rune envelope is ~10–15 bytes

	visible := 0
	for _, r := range runes {
		if isWhitespace(r) {
			b.WriteRune(r)
			continue
		}
		h := math.Mod(hueOffsetDeg+360.0*float64(visible)/float64(visibleCount), 360.0)
		if h < 0 {
			h += 360
		}
		hex := colorful.Hsv(h, sat, val).Hex()
		b.WriteString(s.r.NewStyle().Foreground(lipgloss.Color(hex)).Render(string(r)))
		visible++
	}
	return b.String()
}

func isWhitespace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r':
		return true
	}
	return false
}

// CellStyler renders a single cell content string. Only called for data rows
// (row >= 0); header cells are styled uniformly by Table using hdrStyle.
// Return the styled string; do not change the visible width.
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
