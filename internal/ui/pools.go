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

	title := headerTitle.Render("storage pools") +
		headerLabel.Render("    ") +
		keyHint.Render("s") + headerLabel.Render(" start  ") +
		keyHint.Render("S") + headerLabel.Render(" stop  ") +
		keyHint.Render("⏎") + headerLabel.Render(" volumes  ") +
		keyHint.Render("R") + headerLabel.Render(" refresh  ") +
		keyHint.Render("esc") + headerLabel.Render(" back")

	header := listHeaderRow.Render(strings.Join([]string{
		padRight("NAME", poolNameW),
		padRight("STATE", poolStateW),
		padRight("TYPE", poolTypeW),
		padLeft("CAPACITY", poolCapW),
		padLeft("USED", poolAllocW),
		padLeft("FREE", poolFreeW),
		padRight("USAGE", poolUsageW),
	}, "  "))

	rows := []string{header}
	if m.poolsErr != nil {
		rows = append(rows, "", errorStyle.Render("  error: "+m.poolsErr.Error()))
	} else if len(m.pools) == 0 {
		rows = append(rows, "", lipgloss.NewStyle().Foreground(colDimmed).Italic(true).Render("  no storage pools"))
	} else {
		for i, p := range m.pools {
			rows = append(rows, renderPoolRow(p, i == m.poolsSel))
		}
	}

	pane := listBox.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left,
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
	stateColored := style.Render(padRight(state, poolStateW))

	cap := formatBytes(float64(p.Capacity))
	alloc := formatBytes(float64(p.Allocation))
	free := formatBytes(float64(p.Available))

	usagePct := 0.0
	if p.Capacity > 0 {
		usagePct = float64(p.Allocation) / float64(p.Capacity) * 100
	}
	usageBar := colorBar(usagePct, poolUsageW-7) +
		fmt.Sprintf(" %3.0f%%", usagePct)

	cols := []string{
		padRight(truncate(p.Name, poolNameW), poolNameW),
		stateColored,
		padRight(truncate(p.Type, poolTypeW), poolTypeW),
		padLeft(cap, poolCapW),
		padLeft(alloc, poolAllocW),
		padLeft(free, poolFreeW),
		usageBar,
	}
	row := strings.Join(cols, "  ")
	if selected {
		cols[1] = padRight(state, poolStateW)
		row = strings.Join(cols, "  ")
		return rowSelected.Render(" " + row)
	}
	return " " + row
}

func poolsStatusBar(m Model, width int) string {
	if m.flash != "" && time.Now().Before(m.flashUntil) {
		return statusBar.Width(width).Render(" " + flashStyle.Render(m.flash))
	}
	return statusBar.Width(width).Render(" " +
		key("j/k") + " nav  " + key("s") + " start  " + key("S") + " stop  " +
		key("⏎") + " volumes  " + key("R") + " refresh  " + key("esc") + " back")
}

// volumesView renders the volumes inside a storage pool.
func (m Model) volumesView() string {
	width := m.contentWidth()

	title := headerTitle.Render("volumes: ") + headerValue.Render(m.volumesFor) +
		headerLabel.Render("    ") +
		keyHint.Render("R") + headerLabel.Render(" refresh  ") +
		keyHint.Render("esc") + headerLabel.Render(" back to pools")

	header := listHeaderRow.Render(strings.Join([]string{
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

	pane := listBox.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left,
		append([]string{title, ""}, rows...)...))

	bottom := statusBar.Width(width).Render(" " +
		key("j/k") + " nav  " + key("R") + " refresh  " + key("esc") + " back")
	return lipgloss.JoinVertical(lipgloss.Left, pane, bottom)
}

func renderVolumeRow(v lv.StorageVolume, selected bool) string {
	cols := []string{
		padRight(truncate(v.Name, volNameW), volNameW),
		padRight(truncate(v.Type, volTypeW), volTypeW),
		padLeft(formatBytes(float64(v.Capacity)), volCapW),
		padLeft(formatBytes(float64(v.Allocation)), volAllocW),
		padRight(truncate(v.Path, volPathW), volPathW),
	}
	row := strings.Join(cols, "  ")
	if selected {
		return rowSelected.Render(" " + row)
	}
	return " " + row
}
