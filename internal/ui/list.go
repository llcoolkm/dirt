package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/llcoolkm/dirt/internal/lv"
)

// Column widths for the list table.
const (
	colNameW    = 20
	colStateW   = 9
	colIPW      = 15
	colOSW      = 13
	colVCPUW    = 5
	colMemW     = 8
	colMemPctW  = 6
	colCPUW     = 7
	colUptimeW  = 8
	colIOReadW  = 5
	colIOWriteW = 5
)

// column describes a VM list column. The render function produces the
// cell value for a given domain; left align and required control layout
// and whether the column can be dropped on narrow terminals. The master
// list (vmColumns below) is ordered left→right by priority, so columns
// are dropped from the right until the remaining row fits the width.
type column struct {
	label     string
	sort      sortColumn // 0 for non-sortable
	width     int
	leftAlign bool
	required  bool
	render    func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string
}

// vmColumns is the master list of VM-table columns, left to right, in
// priority order. Required columns (NAME, STATE, IP) are always shown;
// everything else is droppable on narrow terminals.
var vmColumns = []column{
	{label: "NAME", sort: sortByName, width: colNameW, leftAlign: true, required: true,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			return truncate(d.Name, colNameW)
		}},
	{label: "STATE", sort: sortByState, width: colStateW, leftAlign: true, required: true,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			return truncate(d.State.String(), colStateW)
		}},
	{label: "IP", sort: sortByIP, width: colIPW, leftAlign: true, required: true,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.IP == "" {
				return "—"
			}
			return truncate(d.IP, colIPW)
		}},
	{label: "OS", sort: sortByOS, width: colOSW, leftAlign: true,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.OS == "" {
				return "—"
			}
			return truncate(d.OS, colOSW)
		}},
	{label: "vCPU", sort: sortByVCPU, width: colVCPUW,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			return fmt.Sprintf("%d", d.NrVCPU)
		}},
	{label: "MEM", sort: sortByMem, width: colMemW,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			return formatKB(d.MaxMemKB)
		}},
	{label: "MEM%", sort: sortByMemPct, width: colMemPctW,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.State != lv.StateRunning {
				return "—"
			}
			if pct, ok := domainMemUsedPct(d); ok {
				return fmt.Sprintf("%4.1f%%", pct)
			}
			return "—"
		}},
	{label: "CPU%", sort: sortByCPU, width: colCPUW,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.State != lv.StateRunning || h == nil {
				return "—"
			}
			return fmt.Sprintf("%5.1f%%", h.currentCPU())
		}},
	{label: "UPTIME", sort: sortByUptime, width: colUptimeW,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.State != lv.StateRunning {
				return "—"
			}
			if up, accurate := effectiveUptime(d, h, qga); up > 0 && accurate {
				return formatDuration(up)
			}
			return "—"
		}},
	{label: "IO-R", width: colIOReadW,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.State != lv.StateRunning || h == nil {
				return "—"
			}
			return fmt.Sprintf("%.0f", currentRate(h.blockRdOps))
		}},
	{label: "IO-W", width: colIOWriteW,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.State != lv.StateRunning || h == nil {
				return "—"
			}
			return fmt.Sprintf("%.0f", currentRate(h.blockWrOps))
		}},
}

// columnsWidth returns the rendered width of a consecutive slice of
// vmColumns, including the leading indent and two-space separators.
func columnsWidth(cols []column) int {
	if len(cols) == 0 {
		return 0
	}
	w := 1 // leading space from the row indent
	for _, c := range cols {
		w += c.width
	}
	w += (len(cols) - 1) * 2
	return w
}

// fitColumns returns how many of vmColumns fit in avail characters of
// inner row width. Required columns are never dropped even if the row
// would overflow — in that case the caller accepts a bit of wrapping
// rather than hiding critical fields.
func fitColumns(avail int) int {
	required := 0
	for _, c := range vmColumns {
		if !c.required {
			break
		}
		required++
	}
	for n := len(vmColumns); n > required; n-- {
		if columnsWidth(vmColumns[:n]) <= avail {
			return n
		}
	}
	return required
}

// listView renders the VM table.
func (m Model) listView() string {
	width := m.contentWidth()
	doms := m.visibleDomains()

	// Compute how many columns fit. The inner area is the box width
	// minus the rounded border (2) minus the horizontal padding (2).
	inner := width - borderWidth - 2
	if inner < 1 {
		inner = 1
	}
	nCols := fitColumns(inner)

	header := renderHeaderRow(m.sortColumn, m.sortDesc, nCols)

	if len(doms) == 0 {
		empty := lipgloss.NewStyle().Foreground(colDimmed).Italic(true).
			Render("  no domains" + filterSuffix(m.filter))
		body := lipgloss.JoinVertical(lipgloss.Left, header, "", empty)
		return listBox.Width(width - borderWidth).Render(body)
	}

	// Available row count for the table body.
	available := m.listBodyHeight()
	if available < 1 {
		available = 1
	}

	// Adjust offset so the selection stays visible.
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+available {
		m.offset = m.selected - available + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
	end := m.offset + available
	if end > len(doms) {
		end = len(doms)
	}

	rows := make([]string, 0, end-m.offset+1)
	rows = append(rows, header)
	for i := m.offset; i < end; i++ {
		d := doms[i]
		row := renderDataRow(d, m.history[d.UUID], m.guestUptime[d.Name], i == m.selected, nCols)
		rows = append(rows, row)
	}
	return listBox.Width(width - borderWidth).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// renderHeaderRow renders the column-header row, marking the active sort
// column with an arrow (▲ asc, ▼ desc). Only the first nCols of
// vmColumns are shown; dropped columns become invisible.
func renderHeaderRow(active sortColumn, desc bool, nCols int) string {
	if nCols > len(vmColumns) {
		nCols = len(vmColumns)
	}
	cells := make([]string, 0, nCols)
	for _, c := range vmColumns[:nCols] {
		s := c.label
		if c.sort != 0 && active == c.sort {
			arrow := "▲"
			if desc {
				arrow = "▼"
			}
			s = c.label + arrow
		}
		if c.leftAlign {
			cells = append(cells, padRight(s, c.width))
		} else {
			cells = append(cells, padLeft(s, c.width))
		}
	}
	// Leading space matches the per-row indent used by renderDataRow,
	// so columns line up with their headers exactly.
	return listHeaderRow.Render(" " + strings.Join(cells, "  "))
}

// renderDataRow renders one VM row, optionally highlighted. Only the
// first nCols of vmColumns are shown. The STATE column uses a state-
// specific colour when not selected; for selected rows the colour is
// stripped so the rowSelected style can apply its own fg/bg.
func renderDataRow(d lv.Domain, h *domHistory, qga lv.GuestUptime, selected bool, nCols int) string {
	if nCols > len(vmColumns) {
		nCols = len(vmColumns)
	}
	cells := make([]string, 0, nCols)
	for _, c := range vmColumns[:nCols] {
		raw := c.render(d, h, qga)
		var padded string
		if c.leftAlign {
			padded = padRight(raw, c.width)
		} else {
			padded = padLeft(raw, c.width)
		}
		// Colour the STATE cell by the domain state for non-selected rows.
		if c.sort == sortByState && !selected {
			padded = stateStyleFor(d.State).Render(padded)
		}
		cells = append(cells, padded)
	}
	row := strings.Join(cells, "  ")
	if selected {
		return rowSelected.Render(" " + row)
	}
	return " " + row
}

// listBodyHeight returns how many data rows we can show in the list pane.
// Total terminal height minus header pane (≈ 8 lines), list border (2),
// list header row (1), status bar (1), and a small safety margin.
func (m Model) listBodyHeight() int {
	if m.height == 0 {
		return 10
	}
	headerH := 8
	chrome := 2 + 1 + 1 // list border top/bot + header row + status bar
	h := m.height - headerH - chrome - 1
	if h < 1 {
		h = 1
	}
	return h
}

func (m Model) contentWidth() int {
	if m.width <= 0 {
		return 80
	}
	return m.width
}

// domainMemUsedPct computes the same "used memory percent" the per-VM header
// renders in its multi-segment bar. Uses balloon stats when available; falls
// back to the (less meaningful) currently-allocated memory.
//
// Returns ok=false when there's no usable signal (e.g. domain not running, or
// balloon driver absent and MaxMem unknown).
func domainMemUsedPct(d lv.Domain) (float64, bool) {
	totalKB := d.BalloonAvailableKB
	if totalKB == 0 {
		totalKB = d.MaxMemKB
	}
	if totalKB == 0 {
		return 0, false
	}
	hasBalloon := d.BalloonAvailableKB > 0 && d.BalloonUnusedKB > 0
	var usedKB uint64
	if hasBalloon {
		freeKB := d.BalloonUnusedKB
		cacheKB := d.BalloonDiskCachesKB
		if totalKB > freeKB+cacheKB {
			usedKB = totalKB - freeKB - cacheKB
		}
	} else {
		usedKB = d.MemoryKB
		if usedKB > totalKB {
			usedKB = totalKB
		}
	}
	return float64(usedKB) / float64(totalKB) * 100, true
}

func filterSuffix(f string) string {
	if f == "" {
		return ""
	}
	return " matching “" + f + "”"
}

// padRight pads s to w *visible cells* (not bytes), so multi-byte unicode like
// the sort arrows ▲▼ and the ellipsis … line up correctly with ASCII columns.
// Strings already containing ANSI escape sequences are also handled, since
// lipgloss.Width strips them before measuring.
func padRight(s string, w int) string {
	cw := lipgloss.Width(s)
	if cw >= w {
		return s
	}
	return s + strings.Repeat(" ", w-cw)
}

func padLeft(s string, w int) string {
	cw := lipgloss.Width(s)
	if cw >= w {
		return s
	}
	return strings.Repeat(" ", w-cw) + s
}

// truncate shortens s so that its visible width is ≤ w, replacing the cut
// portion with "…". Operates on runes, never byte-slicing UTF-8.
func truncate(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	if w < 1 {
		return ""
	}
	out := make([]rune, 0, w)
	used := 0
	for _, r := range s {
		if used+1 > w-1 {
			break
		}
		out = append(out, r)
		used++
	}
	return string(out) + "…"
}
