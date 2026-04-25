package ui

import (
	"strings"
	"testing"

	"github.com/llcoolkm/dirt/internal/lv"
)

func infoFixture() Model {
	return Model{
		mode:    viewInfo,
		infoFor: "alpha",
		info: lv.DomainInfo{
			Name:     "alpha",
			UUID:     "ua",
			OSType:   "hvm",
			Machine:  "pc",
			Firmware: "BIOS",
			Disks: []lv.DiskInfo{
				{Target: "vda", Device: "disk", Bus: "virtio", DriverType: "qcow2", Source: "/var/lib/libvirt/images/alpha.qcow2"},
				{Target: "sda", Device: "cdrom", Bus: "sata"},
			},
			NICs: []lv.NICInfo{
				{MAC: "52:54:00:aa:bb:cc", Model: "virtio", SourceType: "network", Source: "default", Target: "vnet0"},
			},
			Graphics: []lv.GraphicsInfo{
				{Type: "spice", Port: 5900, Listen: "127.0.0.1"},
			},
		},
		snap: &lv.Snapshot{Domains: []lv.Domain{
			{Name: "alpha", UUID: "ua", State: lv.StateRunning},
		}},
		history: map[string]*domHistory{
			"ua": {
				cpu:        []float64{12.5, 14.0, 15.2},
				memUsedPct: []float64{40.0, 41.5, 42.0},
			},
		},
		width:         200,
		height:        40,
	}
}

func TestInfoViewIncludesIdentity(t *testing.T) {
	m := infoFixture()
	out := stripANSI(m.infoView())
	for _, want := range []string{"alpha", "ua", "BIOS", "vda", "sda", "52:54:00:aa:bb:cc", "spice"} {
		if !strings.Contains(out, want) {
			t.Errorf("info view missing %q\n%s", want, out)
		}
	}
}

func TestInfoUUIDLookup(t *testing.T) {
	m := infoFixture()
	if got := m.infoUUID(); got != "ua" {
		t.Errorf("infoUUID()=%q, want 'ua'", got)
	}
	m.infoFor = "ghost"
	if got := m.infoUUID(); got != "" {
		t.Errorf("missing domain: infoUUID()=%q, want empty", got)
	}
	m.snap = nil
	if got := m.infoUUID(); got != "" {
		t.Errorf("nil snap: infoUUID()=%q, want empty", got)
	}
}

func TestInfoMiniGraphsAppendWhenSpaceAllows(t *testing.T) {
	h := &domHistory{
		cpu:        []float64{1, 2, 3, 4, 5},
		memUsedPct: []float64{10, 20, 30, 40, 50},
	}
	got := withInfoMiniGraphs("info: alpha", h, 200)
	if !strings.Contains(stripANSI(got), "CPU") || !strings.Contains(stripANSI(got), "MEM") {
		t.Errorf("expected CPU/MEM tags in title, got:\n%s", got)
	}
}

func TestInfoMiniGraphsDropOnTinyWidth(t *testing.T) {
	h := &domHistory{cpu: []float64{1}, memUsedPct: []float64{2}}
	got := withInfoMiniGraphs("info: alpha", h, 5)
	// Tiny width — title should be unchanged (no room for the tag).
	if got != "info: alpha" {
		t.Errorf("expected unchanged title on tiny width, got %q", got)
	}
}

func TestInfoViewError(t *testing.T) {
	m := infoFixture()
	m.infoErr = errFake("oops")
	out := stripANSI(m.infoView())
	if !strings.Contains(out, "error") || !strings.Contains(out, "oops") {
		t.Errorf("expected error rendering, got:\n%s", out)
	}
}
