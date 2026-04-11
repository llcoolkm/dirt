package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/llcoolkm/dirt/internal/lv"
)

const (
	netNameW    = 18
	netStateW   = 9
	netAutoW    = 10
	netBridgeW  = 12
	netForwardW = 10
	netLeasesW  = 7
)

// networksView renders the libvirt networks list.
func (m Model) networksView() string {
	width := m.contentWidth()

	title := headerTitle.Render("networks") +
		headerLabel.Render("    ") +
		keyHint.Render("s") + headerLabel.Render(" start  ") +
		keyHint.Render("S") + headerLabel.Render(" stop  ") +
		keyHint.Render("a") + headerLabel.Render(" autostart  ") +
		keyHint.Render("R") + headerLabel.Render(" refresh  ") +
		keyHint.Render("esc") + headerLabel.Render(" back")

	header := listHeaderRow.Render(" " + strings.Join([]string{
		padRight("NAME", netNameW),
		padRight("STATE", netStateW),
		padRight("AUTOSTART", netAutoW),
		padRight("BRIDGE", netBridgeW),
		padRight("FORWARD", netForwardW),
		padLeft("LEASES", netLeasesW),
	}, "  "))

	rows := []string{header}
	if m.networksErr != nil {
		rows = append(rows, "", errorStyle.Render("  error: "+m.networksErr.Error()))
	} else if len(m.networks) == 0 {
		rows = append(rows, "", lipgloss.NewStyle().Foreground(colDimmed).Italic(true).Render("  no networks defined"))
	} else {
		for i, n := range m.networks {
			rows = append(rows, renderNetworkRow(n, i == m.networksSel))
		}
	}

	pane := listBox.Width(width - borderWidth).Render(lipgloss.JoinVertical(lipgloss.Left,
		append([]string{title, ""}, rows...)...))

	bottom := networkStatusBar(m, width)
	return lipgloss.JoinVertical(lipgloss.Left, pane, bottom)
}

func renderNetworkRow(n lv.Network, selected bool) string {
	state := "inactive"
	style := stateShutoff
	if n.Active {
		state = "active"
		style = stateRunning
	}
	auto := "no"
	if n.Autostart {
		auto = "yes"
	}
	leases := "—"
	if n.Active {
		leases = fmt.Sprintf("%d", n.NumLeases)
	}
	stateColored := style.Render(padRight(state, netStateW))

	cols := []string{
		padRight(truncate(n.Name, netNameW), netNameW),
		stateColored,
		padRight(auto, netAutoW),
		padRight(truncate(n.Bridge, netBridgeW), netBridgeW),
		padRight(truncate(n.Forward, netForwardW), netForwardW),
		padLeft(leases, netLeasesW),
	}
	row := strings.Join(cols, "  ")
	if selected {
		cols[1] = padRight(state, netStateW)
		row = strings.Join(cols, "  ")
		return rowSelected.Render(" " + row)
	}
	return " " + row
}

func networkStatusBar(m Model, width int) string {
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
		key("a") + " autostart  " + key("R") + " refresh  " + key("esc") + " back")
}
