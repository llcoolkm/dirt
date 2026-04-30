package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatePathHonoursXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/dirt-state-test")
	got := StatePath()
	want := filepath.Join("/tmp/dirt-state-test", "dirt", "state.yaml")
	if got != want {
		t.Errorf("StatePath = %q, want %q", got, want)
	}
}

func TestSaveAndLoadStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	rev := true
	in := State{
		Theme:       "phosphor",
		SortBy:      "cpu",
		SortReverse: &rev,
		MarkAdvance: "down",
		Columns: map[string]bool{
			"os":  true,
			"cpu": false,
		},
	}
	if err := SaveState(in); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	got, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if got.Theme != in.Theme {
		t.Errorf("Theme: got %q, want %q", got.Theme, in.Theme)
	}
	if got.SortBy != in.SortBy {
		t.Errorf("SortBy: got %q, want %q", got.SortBy, in.SortBy)
	}
	if got.SortReverse == nil || *got.SortReverse != *in.SortReverse {
		t.Errorf("SortReverse: got %v, want %v", got.SortReverse, in.SortReverse)
	}
	if got.MarkAdvance != in.MarkAdvance {
		t.Errorf("MarkAdvance: got %q, want %q", got.MarkAdvance, in.MarkAdvance)
	}
	if got.Columns["os"] != true || got.Columns["cpu"] != false {
		t.Errorf("Columns: got %v, want %v", got.Columns, in.Columns)
	}
}

func TestLoadStateMissingFileIsNotAnError(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	got, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState on missing file: %v", err)
	}
	if got.Theme != "" || got.SortBy != "" || got.SortReverse != nil {
		t.Errorf("expected zero State, got %+v", got)
	}
}

func TestMergeIntoLeavesUnsetFieldsAlone(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Theme = "gruvbox"
	cfg.List.SortBy = "name"
	cfg.List.SortReverse = true

	// State only sets Theme — SortBy and SortReverse must survive untouched.
	State{Theme: "phosphor"}.MergeInto(&cfg)

	if cfg.Theme != "phosphor" {
		t.Errorf("Theme: got %q, want phosphor", cfg.Theme)
	}
	if cfg.List.SortBy != "name" {
		t.Errorf("SortBy clobbered: got %q, want name", cfg.List.SortBy)
	}
	if !cfg.List.SortReverse {
		t.Errorf("SortReverse clobbered: got false, want true")
	}
}

func TestMergeIntoOverlaysColumnsKeyByKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.List.Columns = map[string]bool{"os": true, "cpu": true}

	// State only flips "cpu" — "os" must survive.
	State{Columns: map[string]bool{"cpu": false}}.MergeInto(&cfg)

	if cfg.List.Columns["os"] != true {
		t.Errorf("os column visibility wiped, expected true")
	}
	if cfg.List.Columns["cpu"] != false {
		t.Errorf("cpu column visibility not flipped, got %v", cfg.List.Columns["cpu"])
	}
}

func TestSaveStateCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	if err := SaveState(State{Theme: "default"}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "dirt", "state.yaml")); err != nil {
		t.Errorf("state file not created: %v", err)
	}
}
