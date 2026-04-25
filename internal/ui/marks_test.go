package ui

import (
	"testing"

	"github.com/llcoolkm/dirt/internal/lv"
)

func newMarkModel(domains ...lv.Domain) *Model {
	m := &Model{
		marks: make(map[string]bool),
		snap:  &lv.Snapshot{Domains: domains},
	}
	return m
}

func TestToggleMark(t *testing.T) {
	m := newMarkModel()
	m.toggleMark("uuid-1")
	if !m.isMarked("uuid-1") {
		t.Fatal("expected uuid-1 marked after first toggle")
	}
	if m.markCount() != 1 {
		t.Errorf("markCount=%d, want 1", m.markCount())
	}
	m.toggleMark("uuid-1")
	if m.isMarked("uuid-1") {
		t.Fatal("expected uuid-1 unmarked after second toggle")
	}
	if m.markCount() != 0 {
		t.Errorf("markCount=%d, want 0", m.markCount())
	}
}

func TestToggleMarkEmptyUUIDIsNoOp(t *testing.T) {
	m := newMarkModel()
	m.toggleMark("")
	if m.markCount() != 0 {
		t.Fatalf("empty uuid should not produce a mark, got %d", m.markCount())
	}
}

func TestClearMarks(t *testing.T) {
	m := newMarkModel()
	m.toggleMark("a")
	m.toggleMark("b")
	m.clearMarks()
	if m.markCount() != 0 {
		t.Fatalf("clearMarks left %d entries", m.markCount())
	}
}

func TestMarkAllVisibleAndInvert(t *testing.T) {
	m := newMarkModel(
		lv.Domain{Name: "a", UUID: "ua"},
		lv.Domain{Name: "b", UUID: "ub"},
		lv.Domain{Name: "c", UUID: "uc"},
	)
	m.markAllVisible()
	if m.markCount() != 3 {
		t.Fatalf("markAllVisible: got %d, want 3", m.markCount())
	}
	m.invertMarksVisible()
	if m.markCount() != 0 {
		t.Fatalf("invertMarksVisible after all-marked: got %d, want 0", m.markCount())
	}
	// Mark just one, then invert — should hit the other two.
	m.toggleMark("ua")
	m.invertMarksVisible()
	if m.markCount() != 2 {
		t.Fatalf("invert with 1 marked: got %d, want 2", m.markCount())
	}
	if m.isMarked("ua") {
		t.Error("ua should be unmarked after invert")
	}
	if !m.isMarked("ub") || !m.isMarked("uc") {
		t.Error("ub and uc should be marked after invert")
	}
}

func TestPruneMarksDropsVanished(t *testing.T) {
	m := newMarkModel(
		lv.Domain{Name: "a", UUID: "ua"},
		lv.Domain{Name: "b", UUID: "ub"},
	)
	m.toggleMark("ua")
	m.toggleMark("ub")
	m.toggleMark("ghost") // not in the snapshot
	m.pruneMarks()
	if m.markCount() != 2 {
		t.Errorf("pruneMarks: got %d, want 2 (ghost should drop)", m.markCount())
	}
	if m.isMarked("ghost") {
		t.Error("ghost mark should be pruned")
	}
}

func TestMarkedDomainsInStatesFilters(t *testing.T) {
	m := newMarkModel(
		lv.Domain{Name: "a", UUID: "ua", State: lv.StateRunning},
		lv.Domain{Name: "b", UUID: "ub", State: lv.StateShutoff},
		lv.Domain{Name: "c", UUID: "uc", State: lv.StateRunning},
	)
	m.toggleMark("ua")
	m.toggleMark("ub")
	m.toggleMark("uc")
	got := m.markedDomainsInStates(lv.StateRunning)
	if len(got) != 2 {
		t.Fatalf("running-only: got %d names, want 2", len(got))
	}
	if got[0] != "a" || got[1] != "c" {
		t.Errorf("running-only names: got %v, want [a c]", got)
	}
	stopped := m.markedDomainsInStates(lv.StateShutoff)
	if len(stopped) != 1 || stopped[0] != "b" {
		t.Errorf("shutoff-only names: got %v, want [b]", stopped)
	}
}

func TestMarksHiddenByFilterCounts(t *testing.T) {
	m := newMarkModel(
		lv.Domain{Name: "alpha", UUID: "ua"},
		lv.Domain{Name: "beta", UUID: "ub"},
		lv.Domain{Name: "gamma", UUID: "uc"},
	)
	m.toggleMark("ua")
	m.toggleMark("ub")
	m.toggleMark("uc")
	m.filter = "alpha" // matches only alpha; hides beta + gamma
	if got := m.marksHiddenByFilter(); got != 2 {
		t.Errorf("hidden=2, got %d", got)
	}
	m.filter = ""
	if got := m.marksHiddenByFilter(); got != 0 {
		t.Errorf("hidden=0 with no filter, got %d", got)
	}
}
