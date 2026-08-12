package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/llcoolkm/dirt/internal/lv"
)

func TestApplyGuestOSOverlaysLiveLabel(t *testing.T) {
	m := Model{
		snap: &lv.Snapshot{Domains: []lv.Domain{
			{Name: "vm1", State: lv.StateRunning, OS: "Ubuntu 22.04"},
			{Name: "vm2", State: lv.StateRunning, OS: "Debian 12"},
		}},
		guestOS: map[string]lv.GuestOSInfo{
			// vm1 was upgraded in-guest: QGA reports the new version.
			"vm1": {Available: true, Pretty: "Ubuntu 24.04", FetchedAt: time.Now()},
			// vm2's agent probe failed: keep the libosinfo fallback.
			"vm2": {Available: false, FetchedAt: time.Now()},
		},
	}
	m.applyGuestOS()
	if got := m.snap.Domains[0].OS; got != "Ubuntu 24.04" {
		t.Errorf("vm1 OS = %q, want live 'Ubuntu 24.04'", got)
	}
	if got := m.snap.Domains[1].OS; got != "Debian 12" {
		t.Errorf("vm2 OS = %q, want fallback 'Debian 12'", got)
	}
}

func TestSwapLineMessages(t *testing.T) {
	d := lv.Domain{Name: "vm1", State: lv.StateRunning}
	failed := lv.SwapInfo{Available: false}
	cases := []struct {
		name   string
		osInfo lv.GuestOSInfo
		want   string
	}{
		// Agent alive, guest is Windows: swap query is inapplicable, not missing.
		{"windows guest", lv.GuestOSInfo{Available: true, ID: "mswindows"}, "n/a (Windows guest)"},
		// Agent alive on a non-Windows guest but the exec probe failed.
		{"exec failed", lv.GuestOSInfo{Available: true, ID: "ubuntu"}, "unavailable"},
		// Agent genuinely unreachable: keep the install hint.
		{"no agent", lv.GuestOSInfo{}, "install qemu-guest-agent"},
	}
	for _, c := range cases {
		got := stripANSI(buildVMSwapLine(d, nil, failed, c.osInfo, 80))
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: swap line = %q, want substring %q", c.name, got, c.want)
		}
	}
}

func TestMaybeFetchGuestOSSkipsFreshAndStopped(t *testing.T) {
	m := Model{
		client: nil, // no client → never fetch
		snap: &lv.Snapshot{Domains: []lv.Domain{
			{Name: "vm1", State: lv.StateRunning},
		}},
		guestOS: map[string]lv.GuestOSInfo{},
	}
	if cmd := m.maybeFetchGuestOS(); cmd != nil {
		t.Error("expected nil cmd with nil client")
	}
}
