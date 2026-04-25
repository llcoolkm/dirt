package ui

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/llcoolkm/dirt/internal/lv"
)

// exportTable writes the current filtered VM list to a file in the
// requested format ("csv" or "json"). Returns the path written so
// the flash can quote it. The path is built from $HOME/dirt-export-
// <ts>.<ext> when no path is supplied.
func (m Model) exportTable(format, dest string) (string, error) {
	doms := m.visibleDomains()
	cols := m.activeColumns
	if len(cols) == 0 {
		cols = vmColumns
	}

	if dest == "" {
		ts := time.Now().Format("20060102-150405")
		home, _ := os.UserHomeDir()
		if home == "" {
			home = "."
		}
		dest = filepath.Join(home, fmt.Sprintf("dirt-export-%s.%s", ts, format))
	}

	switch format {
	case "csv":
		return dest, writeCSV(dest, cols, doms, m.history, m.guestUptime)
	case "json":
		return dest, writeJSON(dest, cols, doms, m.history, m.guestUptime)
	default:
		return "", fmt.Errorf("unknown format: %s", format)
	}
}

func writeCSV(path string, cols []column, doms []lv.Domain,
	history map[string]*domHistory, qga map[string]lv.GuestUptime) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	hdr := make([]string, len(cols))
	for i, c := range cols {
		hdr[i] = c.id
	}
	if err := w.Write(hdr); err != nil {
		return err
	}
	for _, d := range doms {
		row := make([]string, len(cols))
		for i, c := range cols {
			row[i] = stripStyling(c.render(d, history[d.UUID], qga[d.Name]))
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(path string, cols []column, doms []lv.Domain,
	history map[string]*domHistory, qga map[string]lv.GuestUptime) error {
	out := make([]map[string]string, 0, len(doms))
	for _, d := range doms {
		row := make(map[string]string, len(cols))
		for _, c := range cols {
			row[c.id] = stripStyling(c.render(d, history[d.UUID], qga[d.Name]))
		}
		out = append(out, row)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// stripStyling drops common formatting glyphs that look right in the
// terminal but are noise in a spreadsheet (em-dashes, percent signs
// applied as cosmetic suffixes are kept; the column renderer's own
// padding spaces are trimmed).
func stripStyling(s string) string {
	return strings.TrimSpace(s)
}
