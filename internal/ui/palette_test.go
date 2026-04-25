package ui

import "testing"

func TestCommonPrefix(t *testing.T) {
	cases := []struct{ a, b, want string }{
		{"foo", "foobar", "foo"},
		{"mem", "mem_pct", "mem"},
		{"abc", "xyz", ""},
		{"", "anything", ""},
		{"unicode-ñ", "unicode-x", "unicode-"},
	}
	for _, c := range cases {
		if got := commonPrefix(c.a, c.b); got != c.want {
			t.Errorf("commonPrefix(%q, %q)=%q, want %q", c.a, c.b, got, c.want)
		}
	}
}

func TestExtendPrefix(t *testing.T) {
	cands := []string{"mark", "memo", "mem_pct", "name"}
	cases := []struct {
		prefix string
		want   string
		ok     bool
	}{
		{"n", "name", true},      // unique → full word
		{"mar", "mark", true},    // unique → full word
		{"me", "mem", true},      // mem* → common prefix
		{"x", "", false},         // no match
		{"", "", true},           // every candidate matches; common prefix is empty (returns prefix)
	}
	for _, c := range cases {
		got, ok := extendPrefix(c.prefix, cands)
		if ok != c.ok {
			t.Errorf("extendPrefix(%q): ok=%v, want %v", c.prefix, ok, c.ok)
			continue
		}
		if got != c.want {
			t.Errorf("extendPrefix(%q)=%q, want %q", c.prefix, got, c.want)
		}
	}
}

func TestCompletePaletteInputTopLevel(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"the", "theme"},     // unique
		{"q", "q"},            // already a single-char canonical
		{"unknown", "unknown"}, // no match — unchanged
	}
	for _, c := range cases {
		if got := completePaletteInput(c.in); got != c.want {
			t.Errorf("completePaletteInput(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}

func TestCompletePaletteInputSubArg(t *testing.T) {
	// :theme + sub-arg uniquely matching mono.
	if got := completePaletteInput("theme mo"); got != "theme mono" {
		t.Errorf("got %q, want 'theme mono'", got)
	}
	// :mark + 'a' uniquely matches "all".
	if got := completePaletteInput("mark a"); got != "mark all" {
		t.Errorf("got %q, want 'mark all'", got)
	}
	// :sort + 'm' has multiple matches (mem, mem_pct) — common prefix "mem".
	if got := completePaletteInput("sort m"); got != "sort mem" {
		t.Errorf("got %q, want 'sort mem'", got)
	}
	// Spacing preserved.
	if got := completePaletteInput("theme  mo"); got != "theme  mono" {
		t.Errorf("got %q, want 'theme  mono'", got)
	}
}

func TestUniquePrefixMatch(t *testing.T) {
	if m, ok := uniquePrefixMatch("the"); !ok || m != "theme" {
		t.Errorf("the → (%q, %v), want (theme, true)", m, ok)
	}
	if _, ok := uniquePrefixMatch(""); ok {
		t.Error("empty prefix should not match uniquely")
	}
	// "j" hits "jobs" only (no other top-level command starts with j).
	if m, ok := uniquePrefixMatch("j"); !ok || m != "jobs" {
		t.Errorf("j → (%q, %v), want (jobs, true)", m, ok)
	}
}
