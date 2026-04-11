package ui

import "testing"

func TestClickedSubviewRow(t *testing.T) {
	cases := []struct {
		name    string
		y       int
		n       int
		wantIdx int
		wantOk  bool
	}{
		// Rows 0–3 are frame and column header — never clickable as data.
		{"top border", 0, 5, 0, false},
		{"title row", 1, 5, 0, false},
		{"blank row", 2, 5, 0, false},
		{"column header row", 3, 5, 0, false},

		// Row 4 is the first data row.
		{"first data row", 4, 5, 0, true},
		{"second data row", 5, 5, 1, true},
		{"last data row", 8, 5, 4, true},

		// Past the last row — ignore.
		{"row just past end", 9, 5, 0, false},
		{"far past end", 100, 5, 0, false},

		// Empty table — never clickable.
		{"empty table, first row", 4, 0, 0, false},
		{"empty table, top border", 0, 0, 0, false},
	}
	for _, tc := range cases {
		gotIdx, gotOk := clickedSubviewRow(tc.y, tc.n)
		if gotOk != tc.wantOk || (gotOk && gotIdx != tc.wantIdx) {
			t.Errorf("%s: clickedSubviewRow(%d, %d) = (%d, %v), want (%d, %v)",
				tc.name, tc.y, tc.n, gotIdx, gotOk, tc.wantIdx, tc.wantOk)
		}
	}
}

func TestHeaderPaneHeight(t *testing.T) {
	// Wide layout: side-by-side, one box of 9 rows.
	wide := Model{width: 200}
	if got := wide.headerPaneHeight(); got != 9 {
		t.Errorf("wide headerPaneHeight = %d, want 9", got)
	}

	// Just-below-threshold layout: stacked, 18 rows total.
	narrow := Model{width: sideBySideMinWidth - 1}
	if got := narrow.headerPaneHeight(); got != 18 {
		t.Errorf("narrow headerPaneHeight = %d, want 18", got)
	}

	// Zero width falls back to the default of 80, which is < sideBySideMinWidth,
	// so we expect the stacked layout.
	zero := Model{width: 0}
	if got := zero.headerPaneHeight(); got != 18 {
		t.Errorf("zero-width headerPaneHeight = %d, want 18", got)
	}
}
