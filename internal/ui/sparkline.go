package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// tail returns the last n elements of s. If s is shorter, returns s as-is.
// Used to clip a rolling history buffer to a fixed display width.
func tail(s []float64, n int) []float64 {
	if n <= 0 {
		return nil
	}
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// sparkline returns a Unicode-block sparkline for the given samples.
// The output is exactly len(samples) cells wide.
func sparkline(samples []float64) string {
	if len(samples) == 0 {
		return ""
	}
	chars := []rune("▁▂▃▄▅▆▇█")
	max := 0.0
	for _, v := range samples {
		if v > max {
			max = v
		}
	}
	if max == 0 {
		// Flat zero — show the lowest band.
		return strings.Repeat(string(chars[0]), len(samples))
	}
	var b strings.Builder
	b.Grow(len(samples) * 3)
	for _, v := range samples {
		idx := int(v / max * float64(len(chars)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(chars) {
			idx = len(chars) - 1
		}
		b.WriteRune(chars[idx])
	}
	return b.String()
}

// bar renders a horizontal bar of the given width. Filled cells are "|";
// empty cells are spaces. pct is clamped to [0, 100].
func bar(pct float64, width int) string {
	if width <= 0 {
		return ""
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	full := int(pct / 100.0 * float64(width))
	if full > width {
		full = width
	}
	return strings.Repeat("|", full) + strings.Repeat(" ", width-full)
}

// barSegment is one coloured slice of a multi-segment bar.
type barSegment struct {
	pct   float64 // share of the bar, 0–100
	color lipgloss.TerminalColor
}

// storageColorBar renders a single-coloured bar with thresholds tuned for
// disk usage: green ≤ 80%, yellow ≤ 95%, red above.
func storageColorBar(pct float64, width int) string {
	b := bar(pct, width)
	style := lipgloss.NewStyle()
	switch {
	case pct >= 95:
		style = style.Foreground(colCrashed).Bold(true)
	case pct >= 80:
		style = style.Foreground(colPaused)
	default:
		style = style.Foreground(colRunning)
	}
	return style.Render(b)
}

// multiBar renders a multi-segment bar using "|" characters in the
// segment colours. Empty cells become spaces. Total of segment pcts
// should be ≤ 100; anything over is clamped at the edge.
func multiBar(segments []barSegment, width int) string {
	if width <= 0 {
		return ""
	}
	var b strings.Builder
	used := 0
	for _, seg := range segments {
		if seg.pct <= 0 {
			continue
		}
		if seg.pct > 100 {
			seg.pct = 100
		}
		cells := int(seg.pct / 100.0 * float64(width))
		if used+cells > width {
			cells = width - used
		}
		if cells <= 0 {
			continue
		}
		style := lipgloss.NewStyle().Foreground(seg.color)
		b.WriteString(style.Render(strings.Repeat("|", cells)))
		used += cells
	}
	if used < width {
		b.WriteString(strings.Repeat(" ", width-used))
	}
	return b.String()
}
