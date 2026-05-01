package ui

import "testing"

func TestIsSortableID(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"name", true},
		{"state", true},
		{"cpu", true},
		{"mem_pct", true},
		{"uptime", true},
		// IO / NET / autostart / persistent / arch all gained sort
		// enums alongside header-click discoverability.
		{"io_r", true},
		{"io_w", true},
		{"net_rx", true},
		{"autostart", true},
		{"arch", true},
		{"", false},
		{"bogus", false},
	}
	for _, c := range cases {
		if got := isSortableID(c.id); got != c.want {
			t.Errorf("isSortableID(%q)=%v, want %v", c.id, got, c.want)
		}
	}
}

func TestSortColumnFromIDFallsBackToState(t *testing.T) {
	if sortColumnFromID("garbage") != sortByState {
		t.Error("unknown id should fall back to sortByState")
	}
	if sortColumnFromID("cpu") != sortByCPU {
		t.Error("cpu id should map to sortByCPU")
	}
}

func TestSortColumnIDRoundTrip(t *testing.T) {
	for _, id := range []string{"name", "state", "ip", "os", "vcpu", "mem", "mem_pct", "cpu", "uptime"} {
		sc := sortColumnFromID(id)
		if got := sortColumnID(sc); got != id {
			t.Errorf("round-trip %q → %v → %q", id, sc, got)
		}
	}
}
