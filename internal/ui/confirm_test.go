package ui

import (
	"testing"

	"github.com/llcoolkm/dirt/internal/lv"
)

// confirmFixture returns a Model whose action handlers won't panic
// because no real lv.Client work is reached — every test here only
// checks state transitions, not command execution.
func confirmFixture(t *testing.T) Model {
	t.Helper()
	return Model{
		snap: &lv.Snapshot{Domains: []lv.Domain{
			{Name: "alpha", UUID: "ua", State: lv.StateRunning},
			{Name: "beta", UUID: "ub", State: lv.StateRunning},
			{Name: "gamma", UUID: "uc", State: lv.StateShutoff},
		}},
		marks:         make(map[string]bool),
		activeColumns: vmColumns,
		sortColumn:    sortByName,
	}
}

func TestConfirmCancelClearsState(t *testing.T) {
	m := confirmFixture(t)
	m.confirming = true
	m.confirmAction = "shutdown"
	m.confirmName = "alpha"

	model, _ := m.handleConfirmKey(keyMsg("n"))
	m = model.(Model)
	if m.confirming {
		t.Error("any non-y key should cancel confirming")
	}
	if m.confirmAction != "" || m.confirmName != "" {
		t.Errorf("cancel should clear action/name, got %q/%q", m.confirmAction, m.confirmName)
	}
}

func TestConfirmEscCancels(t *testing.T) {
	m := confirmFixture(t)
	m.confirming = true
	m.confirmAction = "destroy"
	m.confirmName = "alpha"

	model, _ := m.handleConfirmKey(keyMsg("esc"))
	m = model.(Model)
	if m.confirming {
		t.Error("esc should cancel confirmation")
	}
}

func TestResetConfirmClearsAllFields(t *testing.T) {
	m := confirmFixture(t)
	m.confirming = true
	m.confirmAction = "destroy"
	m.confirmName = "alpha"
	m.confirmBulk = true
	m.confirmTargets = []string{"a", "b"}

	m.resetConfirm()

	if m.confirming || m.confirmAction != "" || m.confirmName != "" ||
		m.confirmBulk || m.confirmTargets != nil {
		t.Errorf("resetConfirm did not zero everything: %+v", m)
	}
}

func TestResetTypedConfirmClearsAllFields(t *testing.T) {
	m := confirmFixture(t)
	m.confirmTyping = true
	m.confirmTypingAction = "undefine"
	m.confirmTypingExpect = "undefine 47"
	m.confirmTypingInput = "undefine 4"
	m.confirmTargets = []string{"a"}

	m.resetTypedConfirm()

	if m.confirmTyping || m.confirmTypingAction != "" || m.confirmTypingExpect != "" ||
		m.confirmTypingInput != "" || m.confirmTargets != nil {
		t.Errorf("resetTypedConfirm did not zero everything: %+v", m)
	}
}

func TestTypedConfirmEscCancels(t *testing.T) {
	m := confirmFixture(t)
	m.confirmTyping = true
	m.confirmTypingAction = "undefine"
	m.confirmTypingExpect = "undefine 47"
	m.confirmTypingInput = "undefin"
	m.confirmTargets = []string{"a"}

	model, _ := m.handleTypedConfirmKey(keyMsg("esc"))
	m = model.(Model)

	if m.confirmTyping {
		t.Error("esc should clear confirmTyping")
	}
}

func TestTypedConfirmAccumulatesAndBackspaces(t *testing.T) {
	m := confirmFixture(t)
	m.confirmTyping = true
	m.confirmTypingExpect = "undefine 3"
	m.confirmTypingInput = ""

	for _, ch := range "und" {
		model, _ := m.handleTypedConfirmKey(keyMsg(string(ch)))
		m = model.(Model)
	}
	if m.confirmTypingInput != "und" {
		t.Errorf("after typing 'und': input=%q, want 'und'", m.confirmTypingInput)
	}

	// Now backspace once.
	model, _ := m.handleTypedConfirmKey(keyMsg("backspace"))
	m = model.(Model)
	if m.confirmTypingInput != "un" {
		t.Errorf("after backspace: input=%q, want 'un'", m.confirmTypingInput)
	}
}

func TestTypedConfirmMismatchCancels(t *testing.T) {
	m := confirmFixture(t)
	m.confirmTyping = true
	m.confirmTypingAction = "undefine"
	m.confirmTypingExpect = "undefine 3"
	m.confirmTypingInput = "wrong"
	m.confirmTargets = []string{"a", "b", "c"}

	model, _ := m.handleTypedConfirmKey(keyMsg("enter"))
	m = model.(Model)

	if m.confirmTyping {
		t.Error("phrase mismatch should clear confirmTyping")
	}
}

// TestStartActionWithoutMarksOnlyTargetsCursorRow is intentionally
// omitted — the `s` cursor-row path constructs an actionCmd whose
// constructor calls m.client.URI(), which would panic with a nil
// client. End-to-end command execution is exercised in integration
// tests, not here.

func TestShutdownSetsConfirmingForRunningVM(t *testing.T) {
	m := confirmFixture(t)
	m.selected = 0 // alpha (running)

	model, _ := m.handleNormalKey(keyMsg("S"))
	m = model.(Model)
	if !m.confirming {
		t.Error("S should request confirmation for a running VM")
	}
	if m.confirmAction != "shutdown" {
		t.Errorf("confirmAction=%q, want shutdown", m.confirmAction)
	}
	if m.confirmName != "alpha" {
		t.Errorf("confirmName=%q, want alpha", m.confirmName)
	}
}

func TestShutdownBulkRoutesViaMarks(t *testing.T) {
	m := confirmFixture(t)
	m.toggleMark("ua") // running
	m.toggleMark("ub") // running
	m.toggleMark("uc") // shutoff — should be filtered out

	model, _ := m.handleNormalKey(keyMsg("S"))
	m = model.(Model)
	if !m.confirming || !m.confirmBulk {
		t.Fatal("S with marks should set bulk confirmation")
	}
	if len(m.confirmTargets) != 2 {
		t.Errorf("targets=%v, want 2 running", m.confirmTargets)
	}
}

func TestUndefineBulkAboveCeilingRequiresTypedConfirm(t *testing.T) {
	m := confirmFixture(t)
	// Replace snap with > 20 stopped domains, all marked.
	m.snap = &lv.Snapshot{}
	m.marks = make(map[string]bool)
	for i := 0; i < bulkUndefineCeiling+5; i++ {
		uuid := "uuid-test-" + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26))
		m.snap.Domains = append(m.snap.Domains, lv.Domain{
			Name: uuid, UUID: uuid, State: lv.StateShutoff,
		})
		m.marks[uuid] = true
	}

	model, _ := m.handleNormalKey(keyMsg("U"))
	m = model.(Model)
	if !m.confirmTyping {
		t.Fatal("above ceiling, U should engage typed confirmation")
	}
	if m.confirmTypingAction != "undefine" {
		t.Errorf("confirmTypingAction=%q, want 'undefine'", m.confirmTypingAction)
	}
	if m.confirmTypingExpect == "" {
		t.Error("expected phrase should be set")
	}
}

func TestUndefineBulkBelowCeilingUsesNormalConfirm(t *testing.T) {
	m := confirmFixture(t)
	m.snap.Domains = []lv.Domain{
		{Name: "a", UUID: "ua", State: lv.StateShutoff},
		{Name: "b", UUID: "ub", State: lv.StateShutoff},
	}
	m.marks = map[string]bool{"ua": true, "ub": true}

	model, _ := m.handleNormalKey(keyMsg("U"))
	m = model.(Model)
	if m.confirmTyping {
		t.Error("below ceiling, U should NOT engage typed confirmation")
	}
	if !m.confirming || !m.confirmBulk {
		t.Error("below ceiling, U should engage bulk confirmation")
	}
}

func TestStartBulkOnNoEligibleFlashesAndNoCmd(t *testing.T) {
	m := confirmFixture(t)
	// Mark only running VMs — none are startable.
	m.marks = map[string]bool{"ua": true, "ub": true}

	model, _ := m.handleNormalKey(keyMsg("s"))
	m = model.(Model)
	if m.confirming {
		t.Error("start with no eligible marked should not engage confirmation")
	}
	if m.flash == "" {
		t.Error("expected a flash explaining no eligible VMs")
	}
}
