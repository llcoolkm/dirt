package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/llcoolkm/dirt/internal/backend"
	"github.com/llcoolkm/dirt/internal/lv"
)

func allRenderFixture() Model {
	return Model{
		mode: viewAll,
		// nil values are fine — the render code only counts entries.
		allBackends: map[string]backend.Backend{
			"lab1": nil,
			"prod": nil,
		},
		allSnapshots: map[string]*lv.Snapshot{
			"lab1": {Hostname: "lab1", Domains: []lv.Domain{
				{Name: "build-01", State: lv.StateRunning, IP: "10.0.0.1", OS: "Ubuntu 24.04", NrVCPU: 4, MemoryKB: 4 * 1024 * 1024, MaxMemKB: 8 * 1024 * 1024},
				{Name: "build-02", State: lv.StateShutoff, IP: "", OS: "Debian 12", NrVCPU: 2, MaxMemKB: 4 * 1024 * 1024},
			}},
			"prod": {Hostname: "prod", Domains: []lv.Domain{
				{Name: "web-01", State: lv.StateRunning, IP: "10.0.1.10", OS: "Alma 9", NrVCPU: 8, MemoryKB: 12 * 1024 * 1024, MaxMemKB: 16 * 1024 * 1024},
			}},
		},
		allErrs: map[string]error{},
		width:   200,
		height:  40,
	}
}

func TestAllViewRendersHostsAndVMs(t *testing.T) {
	m := allRenderFixture()
	out := stripANSI(m.allView())
	for _, want := range []string{"all hosts", "lab1", "prod", "build-01", "build-02", "web-01", "Ubuntu", "Alma"} {
		if !strings.Contains(out, want) {
			t.Errorf("allView missing %q\n%s", want, out)
		}
	}
}

func TestAllViewFilterReducesRows(t *testing.T) {
	m := allRenderFixture()
	m.filter = "web"
	out := stripANSI(m.allView())
	if strings.Contains(out, "build-01") {
		t.Errorf("filter=%q should hide build-01\n%s", m.filter, out)
	}
	if !strings.Contains(out, "web-01") {
		t.Errorf("filter=%q should keep web-01\n%s", m.filter, out)
	}
}

func TestAllViewShowsConnectingHintBeforeAnyData(t *testing.T) {
	m := Model{
		mode:         viewAll,
		allBackends:  map[string]backend.Backend{},
		allSnapshots: map[string]*lv.Snapshot{},
		allErrs:      map[string]error{},
		width:        200,
		height:       40,
	}
	out := stripANSI(m.allView())
	if !strings.Contains(out, "connecting") {
		t.Errorf("expected 'connecting…' hint when no backends opened, got:\n%s", out)
	}
}

func TestAllViewSurfacesPerHostErrors(t *testing.T) {
	m := allRenderFixture()
	m.allErrs["lab1"] = errors.New("connection refused")
	out := stripANSI(m.allView())
	if !strings.Contains(out, "lab1") || !strings.Contains(out, "connection refused") {
		t.Errorf("expected per-host error footer, got:\n%s", out)
	}
}
