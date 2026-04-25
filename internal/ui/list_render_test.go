package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/llcoolkm/dirt/internal/lv"
)

// stripANSI removes lipgloss escape sequences so tests can assert on
// the visible text without colour codes mucking up the comparison.
func stripANSI(s string) string {
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

func renderModel(domains ...lv.Domain) string {
	m := Model{
		snap:          &lv.Snapshot{Domains: domains},
		marks:         make(map[string]bool),
		activeColumns: vmColumns,
		sortColumn:    sortByName,
		width:         200,
		height:        40,
		history:       map[string]*domHistory{},
		guestUptime:   map[string]lv.GuestUptime{},
	}
	return m.listView()
}

func TestListViewIncludesEveryDomainName(t *testing.T) {
	out := renderModel(
		lv.Domain{Name: "alpha", UUID: "ua", State: lv.StateRunning},
		lv.Domain{Name: "beta", UUID: "ub", State: lv.StateShutoff},
		lv.Domain{Name: "gamma", UUID: "uc", State: lv.StateRunning},
	)
	plain := stripANSI(out)
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(plain, name) {
			t.Errorf("rendered table missing %q\n%s", name, plain)
		}
	}
}

func TestListViewEmptyShowsHint(t *testing.T) {
	out := renderModel()
	plain := stripANSI(out)
	if !strings.Contains(plain, "no domains") {
		t.Errorf("expected 'no domains' hint, got:\n%s", plain)
	}
}

func TestListViewMarkGlyphAppearsForMarkedRow(t *testing.T) {
	m := Model{
		snap: &lv.Snapshot{Domains: []lv.Domain{
			{Name: "alpha", UUID: "ua", State: lv.StateRunning},
			{Name: "beta", UUID: "ub", State: lv.StateShutoff},
		}},
		marks:         map[string]bool{"ua": true},
		activeColumns: vmColumns,
		sortColumn:    sortByName,
		width:         200,
		height:        40,
		history:       map[string]*domHistory{},
		guestUptime:   map[string]lv.GuestUptime{},
	}
	out := stripANSI(m.listView())
	if !strings.Contains(out, "✓") {
		t.Errorf("expected ✓ glyph for marked row, got:\n%s", out)
	}
}

func TestListViewViewportIndicatorAppearsWhenScrolling(t *testing.T) {
	doms := make([]lv.Domain, 60)
	for i := range doms {
		doms[i] = lv.Domain{
			Name:  string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)),
			UUID:  "uuid-" + string(rune('a'+i%26)),
			State: lv.StateRunning,
		}
	}
	m := Model{
		snap:          &lv.Snapshot{Domains: doms},
		marks:         make(map[string]bool),
		activeColumns: vmColumns,
		sortColumn:    sortByName,
		width:         200,
		height:        20, // smaller than 60 rows
		history:       map[string]*domHistory{},
		guestUptime:   map[string]lv.GuestUptime{},
	}
	out := stripANSI(m.listView())
	// Indicator looks like "1–N/60" — assert the slash and total are present.
	if !strings.Contains(out, "/60") {
		t.Errorf("expected viewport indicator with /60 total, got:\n%s", out)
	}
}

func TestListViewGroupHeaderRendersCounts(t *testing.T) {
	m := Model{
		snap: &lv.Snapshot{Domains: []lv.Domain{
			{Name: "a", UUID: "ua", State: lv.StateRunning},
			{Name: "b", UUID: "ub", State: lv.StateRunning},
			{Name: "c", UUID: "uc", State: lv.StateShutoff},
		}},
		marks:         make(map[string]bool),
		activeColumns: vmColumns,
		sortColumn:    sortByName,
		groupBy:       "state",
		width:         200,
		height:        40,
		history:       map[string]*domHistory{},
		guestUptime:   map[string]lv.GuestUptime{},
	}
	out := stripANSI(m.listView())
	if !strings.Contains(out, "running") {
		t.Errorf("expected 'running' group header, got:\n%s", out)
	}
	if !strings.Contains(out, "shut off") {
		t.Errorf("expected 'shut off' group header, got:\n%s", out)
	}
	if !strings.Contains(out, "2 total") {
		t.Errorf("running header should show '2 total', got:\n%s", out)
	}
}

// TestListViewSurvivesAllThemes is a smoke test: render a small
// table under every theme and verify the output is non-empty and
// contains every name. Cheap regression catch for theme-coverage
// breakage.
func TestListViewSurvivesAllThemes(t *testing.T) {
	defer ApplyTheme("default")
	for _, theme := range themeNames() {
		t.Run(theme, func(t *testing.T) {
			ApplyTheme(theme)
			out := renderModel(
				lv.Domain{Name: "alpha", UUID: "ua", State: lv.StateRunning},
				lv.Domain{Name: "beta", UUID: "ub", State: lv.StateShutoff},
			)
			plain := stripANSI(out)
			for _, name := range []string{"alpha", "beta"} {
				if !strings.Contains(plain, name) {
					t.Errorf("[%s] missing %q in output", theme, name)
				}
			}
		})
	}
	_ = lipgloss.NewStyle()
}
