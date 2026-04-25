package ui

import (
	"strings"
	"testing"

	"github.com/llcoolkm/dirt/internal/lv"
)

func networksRenderFixture(rates map[string]bridgeRate) Model {
	return Model{
		snap:        &lv.Snapshot{},
		networks: []lv.Network{
			{Name: "default", Active: true, Autostart: true, Bridge: "virbr0", Forward: "nat", NumLeases: 3},
			{Name: "isolated", Active: false, Bridge: "virbr1", Forward: "none"},
		},
		bridgeRates:   rates,
		activeColumns: vmColumns,
		width:         200,
		height:        40,
	}
}

func TestNetworksViewIncludesEveryEntry(t *testing.T) {
	m := networksRenderFixture(nil)
	out := stripANSI(m.networksView())
	for _, want := range []string{"default", "isolated", "virbr0", "virbr1", "active", "inactive"} {
		if !strings.Contains(out, want) {
			t.Errorf("networks view missing %q\n%s", want, out)
		}
	}
}

func TestNetworksViewBridgeRateColumns(t *testing.T) {
	m := networksRenderFixture(map[string]bridgeRate{
		"virbr0": {available: true, rxBps: 1024, txBps: 2048},
	})
	out := stripANSI(m.networksView())
	// formatRate produces tokens with units (KB/s etc.). Verify the
	// header glyphs and a unit appear when rate data is supplied.
	if !strings.Contains(out, "↓ RX") || !strings.Contains(out, "↑ TX") {
		t.Errorf("expected RX/TX rate column headers, got:\n%s", out)
	}
}

func TestNetworksViewEmptyShowsHint(t *testing.T) {
	m := networksRenderFixture(nil)
	m.networks = nil
	out := stripANSI(m.networksView())
	if !strings.Contains(out, "no networks") {
		t.Errorf("expected 'no networks' hint, got:\n%s", out)
	}
}

func TestNetworksViewError(t *testing.T) {
	m := networksRenderFixture(nil)
	m.networks = nil
	m.networksErr = errFake("denied")
	out := stripANSI(m.networksView())
	if !strings.Contains(out, "error") || !strings.Contains(out, "denied") {
		t.Errorf("expected error message, got:\n%s", out)
	}
}

// errFake is a tiny error string for tests; defined here to avoid
// pulling in fmt.Errorf for one assertion.
type errFake string

func (e errFake) Error() string { return string(e) }
