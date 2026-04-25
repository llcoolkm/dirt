package ui

import (
	"strings"
	"testing"

	"github.com/llcoolkm/dirt/internal/config"
)

func TestRenderHostRowConnectedTagging(t *testing.T) {
	h := config.Host{Name: "remote", URI: "qemu+ssh://remote/system"}
	probe := hostProbeStatus{state: probeOK, domains: 5}
	got := renderHostRow(h, probe, h.URI, false)
	plain := stripANSI(got)
	if !strings.Contains(plain, "remote") {
		t.Errorf("name not rendered: %s", plain)
	}
	if !strings.Contains(plain, "connected") {
		t.Errorf("expected 'connected' status for current URI, got: %s", plain)
	}
}

func TestRenderHostRowReachable(t *testing.T) {
	h := config.Host{Name: "remote", URI: "qemu+ssh://remote/system"}
	probe := hostProbeStatus{state: probeOK, domains: 3}
	got := renderHostRow(h, probe, "qemu:///system", false) // different URI = not current
	plain := stripANSI(got)
	if !strings.Contains(plain, "reachable") {
		t.Errorf("expected 'reachable' for non-current OK probe, got: %s", plain)
	}
}

func TestRenderHostRowFailed(t *testing.T) {
	h := config.Host{Name: "down", URI: "qemu+ssh://down/system"}
	probe := hostProbeStatus{state: probeFailed}
	got := renderHostRow(h, probe, "qemu:///system", false)
	plain := stripANSI(got)
	if !strings.Contains(plain, "unreachable") {
		t.Errorf("expected 'unreachable' for failed probe, got: %s", plain)
	}
}

func TestRenderHostRowProbing(t *testing.T) {
	h := config.Host{Name: "init", URI: "qemu+ssh://init/system"}
	got := renderHostRow(h, hostProbeStatus{}, "qemu:///system", false)
	plain := stripANSI(got)
	if !strings.Contains(plain, "probing") {
		t.Errorf("expected 'probing…' for unset probe, got: %s", plain)
	}
}

func TestProbeDisplayAllStates(t *testing.T) {
	cases := []struct {
		p         hostProbeStatus
		isCurrent bool
		want      string
	}{
		{hostProbeStatus{state: probeOK}, true, "• connected"},
		{hostProbeStatus{state: probeOK}, false, "reachable"},
		{hostProbeStatus{state: probeFailed}, false, "unreachable"},
		{hostProbeStatus{}, false, "probing…"},
	}
	for _, c := range cases {
		got, _ := probeDisplay(c.p, c.isCurrent)
		if got != c.want {
			t.Errorf("probeDisplay(%+v, current=%v)=%q, want %q",
				c.p, c.isCurrent, got, c.want)
		}
	}
}
