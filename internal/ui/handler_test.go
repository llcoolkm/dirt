package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/llcoolkm/dirt/internal/lv"
)

// keyMsg crafts a tea.KeyMsg from a key string the way Bubble Tea
// itself reports them. Used by handler tests to drive Update.
func keyMsg(s string) tea.KeyMsg {
	switch s {
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// fakeModel returns a minimum-viable Model with a handful of domains.
// Helpers that take *Model can be called against a pointer.
func fakeModel(names ...string) Model {
	doms := make([]lv.Domain, len(names))
	for i, n := range names {
		doms[i] = lv.Domain{Name: n, UUID: "uuid-" + n, State: lv.StateRunning}
	}
	return Model{
		snap:          &lv.Snapshot{Domains: doms},
		marks:         make(map[string]bool),
		activeColumns: vmColumns,
		sortColumn:    sortByName,
	}
}

func TestSpaceTogglesAndAdvances(t *testing.T) {
	m := fakeModel("a", "b", "c")
	model, _ := m.handleNormalKey(keyMsg(" "))
	m = model.(Model)
	if !m.isMarked("uuid-a") {
		t.Error("expected uuid-a marked after SPACE")
	}
	if m.selected != 1 {
		t.Errorf("selected=%d, want 1 (advanced)", m.selected)
	}
}

func TestSpaceWithCountMarksMultiple(t *testing.T) {
	m := fakeModel("a", "b", "c", "d")
	// Build a count of 3.
	model, _ := m.handleNormalKey(keyMsg("3"))
	m = model.(Model)
	if m.pendingCount != 3 {
		t.Fatalf("pendingCount=%d after '3', want 3", m.pendingCount)
	}
	model, _ = m.handleNormalKey(keyMsg(" "))
	m = model.(Model)
	if m.markCount() != 3 {
		t.Errorf("markCount=%d, want 3", m.markCount())
	}
	if m.selected != 3 {
		t.Errorf("selected=%d, want 3 (advanced 3 times)", m.selected)
	}
}

func TestEscClearsLayered(t *testing.T) {
	m := fakeModel("a", "b")
	m.pendingCount = 5
	m.toggleMark("uuid-a")
	m.filter = "x"

	// First Esc clears count.
	model, _ := m.handleNormalKey(keyMsg("esc"))
	m = model.(Model)
	if m.pendingCount != 0 {
		t.Errorf("first Esc should clear count, got %d", m.pendingCount)
	}
	if m.markCount() != 1 {
		t.Errorf("first Esc should leave mark intact, got count %d", m.markCount())
	}

	// Second Esc clears marks.
	model, _ = m.handleNormalKey(keyMsg("esc"))
	m = model.(Model)
	if m.markCount() != 0 {
		t.Errorf("second Esc should clear marks, got count %d", m.markCount())
	}
	if m.filter != "x" {
		t.Errorf("second Esc should leave filter intact, got %q", m.filter)
	}

	// Third Esc clears filter.
	model, _ = m.handleNormalKey(keyMsg("esc"))
	m = model.(Model)
	if m.filter != "" {
		t.Errorf("third Esc should clear filter, got %q", m.filter)
	}
}

func TestJKMotionsRespectCount(t *testing.T) {
	m := fakeModel("a", "b", "c", "d", "e")
	m.selected = 0

	// 3j → selected = 3
	m.pendingCount = 3
	model, _ := m.handleNormalKey(keyMsg("j"))
	m = model.(Model)
	if m.selected != 3 {
		t.Errorf("3j: selected=%d, want 3", m.selected)
	}
	if m.lastDir != +1 {
		t.Errorf("3j: lastDir=%d, want +1", m.lastDir)
	}
	if m.pendingCount != 0 {
		t.Errorf("3j: count not consumed (%d)", m.pendingCount)
	}

	// 2k → selected = 1
	m.pendingCount = 2
	model, _ = m.handleNormalKey(keyMsg("k"))
	m = model.(Model)
	if m.selected != 1 {
		t.Errorf("2k: selected=%d, want 1", m.selected)
	}
	if m.lastDir != -1 {
		t.Errorf("2k: lastDir=%d, want -1", m.lastDir)
	}
}

func TestGWithCountJumpsToRow(t *testing.T) {
	m := fakeModel("a", "b", "c", "d", "e")
	m.pendingCount = 3
	model, _ := m.handleNormalKey(keyMsg("G"))
	m = model.(Model)
	// 1-indexed row 3 → selected = 2.
	if m.selected != 2 {
		t.Errorf("3G: selected=%d, want 2", m.selected)
	}
}

func TestGWithoutCountJumpsToEnd(t *testing.T) {
	m := fakeModel("a", "b", "c")
	model, _ := m.handleNormalKey(keyMsg("G"))
	m = model.(Model)
	if m.selected != 2 {
		t.Errorf("G: selected=%d, want 2 (end)", m.selected)
	}
	if m.lastDir != -1 {
		t.Errorf("G: lastDir=%d, want -1 (only up makes sense from end)", m.lastDir)
	}
}

func TestSpaceDirectionFollowsLastMotion(t *testing.T) {
	m := fakeModel("a", "b", "c", "d")
	m.selected = 3 // at the bottom
	// Press k → lastDir = -1.
	model, _ := m.handleNormalKey(keyMsg("k"))
	m = model.(Model)
	if m.lastDir != -1 {
		t.Fatalf("k: lastDir=%d, want -1", m.lastDir)
	}
	// Now SPACE should mark and advance up.
	model, _ = m.handleNormalKey(keyMsg(" "))
	m = model.(Model)
	if m.selected != 1 {
		t.Errorf("SPACE after k: selected=%d, want 1 (advanced up)", m.selected)
	}
}

func TestExecMarkCommandAll(t *testing.T) {
	m := fakeModel("a", "b", "c")
	m = m.execMarkCommand("mark all")
	if m.markCount() != 3 {
		t.Errorf(":mark all: markCount=%d, want 3", m.markCount())
	}
}

func TestExecMarkCommandNone(t *testing.T) {
	m := fakeModel("a", "b")
	m.toggleMark("uuid-a")
	m = m.execMarkCommand("mark none")
	if m.markCount() != 0 {
		t.Errorf(":mark none: markCount=%d, want 0", m.markCount())
	}
}

func TestExecSortCommand(t *testing.T) {
	m := fakeModel("a", "b")
	m = m.execSortCommand("cpu desc")
	if m.sortColumn != sortByCPU {
		t.Errorf("sortColumn=%v, want sortByCPU", m.sortColumn)
	}
	if !m.sortDesc {
		t.Error("expected sortDesc=true after 'cpu desc'")
	}
	m = m.execSortCommand("name")
	if m.sortColumn != sortByName {
		t.Errorf("sortColumn=%v, want sortByName", m.sortColumn)
	}
	if m.sortDesc {
		t.Error("expected sortDesc=false after bare 'name'")
	}
}

func TestExecSortCommandUnknownIDIsRejected(t *testing.T) {
	m := fakeModel("a", "b")
	m.sortColumn = sortByName
	m = m.execSortCommand("garbage")
	if m.sortColumn != sortByName {
		t.Error("unknown sort id should not change sortColumn")
	}
}

func TestExecGroupCommand(t *testing.T) {
	m := fakeModel("a", "b")
	m = m.execGroupCommand("os")
	if m.groupBy != "os" {
		t.Errorf("groupBy=%q, want 'os'", m.groupBy)
	}
	m = m.execGroupCommand("none")
	if m.groupBy != "" {
		t.Errorf(":group none: groupBy=%q, want empty", m.groupBy)
	}
}
