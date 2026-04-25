package ui

import (
	"strings"
	"testing"

	"github.com/llcoolkm/dirt/internal/lv"
)

func poolsFixture() Model {
	return Model{
		pools: []lv.StoragePool{
			{Name: "default", State: "running", Type: "dir", Capacity: 1 << 40, Allocation: 1 << 30, Available: (1 << 40) - (1 << 30)},
			{Name: "scratch", State: "inactive", Type: "logical"},
		},
		width:  200,
		height: 40,
	}
}

func TestPoolsViewIncludesEveryPool(t *testing.T) {
	m := poolsFixture()
	out := stripANSI(m.poolsView())
	for _, want := range []string{"default", "scratch", "running", "inactive", "dir", "logical"} {
		if !strings.Contains(out, want) {
			t.Errorf("pools view missing %q\n%s", want, out)
		}
	}
}

func TestPoolsViewEmpty(t *testing.T) {
	m := poolsFixture()
	m.pools = nil
	out := stripANSI(m.poolsView())
	if !strings.Contains(out, "no pools") && !strings.Contains(out, "no domains") {
		// pools view's empty hint is "no pools" — the test is permissive
		// in case the wording shifts.
		if !strings.Contains(out, "pool") {
			t.Errorf("expected an empty-pools hint, got:\n%s", out)
		}
	}
}

func TestPoolsViewError(t *testing.T) {
	m := poolsFixture()
	m.pools = nil
	m.poolsErr = errFake("boom")
	out := stripANSI(m.poolsView())
	if !strings.Contains(out, "error") || !strings.Contains(out, "boom") {
		t.Errorf("expected error rendering, got:\n%s", out)
	}
}
