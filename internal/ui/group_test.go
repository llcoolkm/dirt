package ui

import (
	"testing"

	"github.com/llcoolkm/dirt/internal/lv"
)

func TestGroupKeyForOS(t *testing.T) {
	cases := []struct {
		d    lv.Domain
		want string
	}{
		{lv.Domain{OS: "Ubuntu 25.04"}, "Ubuntu 25.04"},
		{lv.Domain{OS: ""}, "(unknown)"},
	}
	for _, c := range cases {
		if got := groupKeyFor(c.d, "os"); got != c.want {
			t.Errorf("groupKeyFor(os)=%q, want %q", got, c.want)
		}
	}
}

func TestGroupKeyForState(t *testing.T) {
	for _, st := range []lv.State{lv.StateRunning, lv.StateShutoff, lv.StatePaused, lv.StateCrashed} {
		got := groupKeyFor(lv.Domain{State: st}, "state")
		if got != st.String() {
			t.Errorf("groupKeyFor(state, %v)=%q, want %q", st, got, st.String())
		}
	}
}

func TestGroupKeyForUnknownReturnsEmpty(t *testing.T) {
	if got := groupKeyFor(lv.Domain{}, "nonsense"); got != "" {
		t.Errorf("unknown field should return empty key, got %q", got)
	}
}

func TestVisibleDomainsHidesFoldedGroups(t *testing.T) {
	m := Model{
		snap: &lv.Snapshot{Domains: []lv.Domain{
			{Name: "a", State: lv.StateRunning},
			{Name: "b", State: lv.StateShutoff},
			{Name: "c", State: lv.StateRunning},
		}},
		groupBy:      "state",
		foldedGroups: map[string]bool{"running": true},
	}
	got := m.visibleDomains()
	if len(got) != 1 || got[0].Name != "b" {
		names := make([]string, len(got))
		for i, d := range got {
			names[i] = d.Name
		}
		t.Fatalf("folded 'running' group: got %v, want [b]", names)
	}
}

func TestVisibleDomainsGroupsContiguously(t *testing.T) {
	m := Model{
		snap: &lv.Snapshot{Domains: []lv.Domain{
			{Name: "alpha", State: lv.StateRunning},
			{Name: "bravo", State: lv.StateShutoff},
			{Name: "charlie", State: lv.StateRunning},
		}},
		groupBy:    "state",
		sortColumn: sortByName,
	}
	got := m.visibleDomains()
	if len(got) != 3 {
		t.Fatalf("got %d domains, want 3", len(got))
	}
	// All entries with the same group key must be adjacent.
	seen := make(map[string]bool)
	prev := ""
	for _, d := range got {
		k := groupKeyFor(d, "state")
		if k != prev {
			if seen[k] {
				t.Errorf("group %q reappears non-contiguously: %v", k, got)
			}
			seen[k] = true
			prev = k
		}
	}
}
