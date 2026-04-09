package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/llcoolkm/dirt/internal/lv"
)

// Column widths for the snapshot table.
const (
	snapNameW    = 28
	snapStateW   = 10
	snapSizeW    = 8
	snapParentW  = 18
	snapCurrentW = 8
	snapWhenW    = 19
)

// snapshotsView renders the per-domain snapshot list.
func (m Model) snapshotsView() string {
	width := m.contentWidth()

	title := headerTitle.Render("snapshots: ") + headerValue.Render(m.snapshotsFor) +
		headerLabel.Render("    ") +
		keyHint.Render("c") + headerLabel.Render(" create  ") +
		keyHint.Render("r") + headerLabel.Render(" revert  ") +
		keyHint.Render("D") + headerLabel.Render(" delete  ") +
		keyHint.Render("R") + headerLabel.Render(" refresh  ") +
		keyHint.Render("esc") + headerLabel.Render(" back")

	// Header row. Leading space matches the per-row indent below.
	headerRow := listHeaderRow.Render(" " + strings.Join([]string{
		padRight("NAME", snapNameW),
		padRight("STATE", snapStateW),
		padLeft("SIZE", snapSizeW),
		padRight("PARENT", snapParentW),
		padRight("CURRENT", snapCurrentW),
		padRight("CREATED", snapWhenW),
	}, "  "))

	// Body.
	var rows []string
	rows = append(rows, headerRow)

	if m.snapshotsErr != nil {
		rows = append(rows, "")
		rows = append(rows, errorStyle.Render("  error: "+m.snapshotsErr.Error()))
	} else if len(m.snapshots) == 0 {
		rows = append(rows, "")
		rows = append(rows, lipgloss.NewStyle().Foreground(colDimmed).Italic(true).
			Render("  no snapshots — press "+keyHint.Render("c")+" to create one"))
	} else {
		for i, s := range m.snapshots {
			rows = append(rows, renderSnapshotRow(s, i == m.snapshotsSel))
		}
	}

	pane := listBox.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, append([]string{title, ""}, rows...)...))

	// Status / input prompt at the bottom.
	var bottom string
	if m.snapshotInput {
		prompt := keyHint.Render("name: ") + m.snapshotName +
			lipgloss.NewStyle().Foreground(colAccent).Render("█") +
			headerLabel.Render("   (a-z 0-9 _ - . only · enter to create, esc to cancel)")
		bottom = statusBar.Width(width).Render(" " + prompt)
	} else if m.confirming {
		msg := errorStyle.Render(fmt.Sprintf(" ⚠ %s snapshot “%s”? ", m.confirmAction, m.confirmName)) +
			keyHint.Render("y") + statusBar.Render(" to confirm, any other key to cancel")
		bottom = statusBar.Width(width).Render(msg)
	} else if m.flash != "" && time.Now().Before(m.flashUntil) {
		bottom = statusBar.Width(width).Render(" " + flashStyle.Render(m.flash))
	} else {
		bottom = statusBar.Width(width).Render(" " +
			key("j/k") + " nav  " + key("c") + " create  " + key("r") + " revert  " +
			key("D") + " delete  " + key("esc") + " back  " + key("?") + " help")
	}

	return lipgloss.JoinVertical(lipgloss.Left, pane, bottom)
}

// renderSnapshotRow renders one snapshot row, optionally highlighted.
func renderSnapshotRow(s lv.DomainSnapshot, selected bool) string {
	current := ""
	if s.IsCurrent {
		current = "*"
	}
	when := ""
	if !s.CreatedAt.IsZero() {
		when = s.CreatedAt.Format("2006-01-02 15:04:05")
	}
	size := "—"
	if s.SizeBytes > 0 {
		size = formatBytes(float64(s.SizeBytes))
	}
	state := stateColorBySnapshotState(s.State).Render(padRight(s.State, snapStateW))

	cols := []string{
		padRight(truncate(s.Name, snapNameW), snapNameW),
		state,
		padLeft(size, snapSizeW),
		padRight(truncate(s.Parent, snapParentW), snapParentW),
		padRight(current, snapCurrentW),
		padRight(when, snapWhenW),
	}
	row := strings.Join(cols, "  ")
	if selected {
		// Strip color in selected mode for consistent inversion.
		cols[1] = padRight(s.State, snapStateW)
		row = strings.Join(cols, "  ")
		return rowSelected.Render(" " + row)
	}
	return " " + row
}

// stateColorBySnapshotState colours snapshot states (running/shutoff/paused).
func stateColorBySnapshotState(s string) lipgloss.Style {
	switch s {
	case "running":
		return stateRunning
	case "paused", "pmsuspended":
		return statePaused
	case "shutoff", "shutdown", "":
		return stateShutoff
	case "crashed":
		return stateCrashed
	}
	return headerValue
}
