package ui

import (
	"strings"
	"testing"
)

func columnsFixture() Model {
	return Model{
		mode:          viewColumns,
		activeColumns: vmColumns,
	}
}

func TestColumnsViewListsEveryColumn(t *testing.T) {
	m := columnsFixture()
	m.width = 200
	m.height = 40
	out := stripANSI(m.columnsView())
	for _, c := range vmColumns {
		if !strings.Contains(out, c.id) {
			t.Errorf("columns view missing id %q", c.id)
		}
	}
}

func TestCurrentColumnVisibilityReflectsActive(t *testing.T) {
	// Hide cpu and uptime, leave the rest.
	visibility := make(map[string]bool, len(vmColumns))
	for _, c := range vmColumns {
		visibility[c.id] = true
	}
	visibility["cpu"] = false
	visibility["uptime"] = false

	m := Model{activeColumns: filterActiveColumns(vmColumns, visibility)}
	got := m.currentColumnVisibility()
	if got["cpu"] {
		t.Error("cpu should be hidden")
	}
	if got["uptime"] {
		t.Error("uptime should be hidden")
	}
	if !got["name"] {
		t.Error("name should be visible (required)")
	}
}

func TestColumnsToggleSpace(t *testing.T) {
	m := columnsFixture()
	// Move to "cpu" — it's a non-required column.
	for i, c := range vmColumns {
		if c.id == "cpu" {
			m.columnsSel = i
			break
		}
	}
	model, _ := m.handleColumnsKey(keyMsg(" "))
	m = model.(Model)
	vis := m.currentColumnVisibility()
	if vis["cpu"] {
		t.Error("first SPACE should hide cpu")
	}
	model, _ = m.handleColumnsKey(keyMsg(" "))
	m = model.(Model)
	vis = m.currentColumnVisibility()
	if !vis["cpu"] {
		t.Error("second SPACE should re-show cpu")
	}
}

func TestColumnsToggleRequiredRefuses(t *testing.T) {
	m := columnsFixture()
	// Cursor on "name" (required, index 0).
	m.columnsSel = 0
	model, _ := m.handleColumnsKey(keyMsg(" "))
	m = model.(Model)
	if !m.currentColumnVisibility()["name"] {
		t.Error("required column should refuse to hide")
	}
	if m.flash == "" {
		t.Error("expected a flash explaining the refusal")
	}
}

func TestColumnsAllShowsEverything(t *testing.T) {
	m := columnsFixture()
	// Hide cpu first.
	vis := make(map[string]bool, len(vmColumns))
	for _, c := range vmColumns {
		vis[c.id] = true
	}
	vis["cpu"] = false
	m.activeColumns = filterActiveColumns(vmColumns, vis)

	model, _ := m.handleColumnsKey(keyMsg("a"))
	m = model.(Model)
	for _, c := range vmColumns {
		if !m.currentColumnVisibility()[c.id] {
			t.Errorf("'a' should show every column; %q still hidden", c.id)
		}
	}
}

func TestColumnsNoneHidesAllNonRequired(t *testing.T) {
	m := columnsFixture()
	model, _ := m.handleColumnsKey(keyMsg("n"))
	m = model.(Model)
	vis := m.currentColumnVisibility()
	for _, c := range vmColumns {
		if c.required && !vis[c.id] {
			t.Errorf("required %q should remain visible", c.id)
		}
		if !c.required && vis[c.id] {
			t.Errorf("non-required %q should be hidden after 'n'", c.id)
		}
	}
}

func TestColumnsEscReturnsToMain(t *testing.T) {
	m := columnsFixture()
	model, _ := m.handleColumnsKey(keyMsg("esc"))
	m = model.(Model)
	if m.mode != viewMain {
		t.Errorf("esc should return to viewMain, got %v", m.mode)
	}
}

func TestColumnsNavBounds(t *testing.T) {
	m := columnsFixture()
	m.columnsSel = 0
	// k at top should not go negative.
	model, _ := m.handleColumnsKey(keyMsg("k"))
	m = model.(Model)
	if m.columnsSel != 0 {
		t.Errorf("k at top: sel=%d, want 0", m.columnsSel)
	}
	// Walk to the end with G.
	model, _ = m.handleColumnsKey(keyMsg("G"))
	m = model.(Model)
	if m.columnsSel != len(vmColumns)-1 {
		t.Errorf("G: sel=%d, want %d", m.columnsSel, len(vmColumns)-1)
	}
	// j past the end should not overflow.
	model, _ = m.handleColumnsKey(keyMsg("j"))
	m = model.(Model)
	if m.columnsSel != len(vmColumns)-1 {
		t.Errorf("j at end: sel=%d, want %d", m.columnsSel, len(vmColumns)-1)
	}
}
