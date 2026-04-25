package ui

import "testing"

func TestAttachStage1PicksDiskType(t *testing.T) {
	m := Model{
		attachStage:  1,
		attachVerb:   "attach",
		attachDomain: "alpha",
	}
	model, _ := m.handleAttachKey(keyMsg("d"))
	m = model.(Model)
	if m.attachType != "disk" {
		t.Errorf("attachType=%q, want disk", m.attachType)
	}
	if m.attachStage != 2 {
		t.Errorf("attachStage=%d, want 2", m.attachStage)
	}
	if m.attachParam1 == "" {
		t.Error("disk attach should seed a default path")
	}
}

func TestAttachStage1PicksNICType(t *testing.T) {
	m := Model{
		attachStage:  1,
		attachVerb:   "attach",
		attachDomain: "alpha",
	}
	model, _ := m.handleAttachKey(keyMsg("n"))
	m = model.(Model)
	if m.attachType != "nic" {
		t.Errorf("attachType=%q, want nic", m.attachType)
	}
	if m.attachStage != 2 {
		t.Errorf("attachStage=%d, want 2", m.attachStage)
	}
}

func TestAttachStage1EscCancels(t *testing.T) {
	m := Model{attachStage: 1, attachVerb: "attach"}
	model, _ := m.handleAttachKey(keyMsg("esc"))
	m = model.(Model)
	if m.attachStage != 0 {
		t.Errorf("esc should reset attachStage, got %d", m.attachStage)
	}
}

func TestDetachStage1PicksDiskWithEmptyDefault(t *testing.T) {
	m := Model{attachStage: 1, attachVerb: "detach"}
	model, _ := m.handleAttachKey(keyMsg("d"))
	m = model.(Model)
	if m.attachType != "disk" {
		t.Errorf("attachType=%q, want disk", m.attachType)
	}
	// Detach disk seeds a target dev (vdb).
	if m.attachParam1 != "vdb" {
		t.Errorf("detach disk default param1=%q, want 'vdb'", m.attachParam1)
	}
}

func TestDetachStage1PicksNICWithBlankParam(t *testing.T) {
	m := Model{attachStage: 1, attachVerb: "detach"}
	model, _ := m.handleAttachKey(keyMsg("n"))
	m = model.(Model)
	if m.attachType != "nic" {
		t.Errorf("attachType=%q, want nic", m.attachType)
	}
	if m.attachParam1 != "" {
		t.Errorf("detach nic default param1=%q, want empty (no sensible default for MAC)", m.attachParam1)
	}
}

func TestAttachStage2BackspaceTrimsParam(t *testing.T) {
	m := Model{
		attachStage:  2,
		attachVerb:   "attach",
		attachType:   "disk",
		attachParam1: "/path/x",
	}
	model, _ := m.handleAttachKey(keyMsg("backspace"))
	m = model.(Model)
	if m.attachParam1 != "/path/" {
		t.Errorf("backspace: %q, want '/path/'", m.attachParam1)
	}
}

func TestAttachStage2AccumulatesChars(t *testing.T) {
	m := Model{attachStage: 2, attachVerb: "attach", attachType: "nic", attachParam1: ""}
	for _, ch := range "br0" {
		model, _ := m.handleAttachKey(keyMsg(string(ch)))
		m = model.(Model)
	}
	if m.attachParam1 != "br0" {
		t.Errorf("typed param1=%q, want 'br0'", m.attachParam1)
	}
}

func TestAttachStage2EscCancels(t *testing.T) {
	m := Model{attachStage: 2, attachVerb: "attach", attachType: "disk"}
	model, _ := m.handleAttachKey(keyMsg("esc"))
	m = model.(Model)
	if m.attachStage != 0 {
		t.Errorf("esc should reset stage, got %d", m.attachStage)
	}
}

func TestExecuteDetachRejectsEmptyParam(t *testing.T) {
	m := Model{
		attachStage:  2,
		attachVerb:   "detach",
		attachType:   "disk",
		attachDomain: "alpha",
		attachParam1: "",
	}
	model, _ := m.executeDetach()
	m = model.(Model)
	if m.flash == "" {
		t.Error("empty target should flash an error")
	}
}
