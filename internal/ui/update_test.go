package ui

import (
	"testing"

	"github.com/llcoolkm/dirt/internal/lv"
)

// updateFixture spins up a Model that's enough for Update() and
// View() to run without touching the network or libvirt.
func updateFixture() Model {
	return Model{
		snap: &lv.Snapshot{Domains: []lv.Domain{
			{Name: "alpha", UUID: "ua", State: lv.StateRunning},
		}},
		marks:         make(map[string]bool),
		activeColumns: vmColumns,
		sortColumn:    sortByName,
		width:         200,
		height:        40,
		history:       map[string]*domHistory{},
		guestUptime:   map[string]lv.GuestUptime{},
	}
}

func TestUpdateRoutesQuit(t *testing.T) {
	m := updateFixture()
	model, cmd := m.Update(keyMsg("q"))
	if model == nil {
		t.Fatal("Update returned nil model")
	}
	if cmd == nil {
		t.Error("'q' should return a Quit command")
	}
}

func TestUpdateRoutesHelpToggle(t *testing.T) {
	m := updateFixture()
	model, _ := m.Update(keyMsg("?"))
	m = model.(Model)
	if m.mode != viewHelp {
		t.Errorf("? should open help, mode=%v", m.mode)
	}
	model, _ = m.Update(keyMsg("?"))
	m = model.(Model)
	if m.mode == viewHelp {
		t.Error("? should toggle out of help on second press")
	}
}

func TestUpdateRoutesColonToCommanding(t *testing.T) {
	m := updateFixture()
	model, _ := m.Update(keyMsg(":"))
	m = model.(Model)
	if !m.commanding {
		t.Error(": should enter command palette mode")
	}
}

func TestUpdateRoutesSlashToFiltering(t *testing.T) {
	m := updateFixture()
	model, _ := m.Update(keyMsg("/"))
	m = model.(Model)
	if !m.filtering {
		t.Error("/ should enter filter mode")
	}
}

func TestUpdateConfirmingShortcircuitsNormalKeys(t *testing.T) {
	m := updateFixture()
	m.confirming = true
	m.confirmAction = "shutdown"
	m.confirmName = "alpha"
	// Sending 'q' while confirming should NOT quit — the confirm
	// handler treats any non-y key as cancel and consumes it.
	model, _ := m.Update(keyMsg("q"))
	m = model.(Model)
	if m.confirming {
		t.Error("q during confirm should cancel, not pass through")
	}
}

func TestUpdateFilterAccumulatesAndClearsOnEsc(t *testing.T) {
	m := updateFixture()
	m.filtering = true
	for _, ch := range "alp" {
		model, _ := m.Update(keyMsg(string(ch)))
		m = model.(Model)
	}
	if m.filter != "alp" {
		t.Errorf("filter=%q, want 'alp'", m.filter)
	}
	model, _ := m.Update(keyMsg("esc"))
	m = model.(Model)
	if m.filtering {
		t.Error("esc should leave filter mode")
	}
}

func TestUpdateCommandingTabCompletes(t *testing.T) {
	m := updateFixture()
	m.commanding = true
	m.command = "the"
	model, _ := m.Update(keyMsg("tab"))
	m = model.(Model)
	if m.command != "theme" {
		t.Errorf("Tab should complete 'the' → 'theme', got %q", m.command)
	}
}

func TestUpdateCommandingEnterRunsExec(t *testing.T) {
	m := updateFixture()
	m.commanding = true
	m.command = "mark all"
	model, _ := m.Update(keyMsg("enter"))
	m = model.(Model)
	if m.commanding {
		t.Error("enter should leave commanding mode")
	}
	if m.markCount() != 1 {
		t.Errorf(":mark all should mark 1 visible domain, got %d", m.markCount())
	}
}

// TestUpdateWindowSizeMsgIsAccepted is reserved for a future addition
// — tea.WindowSizeMsg drives layout and is worth covering, but the
// existing render tests already exercise the layout paths with
// explicit width / height fields, so we don't duplicate here.

func TestViewSurvivesWithNoSnap(t *testing.T) {
	m := Model{width: 80, height: 24}
	out := m.View()
	if out == "" {
		t.Error("View() with no snap should still render something (splash)")
	}
}

func TestViewWithErrorShowsError(t *testing.T) {
	m := Model{
		err:    errFake("connection refused"),
		width:  80,
		height: 24,
	}
	out := stripANSI(m.View())
	if !contains(out, "connection refused") {
		t.Errorf("error should appear in View output, got:\n%s", out)
	}
}

// contains is a tiny helper because strings.Contains lives in
// strings — we keep the imports here lean for one assertion.
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
