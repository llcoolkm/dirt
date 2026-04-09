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
	colCPUW    = 7
	colUptimeW = 8
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
		row := renderDataRow(d, m.history[d.UUID], i == m.selected)
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
		padRight("IP", colIPW),
		padRight("OS", colOSW),
		mark("vCPU", sortByVCPU, false, colVCPUW),
		mark("MEM", sortByMem, false, colMemW),
		mark("CPU%", sortByCPU, false, colCPUW),
		padLeft("UPTIME", colUptimeW),
	}
	return listHeaderRow.Render(strings.Join(cols, "  "))
}

// renderDataRow renders one VM row, optionally highlighted.
func renderDataRow(d lv.Domain, h *domHistory, selected bool) string {
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
	cpu := "—"
	uptime := "—"
	if d.State == lv.StateRunning && h != nil {
		cpu = fmt.Sprintf("%5.1f%%", h.currentCPU())
		if up := h.uptime(); up > 0 {
			uptime = formatDuration(up)
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
		padLeft(cpu, colCPUW),
		padLeft(uptime, colUptimeW),
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

func filterSuffix(f string) string {
	if f == "" {
		return ""
	}
	return " matching “" + f + "”"
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s[:w]
	}
	return s + strings.Repeat(" ", w-len(s))
}

func padLeft(s string, w int) string {
	if len(s) >= w {
		return s[:w]
	}
	return strings.Repeat(" ", w-len(s)) + s
}

func truncate(s string, w int) string {
	if len(s) <= w {
		return s
	}
	if w < 1 {
		return ""
	}
	return s[:w-1] + "…"
}
