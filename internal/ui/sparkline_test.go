package ui

import (
	"strings"
	"testing"
)

func TestSparkline(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if got := sparkline(nil); got != "" {
			t.Errorf("sparkline(nil) = %q, want empty", got)
		}
	})

	t.Run("flat zero", func(t *testing.T) {
		got := sparkline([]float64{0, 0, 0, 0})
		// Each rune is 3 bytes (▁), so 4 cells = 12 bytes.
		if cells := countRunes(got); cells != 4 {
			t.Errorf("sparkline 4 zeros = %q (cells=%d), want 4 cells", got, cells)
		}
	})

	t.Run("ramp", func(t *testing.T) {
		got := sparkline([]float64{1, 2, 3, 4, 5, 6, 7, 8})
		if cells := countRunes(got); cells != 8 {
			t.Errorf("sparkline 8 vals = %q (cells=%d), want 8 cells", got, cells)
		}
		// First should be the lowest band, last the highest.
		runes := []rune(got)
		if runes[0] == runes[len(runes)-1] {
			t.Errorf("sparkline ramp produced flat output: %q", got)
		}
	})
}

func TestBar(t *testing.T) {
	cases := []struct {
		pct      float64
		width    int
		wantFill int // count of "|" chars
	}{
		{0, 10, 0},
		{50, 10, 5},
		{100, 10, 10},
		{75, 8, 6},
		{-5, 10, 0},   // clamps to 0
		{150, 10, 10}, // clamps to 100
	}
	for _, c := range cases {
		got := bar(c.pct, c.width)
		fill := strings.Count(got, "|")
		if fill != c.wantFill {
			t.Errorf("bar(%g, %d) fill=%d, want %d (got %q)",
				c.pct, c.width, fill, c.wantFill, got)
		}
		// Total visible cells should equal width.
		visible := strings.Count(got, "|") + strings.Count(got, " ")
		if visible != c.width {
			t.Errorf("bar(%g, %d) visible=%d, want %d", c.pct, c.width, visible, c.width)
		}
	}
}

func TestMultiBar(t *testing.T) {
	t.Run("two segments", func(t *testing.T) {
		got := multiBar([]barSegment{
			{pct: 30, color: colMemUsed},
			{pct: 20, color: colMemCache},
		}, 10)
		// 3 used cells + 2 cache cells + 5 free cells.
		// We strip ansi to count visible chars.
		visible := stripANSIBytes(got)
		if want := 10; len([]rune(visible)) != want {
			t.Errorf("multiBar = %q (visible %d runes), want %d runes", visible, len([]rune(visible)), want)
		}
	})

	t.Run("zero width", func(t *testing.T) {
		if got := multiBar([]barSegment{{pct: 50, color: colMemUsed}}, 0); got != "" {
			t.Errorf("multiBar zero width = %q, want empty", got)
		}
	})

	t.Run("over 100 clamps", func(t *testing.T) {
		got := multiBar([]barSegment{
			{pct: 80, color: colMemUsed},
			{pct: 80, color: colMemCache}, // would push past 100%
		}, 10)
		visible := stripANSIBytes(got)
		// Should still be exactly 10 visible cells.
		if rcount := len([]rune(visible)); rcount != 10 {
			t.Errorf("multiBar overflow = %q (%d runes), want 10", visible, rcount)
		}
	})
}

// countRunes returns the number of runes in s, useful for cell-counting strings
// containing multibyte block characters.
func countRunes(s string) int { return len([]rune(s)) }

// stripANSIBytes removes ANSI escape sequences for tests.
func stripANSIBytes(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		if r == 0x1b {
			in = true
			continue
		}
		if in {
			if r == 'm' {
				in = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
