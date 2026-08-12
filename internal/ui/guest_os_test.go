package ui

import (
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
