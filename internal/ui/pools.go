package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/llcoolkm/dirt/internal/lv"
)

const (
	poolNameW   = 18
	poolStateW  = 10
	poolTypeW   = 7
	poolCapW    = 9
	poolAllocW  = 9
	poolFreeW   = 9
	poolUsageW  = 18

	volNameW  = 32
	volTypeW  = 7
	volCapW   = 10
	volAllocW = 10
	volPathW  = 40
)

// poolsView renders the storage pools list.
func (m Model) poolsView() string {
	width := m.contentWidth()

	title := headerTitle.Render("storage pools")

	active := m.poolsSortIdx
	desc := m.poolsSortDesc
	header := listHeaderRow.Render(" " + strings.Join([]string{
		padRight(arrowedHeader("NAME", active == poolColName, desc), poolNameW),
		padRight(arrowedHeader("STATE", active == poolColState, desc), poolStateW),
		padRight(arrowedHeader("TYPE", active == poolColType, desc), poolTypeW),
		padLeft(arrowedHeader("CAPACITY", active == poolColCap, desc), poolCapW),
		padLeft(arrowedHeader("USED", active == poolColAlloc, desc), poolAllocW),
		padLeft(arrowedHeader("FREE", active == poolColFree, desc), poolFreeW),
		padRight(arrowedHeader("USAGE", active == poolColUsage, desc), poolUsageW),
	}, "  "))

	rows := []string{header}
	if m.poolsErr != nil {
		rows = append(rows, "", errorStyle.Render("  error: "+m.poolsErr.Error()))
	} else if len(m.pools) == 0 {
		rows = append(rows, "", lipgloss.NewStyle().Foreground(colDimmed).Italic(true).Render("  no storage pools"))
	} else {
		sorted := m.sortedPools()
		for i, p := range sorted {
			rows = append(rows, renderPoolRow(p, i == m.poolsSel))
		}
	}

	pane := listBox.Width(width - borderWidth).Render(lipgloss.JoinVertical(lipgloss.Left,
		append([]string{title, ""}, rows...)...))

	bottom := poolsStatusBar(m, width)
	return lipgloss.JoinVertical(lipgloss.Left, pane, bottom)
}

func renderPoolRow(p lv.StoragePool, selected bool) string {
	state := p.State
	style := stateShutoff
	if state == "running" {
		style = stateRunning
	} else if state == "degraded" || state == "inaccessible" {
		style = stateCrashed
	}

	cap := formatBytes(float64(p.Capacity))
	alloc := formatBytes(float64(p.Allocation))
	free := formatBytes(float64(p.Available))

	usagePct := 0.0
	if p.Capacity > 0 {
		usagePct = float64(p.Allocation) / float64(p.Capacity) * 100
	}

	// Selected rows must carry no inner ANSI — any `\x1b[m` reset
	// inside a cell terminates the rowSelected wrap mid-row. Build
	// a plain bar (no colour) at the same visible width as the
	// non-selected branch so column alignment doesn't drift.
	if selected {
		pctStr := fmt.Sprintf(" %3.0f%%", usagePct)
		row := strings.Join([]string{
			padRight(truncate(p.Name, poolNameW), poolNameW),
			padRight(state, poolStateW),
			padRight(truncate(p.Type, poolTypeW), poolTypeW),
			padLeft(cap, poolCapW),
			padLeft(alloc, poolAllocW),
			padLeft(free, poolFreeW),
			bar(usagePct, poolUsageW-7) + pctStr,
		}, "  ")
		return rowSelected.Render(" " + row)
	}

	stateColored := style.Render(padRight(state, poolStateW))
	pctStr := fmt.Sprintf(" %3.0f%%", usagePct)
	if usagePct >= 95 {
		pctStr = errorStyle.Render(pctStr)
	} else if usagePct >= 80 {
		pctStr = lipgloss.NewStyle().Foreground(colPaused).Bold(true).Render(pctStr)
	}
	usageBar := storageColorBar(usagePct, poolUsageW-7) + pctStr

	fg := lipgloss.NewStyle().Foreground(colFG)
	cols := []string{
		fg.Render(padRight(truncate(p.Name, poolNameW), poolNameW)),
		stateColored,
		fg.Render(padRight(truncate(p.Type, poolTypeW), poolTypeW)),
		fg.Render(padLeft(cap, poolCapW)),
		fg.Render(padLeft(alloc, poolAllocW)),
		fg.Render(padLeft(free, poolFreeW)),
		usageBar,
	}
	return " " + strings.Join(cols, "  ")
}

func poolsStatusBar(m Model, width int) string {
	if m.confirming {
		label := friendlyConfirmAction(m.confirmAction)
		msg := errorStyle.Render(fmt.Sprintf(" ⚠ %s “%s”? ", label, m.confirmName)) +
			keyHint.Render("y") + statusBar.Render(" to confirm, any other key to cancel")
		return statusBar.Width(width).Render(msg)
	}
	if m.flash != "" && time.Now().Before(m.flashUntil) {
		return statusBar.Width(width).Render(" " + flashStyle.Render(m.flash))
	}
	return statusBar.Width(width).Render(" " +
		key("j/k") + " nav  " + key("s") + " start  " + key("S") + " stop  " +
		key("Enter") + " volumes  " + key("R") + " refresh  " + key("esc") + " back")
}

// friendlyConfirmAction expands an internal action code into a human label
// for the confirm prompt. Used by network and pool views.
func friendlyConfirmAction(action string) string {
	switch action {
	case "stop-net":
		return "stop network"
	case "stop-pool":
		return "stop pool"
	case "delete-snap":
		return "delete snapshot"
	}
	return action
}

// volumesView renders the volumes inside a storage pool.
func (m Model) volumesView() string {
	width := m.contentWidth()

	title := headerTitle.Render("volumes: ") + headerValue.Render(m.volumesFor)

	header := listHeaderRow.Render(" " + strings.Join([]string{
		padRight("NAME", volNameW),
		padRight("TYPE", volTypeW),
		padLeft("CAPACITY", volCapW),
		padLeft("ALLOCATED", volAllocW),
		padRight("PATH", volPathW),
	}, "  "))

	rows := []string{header}
	if m.volumesErr != nil {
		rows = append(rows, "", errorStyle.Render("  error: "+m.volumesErr.Error()))
	} else if len(m.volumes) == 0 {
		rows = append(rows, "", lipgloss.NewStyle().Foreground(colDimmed).Italic(true).Render("  no volumes in this pool"))
	} else {
		for i, v := range m.volumes {
			rows = append(rows, renderVolumeRow(v, i == m.volumesSel))
		}
	}

	pane := listBox.Width(width - borderWidth).Render(lipgloss.JoinVertical(lipgloss.Left,
		append([]string{title, ""}, rows...)...))

	bottom := volumeStatusBar(m, width)
	return lipgloss.JoinVertical(lipgloss.Left, pane, bottom)
}

func volumeStatusBar(m Model, width int) string {
	cursor := lipgloss.NewStyle().Foreground(colAccent).Render("█")
	if m.volInputStage == 1 {
		prompt := keyHint.Render("name: ") + m.volInputName + cursor +
			headerLabel.Render("   (enter → size, esc to cancel)")
		return statusBar.Width(width).Render(" " + prompt)
	}
	if m.volInputStage == 2 {
		prompt := keyHint.Render("size: ") + m.volInputSize + cursor +
			headerLabel.Render("   (e.g. 10G, 500M — enter to create, esc to cancel)")
		return statusBar.Width(width).Render(" " + prompt)
	}
	if m.confirming {
		msg := errorStyle.Render(fmt.Sprintf(" ⚠ delete volume “%s”? ", m.confirmName)) +
			keyHint.Render("y") + statusBar.Render(" to confirm, any other key to cancel")
		return statusBar.Width(width).Render(msg)
	}
	return statusBar.Width(width).Render(" " +
		key("j/k") + " nav  " + key("c") + " create  " + key("D") + " delete  " +
		key("R") + " refresh  " + key("esc") + " back")
}

func renderVolumeRow(v lv.StorageVolume, selected bool) string {
	if selected {
		row := strings.Join([]string{
			padRight(truncate(v.Name, volNameW), volNameW),
			padRight(truncate(v.Type, volTypeW), volTypeW),
			padLeft(formatBytes(float64(v.Capacity)), volCapW),
			padLeft(formatBytes(float64(v.Allocation)), volAllocW),
			padRight(truncate(v.Path, volPathW), volPathW),
		}, "  ")
		return rowSelected.Render(" " + row)
	}
	fg := lipgloss.NewStyle().Foreground(colFG)
	cols := []string{
		fg.Render(padRight(truncate(v.Name, volNameW), volNameW)),
		fg.Render(padRight(truncate(v.Type, volTypeW), volTypeW)),
		fg.Render(padLeft(formatBytes(float64(v.Capacity)), volCapW)),
		fg.Render(padLeft(formatBytes(float64(v.Allocation)), volAllocW)),
		fg.Render(padRight(truncate(v.Path, volPathW), volPathW)),
	}
	return " " + strings.Join(cols, "  ")
}
