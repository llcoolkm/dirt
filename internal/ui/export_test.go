package ui

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/llcoolkm/dirt/internal/lv"
)

func exportFixture(t *testing.T) Model {
	t.Helper()
	return Model{
		snap: &lv.Snapshot{Domains: []lv.Domain{
			{Name: "alpha", UUID: "ua", State: lv.StateRunning, IP: "10.0.0.1", OS: "Ubuntu 25.04", NrVCPU: 2, MaxMemKB: 1 << 20},
			{Name: "beta", UUID: "ub", State: lv.StateShutoff, NrVCPU: 1, MaxMemKB: 512 * 1024},
		}},
		activeColumns: vmColumns,
		sortColumn:    sortByName,
	}
}

func TestExportCSV(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out.csv")
	m := exportFixture(t)

	path, err := m.exportTable("csv", dest)
	if err != nil {
		t.Fatalf("exportTable: %v", err)
	}
	if path != dest {
		t.Errorf("returned path %q, want %q", path, dest)
	}

	f, err := os.Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 { // header + two domains
		t.Fatalf("got %d rows, want 3", len(rows))
	}
	if rows[0][0] != "name" {
		t.Errorf("first column header=%q, want name", rows[0][0])
	}
	if rows[1][0] != "alpha" {
		t.Errorf("first data row name=%q, want alpha", rows[1][0])
	}
}

func TestExportJSON(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out.json")
	m := exportFixture(t)

	if _, err := m.exportTable("json", dest); err != nil {
		t.Fatalf("exportTable: %v", err)
	}

	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]string
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0]["name"] != "alpha" {
		t.Errorf("rows[0].name=%q, want alpha", rows[0]["name"])
	}
}

func TestExportRespectsFilter(t *testing.T) {
	dir := t.TempDir()
	m := exportFixture(t)
	m.filter = "alp"

	dest := filepath.Join(dir, "filtered.csv")
	if _, err := m.exportTable("csv", dest); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(b), "\n") != 2 { // header + alpha only
		t.Errorf("filter not honoured — content:\n%s", b)
	}
}

func TestExportUnknownFormatErrors(t *testing.T) {
	m := exportFixture(t)
	if _, err := m.exportTable("xml", ""); err == nil {
		t.Error("expected error for unknown format 'xml'")
	}
}
