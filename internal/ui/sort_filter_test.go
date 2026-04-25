package ui

import (
	"testing"

	"github.com/llcoolkm/dirt/internal/lv"
)

func sortFilterFixture() Model {
	return Model{
		snap: &lv.Snapshot{Domains: []lv.Domain{
			{Name: "charlie", UUID: "uc", State: lv.StateRunning, NrVCPU: 4, MaxMemKB: 4 << 20, IP: "10.0.0.3"},
			{Name: "alpha", UUID: "ua", State: lv.StateShutoff, NrVCPU: 1, MaxMemKB: 1 << 20, IP: "10.0.0.1"},
			{Name: "beta", UUID: "ub", State: lv.StateRunning, NrVCPU: 2, MaxMemKB: 2 << 20, IP: "10.0.0.2"},
		}},
		marks:         make(map[string]bool),
		activeColumns: vmColumns,
	}
}

func TestVisibleDomainsSortByName(t *testing.T) {
	m := sortFilterFixture()
	m.sortColumn = sortByName
	got := m.visibleDomains()
	want := []string{"alpha", "beta", "charlie"}
	for i, d := range got {
		if d.Name != want[i] {
			t.Errorf("sortByName[%d]=%q, want %q", i, d.Name, want[i])
		}
	}
}

func TestVisibleDomainsSortByNameDesc(t *testing.T) {
	m := sortFilterFixture()
	m.sortColumn = sortByName
	m.sortDesc = true
	got := m.visibleDomains()
	want := []string{"charlie", "beta", "alpha"}
	for i, d := range got {
		if d.Name != want[i] {
			t.Errorf("sortByName desc[%d]=%q, want %q", i, d.Name, want[i])
		}
	}
}

func TestVisibleDomainsSortByVCPU(t *testing.T) {
	m := sortFilterFixture()
	m.sortColumn = sortByVCPU
	got := m.visibleDomains()
	// vCPU sort defaults to descending (largest first) — that's the
	// convention for resource-magnitude columns.
	wantVCPU := []uint{4, 2, 1}
	for i, d := range got {
		if d.NrVCPU != wantVCPU[i] {
			t.Errorf("sortByVCPU[%d].NrVCPU=%d, want %d", i, d.NrVCPU, wantVCPU[i])
		}
	}
}

func TestVisibleDomainsSortByMem(t *testing.T) {
	m := sortFilterFixture()
	m.sortColumn = sortByMem
	got := m.visibleDomains()
	// Memory sort defaults to descending — same convention as vCPU.
	wantNames := []string{"charlie", "beta", "alpha"}
	for i, d := range got {
		if d.Name != wantNames[i] {
			t.Errorf("sortByMem[%d]=%q, want %q", i, d.Name, wantNames[i])
		}
	}
}

func TestVisibleDomainsFilterCaseInsensitive(t *testing.T) {
	m := sortFilterFixture()
	m.sortColumn = sortByName
	m.filter = "BET"
	got := m.visibleDomains()
	if len(got) != 1 || got[0].Name != "beta" {
		t.Errorf("case-insensitive filter for 'BET': got %v", got)
	}
}

func TestVisibleDomainsFilterNoMatchReturnsEmpty(t *testing.T) {
	m := sortFilterFixture()
	m.sortColumn = sortByName
	m.filter = "nothingmatches"
	if got := m.visibleDomains(); len(got) != 0 {
		t.Errorf("expected 0 matches, got %d", len(got))
	}
}

func TestVisibleDomainsFilterTrimsWhitespace(t *testing.T) {
	m := sortFilterFixture()
	m.sortColumn = sortByName
	m.filter = "   alpha  "
	got := m.visibleDomains()
	if len(got) != 1 || got[0].Name != "alpha" {
		t.Errorf("filter should trim whitespace; got %v", got)
	}
}

func TestVisibleDomainsEmptySnapshot(t *testing.T) {
	m := Model{snap: nil}
	if got := m.visibleDomains(); got != nil {
		t.Errorf("nil snapshot should return nil slice, got %v", got)
	}
}
