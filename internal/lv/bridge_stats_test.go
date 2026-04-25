package lv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadUintMissingFile(t *testing.T) {
	if _, ok := readUint("/nonexistent/path/here"); ok {
		t.Error("missing path should return ok=false")
	}
}

func TestReadUintParsesValue(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "n")
	if err := os.WriteFile(p, []byte("12345\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, ok := readUint(p)
	if !ok {
		t.Fatal("expected ok=true for valid uint")
	}
	if got != 12345 {
		t.Errorf("got %d, want 12345", got)
	}
}

func TestReadUintRejectsGarbage(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "garbage")
	if err := os.WriteFile(p, []byte("not a number"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := readUint(p); ok {
		t.Error("garbage content should return ok=false")
	}
}

func TestReadBridgeStatsMissingInterface(t *testing.T) {
	got := ReadBridgeStats([]string{"definitely-not-a-bridge-12345"})
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	if got[0].OK {
		t.Error("missing interface should return OK=false")
	}
	if got[0].Name != "definitely-not-a-bridge-12345" {
		t.Errorf("name not preserved: %q", got[0].Name)
	}
}

func TestReadBridgeStatsEmptyName(t *testing.T) {
	got := ReadBridgeStats([]string{""})
	if got[0].OK {
		t.Error("empty name should produce OK=false")
	}
}

func TestBridgeStatsString(t *testing.T) {
	ok := BridgeStats{Name: "br0", OK: true, RxBytes: 100, TxBytes: 200}
	if got := ok.String(); got == "" {
		t.Error("OK BridgeStats.String() should not be empty")
	}
	bad := BridgeStats{Name: "br1", OK: false}
	if got := bad.String(); got == "" {
		t.Error("bad BridgeStats.String() should not be empty")
	}
}
