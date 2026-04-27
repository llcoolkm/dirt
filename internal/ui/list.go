package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
	// Bar columns: [█████] = 1+5+1 = 7 cells. The bar's fill is the
	// percentage — a numeric label would be redundant.
	colBarW = 7
)

// column describes a VM list column. The id is a stable lowercase
// identifier used in config.yaml and for lookups; the label is the
// display string shown in the header row. The render function produces
// the cell value for a given domain; leftAlign and required control
// layout and whether the column can be dropped on narrow terminals.
// The master list (vmColumns below) is ordered left→right by priority,
// so columns are dropped from the right until the row fits the width.
type column struct {
	id        string
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
	{id: "name", label: "NAME", sort: sortByName, width: colNameW, leftAlign: true, required: true,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			return truncate(d.Name, colNameW)
		}},
	{id: "state", label: "STATE", sort: sortByState, width: colStateW, leftAlign: true, required: true,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			return truncate(d.State.String(), colStateW)
		}},
	{id: "ip", label: "IP", sort: sortByIP, width: colIPW, leftAlign: true, required: true,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.IP == "" {
				return "—"
			}
			return truncate(d.IP, colIPW)
		}},
	{id: "os", label: "OS", sort: sortByOS, width: colOSW, leftAlign: true,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.OS == "" {
				return "—"
			}
			return truncate(d.OS, colOSW)
		}},
	{id: "vcpu", label: "vCPU", sort: sortByVCPU, width: colVCPUW,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			return fmt.Sprintf("%d", d.NrVCPU)
		}},
	{id: "mem", label: "MEM", sort: sortByMem, width: colMemW,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			return formatKB(d.MaxMemKB)
		}},
	{id: "mem_pct", label: "MEM%", sort: sortByMemPct, width: colMemPctW,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.State != lv.StateRunning {
				return "—"
			}
			if pct, ok := domainMemUsedPct(d); ok {
				return fmt.Sprintf("%4.1f%%", pct)
			}
			return "—"
		}},
	{id: "mem_bar", label: "MEM", width: colBarW, leftAlign: true,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.State != lv.StateRunning {
				return ""
			}
			pct, ok := domainMemUsedPct(d)
			if !ok {
				return ""
			}
			return "[" + colorBar(pct, 5) + "]"
		}},
	{id: "cpu", label: "CPU%", sort: sortByCPU, width: colCPUW,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.State != lv.StateRunning || h == nil {
				return "—"
			}
			return fmt.Sprintf("%5.1f%%", h.currentCPU())
		}},
	{id: "cpu_bar", label: "CPU", width: colBarW, leftAlign: true,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.State != lv.StateRunning || h == nil {
				return ""
			}
			return "[" + colorBar(h.currentCPU(), 5) + "]"
		}},
	{id: "uptime", label: "UPTIME", sort: sortByUptime, width: colUptimeW,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.State != lv.StateRunning {
				return "—"
			}
			if up, accurate := effectiveUptime(d, h, qga); up > 0 && accurate {
				return formatDuration(up)
			}
			return "—"
		}},
	{id: "io_r", label: "IO-R", width: colIOReadW,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.State != lv.StateRunning || h == nil {
				return "—"
			}
			return fmt.Sprintf("%.0f", currentRate(h.blockRdOps))
		}},
	{id: "io_w", label: "IO-W", width: colIOWriteW,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.State != lv.StateRunning || h == nil {
				return "—"
			}
			return fmt.Sprintf("%.0f", currentRate(h.blockWrOps))
		}},
	{id: "disk_bar", label: "DISK", width: colBarW, leftAlign: true,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.TotalDiskCapacityBytes == 0 {
				return ""
			}
			pct := float64(d.TotalDiskAllocationBytes) / float64(d.TotalDiskCapacityBytes) * 100
			return "[" + storageColorBar(pct, 5) + "]"
		}},
	{id: "net_rate", label: "NET", width: 16, leftAlign: true,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.State != lv.StateRunning || h == nil {
				return "—"
			}
			rx := currentRate(h.netRx)
			tx := currentRate(h.netTx)
			return fmt.Sprintf("↓%s ↑%s", formatRate(rx), formatRate(tx))
		}},
	{id: "autostart", label: "AUTO", width: 5, leftAlign: true,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.Autostart {
				return "Y"
			}
			return "N"
		}},
	{id: "persistent", label: "PERS", width: 5, leftAlign: true,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.Persistent {
				return "Y"
			}
			return "N"
		}},
	{id: "arch", label: "ARCH", width: 8, leftAlign: true,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if d.Arch == "" {
				return "—"
			}
			return truncate(d.Arch, 8)
		}},
	{id: "tag", label: "TAGS", sort: sortByTag, width: 18, leftAlign: true,
		render: func(d lv.Domain, h *domHistory, qga lv.GuestUptime) string {
			if len(d.Tags) == 0 {
				return "—"
			}
			return truncate(strings.Join(d.Tags, ","), 18)
		}},
}

// filterActiveColumns returns the subset of vmColumns the config
// wants shown. Required columns are always retained. Optional
// columns are included when visibility[id] is true OR absent — a
// missing entry is treated as visible, so a sparse config does not
// accidentally hide everything.
func filterActiveColumns(all []column, visibility map[string]bool) []column {
	out := make([]column, 0, len(all))
	for _, c := range all {
		if c.required {
			out = append(out, c)
			continue
		}
		if visible, present := visibility[c.id]; present && !visible {
			continue
		}
		out = append(out, c)
	}
	return out
}

// sortColumnFromID returns the sortColumn enum value for a config
// "sort_by" string. Unknown values fall through to sortByState so an
// invalid config still produces a sensible default.
func sortColumnFromID(id string) sortColumn {
	for _, c := range vmColumns {
		if c.id == id && c.sort != 0 {
			return c.sort
		}
	}
	return sortByState
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

// fitColumns returns how many of cols fit in avail characters of
// inner row width. Required columns are never dropped even if the row
// would overflow — in that case the caller accepts a bit of wrapping
// rather than hiding critical fields. cols is the caller's active
// column slice, so the fit respects the user's column-visibility
// preferences from config.yaml.
func fitColumns(cols []column, avail int) int {
	required := 0
	for _, c := range cols {
		if !c.required {
			break
		}
		required++
	}
	for n := len(cols); n > required; n-- {
		if columnsWidth(cols[:n]) <= avail {
			return n
		}
	}
	return required
}

// listView renders the VM table.
func (m Model) listView() string {
	width := m.contentWidth()
	doms := m.visibleDomains()

	cols := m.activeColumns
	if len(cols) == 0 {
		cols = vmColumns
	}

	// Compute how many columns fit. The inner area is the box width
	// minus the rounded border (2) minus the horizontal padding (2).
	inner := width - borderWidth - 2
	if inner < 1 {
		inner = 1
	}
	nCols := fitColumns(cols, inner)

	header := renderHeaderRow(cols, m.sortColumn, m.sortDesc, nCols)

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

	// Viewport indicator, right-aligned on the header row when the
	// table is longer than the visible window. Dropped silently if
	// the terminal is too narrow to host it alongside the columns.
	if len(doms) > available {
		pos := fmt.Sprintf("%d–%d/%d", m.offset+1, end, len(doms))
		gap := inner - lipgloss.Width(header) - lipgloss.Width(pos)
		if gap >= 2 {
			header = header + strings.Repeat(" ", gap) + headerLabel.Render(pos)
		}
	}

	rows := make([]string, 0, end-m.offset+1)
	rows = append(rows, header)
	prevGroup := "<<unset>>"
	for i := m.offset; i < end; i++ {
		d := doms[i]
		if m.groupBy != "" {
			gk := groupKeyFor(d, m.groupBy)
			if gk != prevGroup {
				rows = append(rows, renderGroupHeader(m, gk, doms))
				prevGroup = gk
			}
		}
		row := renderDataRow(cols, d, m.history[d.UUID], m.guestUptime[d.Name], i == m.selected, m.isMarked(d.UUID), nCols)
		rows = append(rows, row)
	}
	return listBox.Width(width - borderWidth).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// renderGroupHeader produces the bold separator line shown above the
// first row of each group. Counts every domain in the group across
// the *unfiltered* snapshot — folded ones included — so the header
// always reflects reality.
func renderGroupHeader(m Model, key string, visible []lv.Domain) string {
	var total, running int
	if m.snap != nil {
		for _, d := range m.snap.Domains {
			if groupKeyFor(d, m.groupBy) != key {
				continue
			}
			total++
			if d.State == lv.StateRunning {
				running++
			}
		}
	}
	folded := ""
	if m.foldedGroups[key] {
		folded = "  ▶ folded"
	}
	label := fmt.Sprintf("▼ %s  ·  %d total · %d running%s",
		key, total, running, folded)
	if m.foldedGroups[key] {
		label = strings.Replace(label, "▼", "▶", 1)
	}
	return " " + listHeaderRow.Render(label)
}

// renderHeaderRow renders the column-header row, marking the active
// sort column with an arrow (▲ asc, ▼ desc). Only the first nCols of
// cols are shown; dropped columns become invisible.
func renderHeaderRow(cols []column, active sortColumn, desc bool, nCols int) string {
	if nCols > len(cols) {
		nCols = len(cols)
	}
	cells := make([]string, 0, nCols)
	for _, c := range cols[:nCols] {
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
// first nCols of cols are shown. The STATE column uses a state-
// specific colour when not selected; for selected rows the colour is
// stripped so the rowSelected style can apply its own fg/bg. The
// leading char doubles as the mark glyph: "✓" for marked rows, space
// otherwise — so marks stay visible without stealing column width.
func renderDataRow(cols []column, d lv.Domain, h *domHistory, qga lv.GuestUptime, selected, marked bool, nCols int) string {
	if nCols > len(cols) {
		nCols = len(cols)
	}
	cells := make([]string, 0, nCols)
	for _, c := range cols[:nCols] {
		raw := c.render(d, h, qga)
		// Stripping ANSI is essential for selected rows: an inner
		// `\x1b[m` reset (from a coloured bar or state cell) would
		// terminate the rowSelected style mid-row.
		if selected {
			raw = ansi.Strip(raw)
		}
		var padded string
		if c.leftAlign {
			padded = padRight(raw, c.width)
		} else {
			padded = padLeft(raw, c.width)
		}
		// Colour the STATE cell by the domain state for non-selected
		// rows; everything else gets the theme's default foreground so
		// monochrome / phosphor / shades themes can colour the table
		// proper, not just the state column.
		if !selected {
			if c.sort == sortByState {
				padded = stateStyleFor(d.State).Render(padded)
			} else {
				padded = lipgloss.NewStyle().Foreground(colFG).Render(padded)
			}
		}
		cells = append(cells, padded)
	}
	row := strings.Join(cells, "  ")
	if selected {
		prefix := " "
		if marked {
			prefix = "✓"
		}
		return rowSelected.Render(prefix + row)
	}
	if marked {
		return markStyle.Render("✓") + row
	}
	return " " + row
}

// listBodyHeight returns how many data rows we can show in the list pane.
// Total terminal height minus header pane (width-aware: 9 wide, 18 narrow),
// list border (2), list header row (1), status bar (1), safety margin (1).
func (m Model) listBodyHeight() int {
	if m.height == 0 {
		return 10
	}
	chrome := 2 + 1 + 1 + 1 // list border top/bot + header row + status bar + margin
	h := m.height - m.headerPaneHeight() - chrome
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

// runeBackspace returns s with its last rune removed, or s unchanged if
// empty. Byte-wise slicing is unsafe for non-ASCII input (Swedish å, ö,
// Spanish ñ, etc.) which would be chopped mid-codepoint on backspace.
func runeBackspace(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return string(r[:len(r)-1])
}

// navSelect handles the common j/k/g/G/Home/End/arrow-key selection
// pattern used by the hosts, snapshots, networks, pools, volumes, and
// leases views. Mutates *sel in place, returns true if the key was
// recognised as a navigation key (the caller should then short-circuit).
func navSelect(key string, sel *int, n int) bool {
	switch key {
	case "j", "down":
		if *sel < n-1 {
			*sel++
		}
		return true
	case "k", "up":
		if *sel > 0 {
			*sel--
		}
		return true
	case "g", "home":
		*sel = 0
		return true
	case "G", "end":
		if n > 0 {
			*sel = n - 1
		}
		return true
	}
	return false
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
