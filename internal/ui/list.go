package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/llcoolkm/dirt/internal/lv"
)

// Column widths for the list table.
const (
	colNameW   = 20
	colStateW  = 9
	colIPW     = 15
	colOSW     = 13
	colVCPUW   = 5
	colMemW    = 8
	colMemPctW = 6
	colCPUW    = 7
	colUptimeW = 8
	colIOReadW = 5
	colIOWriteW = 5
)

// listView renders the VM table.
func (m Model) listView() string {
	width := m.contentWidth()
	doms := m.visibleDomains()

	header := renderHeaderRow(m.sortColumn, m.sortDesc)

	if len(doms) == 0 {
		empty := lipgloss.NewStyle().Foreground(colDimmed).Italic(true).
			Render("  no domains" + filterSuffix(m.filter))
		body := lipgloss.JoinVertical(lipgloss.Left, header, "", empty)
		return listBox.Width(width).Render(body)
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
		row := renderDataRow(d, m.history[d.UUID], m.guestUptime[d.Name], i == m.selected)
		rows = append(rows, row)
	}
	return listBox.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// renderHeaderRow renders the column-header row, marking the active sort
// column with an arrow (▲ asc, ▼ desc).
func renderHeaderRow(active sortColumn, desc bool) string {
	mark := func(label string, col sortColumn, leftAlign bool, w int) string {
		s := label
		if active == col {
			arrow := "▲"
			if desc {
				arrow = "▼"
			}
			s = label + arrow
		}
		if leftAlign {
			return padRight(s, w)
		}
		return padLeft(s, w)
	}
	cols := []string{
		mark("NAME", sortByName, true, colNameW),
		mark("STATE", sortByState, true, colStateW),
		mark("IP", sortByIP, true, colIPW),
		mark("OS", sortByOS, true, colOSW),
		mark("vCPU", sortByVCPU, false, colVCPUW),
		mark("MEM", sortByMem, false, colMemW),
		mark("MEM%", sortByMemPct, false, colMemPctW),
		mark("CPU%", sortByCPU, false, colCPUW),
		mark("UPTIME", sortByUptime, false, colUptimeW),
		padLeft("IO-R", colIOReadW),
		padLeft("IO-W", colIOWriteW),
	}
	// Leading space matches the per-row indent used by renderDataRow,
	// so columns line up with their headers exactly.
	return listHeaderRow.Render(" " + strings.Join(cols, "  "))
}

// renderDataRow renders one VM row, optionally highlighted.
func renderDataRow(d lv.Domain, h *domHistory, qga lv.GuestUptime, selected bool) string {
	name := truncate(d.Name, colNameW)
	state := truncate(d.State.String(), colStateW)
	ip := truncate(d.IP, colIPW)
	if ip == "" {
		ip = "—"
	}
	osLabel := truncate(d.OS, colOSW)
	if osLabel == "" {
		osLabel = "—"
	}
	mem := formatKB(d.MaxMemKB)
	memPct := "—"
	cpu := "—"
	uptime := "—"
	ior := "—"
	iow := "—"
	if d.State == lv.StateRunning {
		if h != nil {
			cpu = fmt.Sprintf("%5.1f%%", h.currentCPU())
			ior = fmt.Sprintf("%.0f", currentRate(h.blockRdOps))
			iow = fmt.Sprintf("%.0f", currentRate(h.blockWrOps))
		}
		if pct, ok := domainMemUsedPct(d); ok {
			memPct = fmt.Sprintf("%4.1f%%", pct)
		}
		if up, accurate := effectiveUptime(d, h, qga); up > 0 {
			uptime = formatDuration(up)
			if !accurate {
				uptime = "≥" + uptime
			}
		}
	}

	stateColored := stateStyleFor(d.State).Render(padRight(state, colStateW))

	cols := []string{
		padRight(name, colNameW),
		stateColored,
		padRight(ip, colIPW),
		padRight(osLabel, colOSW),
		padLeft(fmt.Sprintf("%d", d.NrVCPU), colVCPUW),
		padLeft(mem, colMemW),
		padLeft(memPct, colMemPctW),
		padLeft(cpu, colCPUW),
		padLeft(uptime, colUptimeW),
		padLeft(ior, colIOReadW),
		padLeft(iow, colIOWriteW),
	}
	row := strings.Join(cols, "  ")
	if selected {
		// rowSelected sets bg/fg; lipgloss will respect existing fg in segments,
		// so for the selected row we strip color from state and re-render plain.
		plainState := padRight(state, colStateW)
		cols[1] = plainState
		row = strings.Join(cols, "  ")
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
