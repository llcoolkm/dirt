package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.Refresh != time.Second {
		t.Errorf("default refresh = %v, want 1s", c.Refresh)
	}
	if c.List.SortBy != "state" {
		t.Errorf("default sort_by = %q, want state", c.List.SortBy)
	}
	if !c.List.Columns["cpu"] {
		t.Error("default columns should have cpu visible")
	}
}

func TestLoadConfigMissingReturnsDefault(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(c, DefaultConfig()) {
		t.Errorf("missing file should return defaults\n got:  %+v\n want: %+v", c, DefaultConfig())
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	in := DefaultConfig()
	in.Refresh = 500 * time.Millisecond
	in.List.SortBy = "cpu"
	in.List.SortReverse = true
	in.List.Columns["io_w"] = false

	if err := SaveConfig(in); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	out, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if out.Refresh != in.Refresh {
		t.Errorf("refresh round-trip: got %v, want %v", out.Refresh, in.Refresh)
	}
	if out.List.SortBy != in.List.SortBy || out.List.SortReverse != in.List.SortReverse {
		t.Errorf("sort round-trip: got %+v, want %+v", out.List, in.List)
	}
	if out.List.Columns["io_w"] {
		t.Errorf("io_w should be hidden in round-trip, got visible")
	}
}

func TestLoadConfigFillsMissingFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "dirt"), 0o700); err != nil {
		t.Fatal(err)
	}
	// A minimal YAML with only refresh set — every other field must be
	// filled from DefaultConfig().
	minimal := []byte("refresh: 3s\n")
	if err := os.WriteFile(filepath.Join(dir, "dirt", "config.yaml"), minimal, 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if c.Refresh != 3*time.Second {
		t.Errorf("refresh = %v, want 3s", c.Refresh)
	}
	if c.List.SortBy != "state" {
		t.Errorf("sort_by not filled: got %q, want state", c.List.SortBy)
	}
	if len(c.List.Columns) == 0 {
		t.Error("columns map should have been filled with defaults")
	}
	if c.Theme == "" {
		t.Error("theme should have been filled")
	}
}

func TestSeedConfigIfMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	c, err := SeedConfigIfMissing()
	if err != nil {
		t.Fatalf("SeedConfigIfMissing: %v", err)
	}
	if !reflect.DeepEqual(c, DefaultConfig()) {
		t.Errorf("seeded config should match defaults\n got:  %+v\n want: %+v", c, DefaultConfig())
	}
	// File should now exist.
	if _, err := os.Stat(filepath.Join(dir, "dirt", "config.yaml")); err != nil {
		t.Errorf("expected config.yaml to exist after seeding: %v", err)
	}

	// Second call must not overwrite; should return the same thing.
	again, err := SeedConfigIfMissing()
	if err != nil {
		t.Fatalf("SeedConfigIfMissing (second): %v", err)
	}
	if !reflect.DeepEqual(again, c) {
		t.Errorf("second seed differs from first:\n first: %+v\n  again: %+v", c, again)
	}
}
