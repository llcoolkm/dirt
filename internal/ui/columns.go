package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
)

// columnsView renders the column-visibility picker. SPACE toggles the
// row under the cursor; required columns refuse to toggle (they would
// hide critical fields). Esc returns to the main list.
func (m Model) columnsView() string {
	width := m.contentWidth()
	title := headerTitle.Render("columns") +
		headerLabel.Render("  ·  ") +
		headerValue.Render("which columns appear in the VM list")

	header := listHeaderRow.Render(" " + strings.Join([]string{
		padRight("ON", 4),
		padRight("ID", 12),
		padRight("LABEL", 10),
		padRight("NOTE", 30),
	}, "  "))

	rows := []string{title, "", header}
	visibility := m.currentColumnVisibility()
	fg := lipgloss.NewStyle().Foreground(colFG)
	for i, c := range vmColumns {
		on := "✓"
		if !visibility[c.id] {
			on = " "
		}
		note := ""
		if c.required {
			note = "(required — always shown)"
		}
		cells := []string{
			markStyle.Render(padRight(on, 4)),
			fg.Render(padRight(c.id, 12)),
			fg.Render(padRight(c.label, 10)),
			fg.Render(padRight(note, 30)),
		}
		row := strings.Join(cells, "  ")
		if i == m.columnsSel {
			cells[0] = padRight(on, 4)
			row = strings.Join(cells, "  ")
			rows = append(rows, rowSelected.Render(" "+row))
		} else {
			rows = append(rows, " "+row)
		}
	}

	pane := listBox.Width(width - borderWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left, rows...))

	bottom := statusBar.Width(width).Render(" " +
		key("j/k") + statusBar.Render(" nav  ") +
		key("space") + statusBar.Render(" toggle  ") +
		key("a") + statusBar.Render(" all  ") +
		key("n") + statusBar.Render(" none  ") +
		key("esc") + statusBar.Render(" back"))
	return lipgloss.JoinVertical(lipgloss.Left, pane, bottom)
}

// currentColumnVisibility derives a {id: visible} map from
// activeColumns: any column present in activeColumns is visible,
// any in vmColumns but absent from activeColumns is hidden.
func (m Model) currentColumnVisibility() map[string]bool {
	out := make(map[string]bool, len(vmColumns))
	for _, c := range vmColumns {
		out[c.id] = false
	}
	for _, c := range m.activeColumns {
		out[c.id] = true
	}
	return out
}

func (m Model) handleColumnsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(vmColumns)
	switch msg.String() {
	case "esc", "q":
		m.mode = viewMain
		return m, nil
	case "j", "down":
		if m.columnsSel < n-1 {
			m.columnsSel++
		}
		return m, nil
	case "k", "up":
		if m.columnsSel > 0 {
			m.columnsSel--
		}
		return m, nil
	case "g", "home":
		m.columnsSel = 0
		return m, nil
	case "G", "end":
		m.columnsSel = n - 1
		return m, nil
	case " ", "enter":
		if m.columnsSel < 0 || m.columnsSel >= n {
			return m, nil
		}
		c := vmColumns[m.columnsSel]
		if c.required {
			m.flashf("%s is required — cannot hide", c.id)
			return m, nil
		}
		vis := m.currentColumnVisibility()
		vis[c.id] = !vis[c.id]
		m.activeColumns = filterActiveColumns(vmColumns, vis)
		return m, nil
	case "a":
		// Show every column.
		vis := make(map[string]bool, len(vmColumns))
		for _, c := range vmColumns {
			vis[c.id] = true
		}
		m.activeColumns = filterActiveColumns(vmColumns, vis)
		return m, nil
	case "n":
		// Hide every non-required column.
		vis := make(map[string]bool, len(vmColumns))
		for _, c := range vmColumns {
			vis[c.id] = false
		}
		m.activeColumns = filterActiveColumns(vmColumns, vis)
		return m, nil
	}
	return m, nil
}

// flashColumnSummary is used when the master types `:columns` from
// outside the view; left here for the curious palette user. (Currently
// unused — the palette handler routes to viewColumns directly.)
func (m Model) flashColumnSummary() string {
	visible := make([]string, 0)
	hidden := make([]string, 0)
	vis := m.currentColumnVisibility()
	for _, c := range vmColumns {
		if vis[c.id] {
			visible = append(visible, c.id)
		} else {
			hidden = append(hidden, c.id)
		}
	}
	return fmt.Sprintf("visible: %s; hidden: %s",
		strings.Join(visible, ","), strings.Join(hidden, ","))
}
