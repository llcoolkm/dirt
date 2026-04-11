package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/llcoolkm/dirt/internal/lv"
)

// snap returns a DomainSnapshot with a given name, parent, and creation
// offset in seconds (relative to an arbitrary base time), so tests can
// control sibling order without sprinkling time.Now calls.
func snap(name, parent string, offset int) lv.DomainSnapshot {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return lv.DomainSnapshot{
		Name:      name,
		Parent:    parent,
		CreatedAt: base.Add(time.Duration(offset) * time.Second),
	}
}

func TestSortSnapshotsAsTreeEmpty(t *testing.T) {
	out, pre := sortSnapshotsAsTree(nil)
	if out != nil || pre != nil {
		t.Errorf("expected nil for empty input, got %v %v", out, pre)
	}
}

func TestSortSnapshotsAsTreeFlat(t *testing.T) {
	// Three root snapshots, no parents. They sort by creation time and
	// each gets an empty prefix — roots sit flush against the left edge.
	in := []lv.DomainSnapshot{
		snap("c", "", 30),
		snap("a", "", 10),
		snap("b", "", 20),
	}
	out, pre := sortSnapshotsAsTree(in)
	wantNames := []string{"a", "b", "c"}
	if len(out) != 3 {
		t.Fatalf("got %d items, want 3", len(out))
	}
	for i, s := range out {
		if s.Name != wantNames[i] {
			t.Errorf("position %d: got %q, want %q", i, s.Name, wantNames[i])
		}
		if pre[i] != "" {
			t.Errorf("root %q should have empty prefix, got %q", s.Name, pre[i])
		}
	}
}

func TestSortSnapshotsAsTreeLinearChain(t *testing.T) {
	// a -> b -> c (each is parent of the next).
	//   a
	//   └─ b
	//      └─ c
	in := []lv.DomainSnapshot{
		snap("c", "b", 30),
		snap("a", "", 10),
		snap("b", "a", 20),
	}
	out, pre := sortSnapshotsAsTree(in)
	wantNames := []string{"a", "b", "c"}
	for i, s := range out {
		if s.Name != wantNames[i] {
			t.Errorf("position %d: got %q, want %q", i, s.Name, wantNames[i])
		}
	}
	wantPrefixes := []string{
		"",       // a: root, no prefix
		"└─ ",    // b: only child of a
		"   └─ ", // c: only child of b; b was last so "   " pad
	}
	for i, p := range wantPrefixes {
		if pre[i] != p {
			t.Errorf("position %d: got prefix %q, want %q", i, pre[i], p)
		}
	}
}

func TestSortSnapshotsAsTreeForkedChildren(t *testing.T) {
	// Structure:
	//   a
	//   ├─ b
	//   │  └─ d
	//   └─ c
	in := []lv.DomainSnapshot{
		snap("d", "b", 30),
		snap("c", "a", 25),
		snap("b", "a", 20),
		snap("a", "", 10),
	}
	out, pre := sortSnapshotsAsTree(in)
	wantNames := []string{"a", "b", "d", "c"}
	if len(out) != 4 {
		t.Fatalf("got %d items, want 4", len(out))
	}
	for i, s := range out {
		if s.Name != wantNames[i] {
			t.Errorf("position %d: got %q, want %q", i, s.Name, wantNames[i])
		}
	}
	wantPrefixes := []string{
		"",       // a is root
		"├─ ",    // b is first of two children of a
		"│  └─ ", // d is only child of b; b still has c coming so "│  "
		"└─ ",    // c is last child of a
	}
	for i, p := range wantPrefixes {
		if pre[i] != p {
			t.Errorf("position %d (%q): got prefix %q, want %q",
				i, wantNames[i], pre[i], p)
		}
	}
}

func TestSortSnapshotsAsTreeOrphan(t *testing.T) {
	// "c" claims parent "nowhere", which is not present. It should still
	// appear in the output so the user can see and delete it.
	in := []lv.DomainSnapshot{
		snap("a", "", 10),
		snap("b", "a", 20),
		snap("c", "nowhere", 30),
	}
	out, _ := sortSnapshotsAsTree(in)
	if len(out) != 3 {
		t.Fatalf("got %d items, want 3", len(out))
	}
	names := make([]string, len(out))
	for i, s := range out {
		names[i] = s.Name
	}
	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "c") {
		t.Errorf("orphan snapshot c missing from output: %s", joined)
	}
}
