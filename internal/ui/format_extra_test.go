package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestFitColumnsKeepsRequiredEvenOnOverflow(t *testing.T) {
	// Force a width below the required-columns total — fitColumns
	// must still return at least the required count rather than
	// hiding critical fields.
	got := fitColumns(vmColumns, 1)
	required := 0
	for _, c := range vmColumns {
		if !c.required {
			break
		}
		required++
	}
	if got != required {
		t.Errorf("fitColumns(narrow)=%d, want required=%d", got, required)
	}
}

func TestFitColumnsFitsAllWhenWide(t *testing.T) {
	got := fitColumns(vmColumns, 10000)
	if got != len(vmColumns) {
		t.Errorf("fitColumns(wide)=%d, want %d", got, len(vmColumns))
	}
}

func TestColumnsWidthIsSumPlusSeparators(t *testing.T) {
	w := columnsWidth(vmColumns[:3])
	// 1 leading + sum(widths) + 2*(n-1) separators.
	want := 1 + colNameW + colStateW + colIPW + 2*2
	if w != want {
		t.Errorf("columnsWidth=%d, want %d", w, want)
	}
}

func TestColumnsWidthEmptyIsZero(t *testing.T) {
	if w := columnsWidth(nil); w != 0 {
		t.Errorf("empty columnsWidth=%d, want 0", w)
	}
}

func TestRuneBackspaceHandlesUnicode(t *testing.T) {
	// "ñ" is two bytes in UTF-8; runeBackspace must trim a whole
	// codepoint, not chop mid-byte.
	if got := runeBackspace("año"); got != "añ" {
		t.Errorf("runeBackspace(año)=%q, want 'añ'", got)
	}
	if got := runeBackspace(""); got != "" {
		t.Errorf("runeBackspace empty=%q, want empty", got)
	}
}

func TestTruncateAddsEllipsisOnOverflow(t *testing.T) {
	got := truncate("abcdefghij", 5)
	if lipgloss.Width(got) > 5 {
		t.Errorf("truncate width=%d, want ≤ 5 (got %q)", lipgloss.Width(got), got)
	}
	if got == "abcdefghij" {
		t.Error("expected truncation, got full string")
	}
}

func TestTruncateUnicodeWidthAware(t *testing.T) {
	// "ñoño" is 4 cells; truncate to 3 should land at 3 cells.
	got := truncate("ñoño", 3)
	if w := lipgloss.Width(got); w > 3 {
		t.Errorf("truncate(ñoño,3)=%q (width %d), want ≤ 3", got, w)
	}
}

func TestFilterSuffixFormat(t *testing.T) {
	if filterSuffix("") != "" {
		t.Error("empty filter should yield empty suffix")
	}
	got := filterSuffix("abc")
	if got == "" {
		t.Error("non-empty filter should yield a suffix")
	}
}

func TestPadLeftRightWidthAware(t *testing.T) {
	if got := padLeft("a", 4); lipgloss.Width(got) != 4 {
		t.Errorf("padLeft width=%d, want 4 (got %q)", lipgloss.Width(got), got)
	}
	if got := padRight("a", 4); lipgloss.Width(got) != 4 {
		t.Errorf("padRight width=%d, want 4 (got %q)", lipgloss.Width(got), got)
	}
	// String already at width — should be returned unchanged.
	if got := padRight("hello", 5); got != "hello" {
		t.Errorf("padRight at width: %q, want 'hello'", got)
	}
}

func TestNavSelectBoundsAndKeys(t *testing.T) {
	cases := []struct {
		key     string
		startSel int
		n        int
		wantSel  int
		wantOk   bool
	}{
		{"j", 0, 3, 1, true},
		{"j", 2, 3, 2, true}, // already at end, no further
		{"k", 1, 3, 0, true},
		{"k", 0, 3, 0, true}, // already at top
		{"g", 5, 10, 0, true},
		{"G", 0, 10, 9, true},
		{"q", 5, 10, 5, false}, // unrecognised key
	}
	for _, c := range cases {
		sel := c.startSel
		ok := navSelect(c.key, &sel, c.n)
		if ok != c.wantOk {
			t.Errorf("%s: ok=%v, want %v", c.key, ok, c.wantOk)
		}
		if sel != c.wantSel {
			t.Errorf("%s from %d: sel=%d, want %d", c.key, c.startSel, sel, c.wantSel)
		}
	}
}
