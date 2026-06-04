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

func diskSortFixture() Model {
	return Model{
		snap: &lv.Snapshot{Domains: []lv.Domain{
			// charlie: 30G used of 100G  → 70G free, 30%
			{Name: "charlie", UUID: "uc", State: lv.StateRunning,
				TotalDiskCapacityBytes: 100 << 30, TotalDiskAllocationBytes: 30 << 30},
			// alpha: 10G used of 20G    → 10G free, 50%
			{Name: "alpha", UUID: "ua", State: lv.StateShutoff,
				TotalDiskCapacityBytes: 20 << 30, TotalDiskAllocationBytes: 10 << 30},
			// beta: 80G used of 100G    → 20G free, 80%
			{Name: "beta", UUID: "ub", State: lv.StateRunning,
				TotalDiskCapacityBytes: 100 << 30, TotalDiskAllocationBytes: 80 << 30},
		}},
		marks:         make(map[string]bool),
		activeColumns: vmColumns,
	}
}

func TestVisibleDomainsSortByDiskUsed(t *testing.T) {
	m := diskSortFixture()
	m.sortColumn = sortByDiskUsed
	got := m.visibleDomains()
	// Used bytes descending: beta(80G) > charlie(30G) > alpha(10G).
	want := []string{"beta", "charlie", "alpha"}
	for i, d := range got {
		if d.Name != want[i] {
			t.Errorf("sortByDiskUsed[%d]=%q, want %q", i, d.Name, want[i])
		}
	}
}

func TestVisibleDomainsSortByDiskFree(t *testing.T) {
	m := diskSortFixture()
	m.sortColumn = sortByDiskFree
	got := m.visibleDomains()
	// Free bytes descending: charlie(70G) > beta(20G) > alpha(10G).
	want := []string{"charlie", "beta", "alpha"}
	for i, d := range got {
		if d.Name != want[i] {
			t.Errorf("sortByDiskFree[%d]=%q, want %q", i, d.Name, want[i])
		}
	}
}

func TestVisibleDomainsSortByDiskPct(t *testing.T) {
	m := diskSortFixture()
	m.sortColumn = sortByDiskPct
	got := m.visibleDomains()
	// Usage fraction descending: beta(80%) > alpha(50%) > charlie(30%).
	want := []string{"beta", "alpha", "charlie"}
	for i, d := range got {
		if d.Name != want[i] {
			t.Errorf("sortByDiskPct[%d]=%q, want %q", i, d.Name, want[i])
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
