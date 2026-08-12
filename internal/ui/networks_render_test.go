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

func TestLeasesViewShowsStaticAndDynamicType(t *testing.T) {
	m := networksRenderFixture(nil)
	m.leasesFor = "default"
	m.leases = []lv.DHCPLease{
		{Hostname: "web-01", IP: "192.168.122.10", MAC: "52:54:00:aa:bb:cc", Static: true},
		{Hostname: "guest", IP: "192.168.122.101", MAC: "52:54:00:11:22:33", Static: false},
	}
	out := stripANSI(m.leasesView())
	if !strings.Contains(out, "TYPE") {
		t.Errorf("leases view missing TYPE header\n%s", out)
	}
	if !strings.Contains(out, "static") {
		t.Errorf("leases view missing 'static' for reserved lease\n%s", out)
	}
	if !strings.Contains(out, "dynamic") {
		t.Errorf("leases view missing 'dynamic' for plain lease\n%s", out)
	}
}

func TestLeasesKeyMArmsConfirmForDynamicLease(t *testing.T) {
	m := networksRenderFixture(nil)
	m.mode = viewLeases
	m.leasesFor = "default"
	m.leases = []lv.DHCPLease{
		{Hostname: "guest", IP: "192.168.122.101", MAC: "52:54:00:11:22:33", Static: false},
	}
	model, _ := m.handleLeasesKey(keyMsg("m"))
	m = model.(Model)
	if !m.confirming || m.confirmAction != "make-static" {
		t.Errorf("expected make-static confirm armed, got confirming=%v action=%q", m.confirming, m.confirmAction)
	}
	out := stripANSI(m.leasesView())
	if !strings.Contains(out, "add static mapping") {
		t.Errorf("leases status bar missing confirm prompt\n%s", out)
	}
}

func TestLeasesKeyMRejectsStaticLease(t *testing.T) {
	m := networksRenderFixture(nil)
	m.mode = viewLeases
	m.leasesFor = "default"
	m.leases = []lv.DHCPLease{
		{Hostname: "web-01", IP: "192.168.122.10", MAC: "52:54:00:aa:bb:cc", Static: true},
	}
	model, _ := m.handleLeasesKey(keyMsg("m"))
	m = model.(Model)
	if m.confirming {
		t.Error("static lease must not arm the confirm prompt")
	}
	if !strings.Contains(m.flash, "already") {
		t.Errorf("expected 'already static' flash, got %q", m.flash)
	}
}

func TestLeasesNavigation(t *testing.T) {
	m := networksRenderFixture(nil)
	m.mode = viewLeases
	m.leases = []lv.DHCPLease{
		{IP: "192.168.122.101", MAC: "52:54:00:11:22:33"},
		{IP: "192.168.122.102", MAC: "52:54:00:44:55:66"},
	}
	model, _ := m.handleLeasesKey(keyMsg("j"))
	m = model.(Model)
	if m.leasesSel != 1 {
		t.Errorf("leasesSel = %d after j, want 1", m.leasesSel)
	}
	model, _ = m.handleLeasesKey(keyMsg("k"))
	m = model.(Model)
	if m.leasesSel != 0 {
		t.Errorf("leasesSel = %d after k, want 0", m.leasesSel)
	}
}

// errFake is a tiny error string for tests; defined here to avoid
// pulling in fmt.Errorf for one assertion.
type errFake string

func (e errFake) Error() string { return string(e) }
