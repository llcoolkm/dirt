package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// helpView renders the full-screen help modal shown when ? is pressed.
func (m Model) helpView() string {
	width := m.contentWidth()
	height := m.height
	if height == 0 {
		height = 24
	}

	title := headerTitle.Render("dirt — help")
	subtitle := headerLabel.Render("press ") + keyHint.Render("?") +
		headerLabel.Render(" or ") + keyHint.Render("esc") + headerLabel.Render(" to dismiss")

	sections := []helpSection{
		{
			title: "Navigation",
			rows: []helpRow{
				{"j / ↓", "move down"},
				{"k / ↑", "move up"},
				{"g / Home", "jump to top"},
				{"G / End", "jump to bottom"},
				{"Ctrl-d / PgDn", "page down"},
				{"Ctrl-u / PgUp", "page up"},
			},
		},
		{
			title: "Filter & Search",
			rows: []helpRow{
				{"/", "filter VM list by substring"},
				{"Esc", "clear filter"},
			},
		},
		{
			title: "Sort (numbers match column order)",
			rows: []helpRow{
				{"1", "sort by name"},
				{"2", "sort by state"},
				{"3", "sort by IP"},
				{"4", "sort by OS"},
				{"5", "sort by vCPU count"},
				{"6", "sort by allocated memory"},
				{"7", "sort by used memory %"},
				{"8", "sort by CPU%"},
				{"9", "sort by uptime"},
				{"(same key)", "press again to reverse direction"},
			},
		},
		{
			title: "Lifecycle (selected VM)",
			rows: []helpRow{
				{"s", "start (if stopped)"},
				{"S", "graceful shutdown"},
				{"D", "destroy — force off (asks y to confirm)"},
				{"r", "reboot"},
				{"p", "pause"},
				{"R", "resume from pause"},
				{"c", "open serial console (Ctrl-] to detach)"},
				{"e", "edit XML in $EDITOR (virsh edit)"},
				{"x", "undefine — delete a stopped VM (asks y)"},
			},
		},
		{
			title: "Command palette & Views",
			rows: []helpRow{
				{":", "open command palette"},
				{":snap", "snapshots of selected VM"},
				{":net", "libvirt networks"},
				{":pool", "storage pools"},
				{":vm", "back to VM list"},
				{":help", "this help screen"},
				{":q", "quit"},
			},
		},
		{
			title: "Snapshots view",
			rows: []helpRow{
				{"j / k", "navigate snapshots"},
				{"c", "create snapshot (prompts for name)"},
				{"r", "revert to snapshot (asks y)"},
				{"D / x", "delete snapshot (asks y)"},
				{"R / F5", "refresh list"},
				{"esc / q", "back to VM list"},
			},
		},
		{
			title: "Networks view",
			rows: []helpRow{
				{"j / k", "navigate networks"},
				{"s / S", "start / stop network"},
				{"a", "toggle autostart"},
				{"R / F5", "refresh list"},
				{"esc / q", "back to VM list"},
			},
		},
		{
			title: "Pools / Volumes view",
			rows: []helpRow{
				{"j / k", "navigate pools/volumes"},
				{"s / S", "start / stop pool"},
				{"Enter / d", "drill into pool's volumes"},
				{"R / F5", "refresh"},
				{"esc / q", "back"},
			},
		},
		{
			title: "Detail view",
			rows: []helpRow{
				{"d / Enter", "open detail (live XML)"},
				{"j/k / arrows", "scroll by line"},
				{"PgUp/PgDn / ←/→", "scroll by page"},
				{"g / Home", "top of XML"},
				{"G / End", "bottom of XML"},
				{"/", "incremental search"},
				{"n / N", "next / previous match"},
				{"Esc", "clear search; second Esc closes detail"},
				{"q", "close detail"},
			},
		},
		{
			title: "Application",
			rows: []helpRow{
				{"?", "toggle this help"},
				{"q / Ctrl-c", "quit dirt"},
			},
		},
	}

	// Render all sections in two columns to use horizontal space.
	col1Sections := sections[:len(sections)/2+len(sections)%2]
	col2Sections := sections[len(sections)/2+len(sections)%2:]

	col1 := renderHelpColumn(col1Sections)
	col2 := renderHelpColumn(col2Sections)

	colW := (width - 6) / 2
	col1Box := lipgloss.NewStyle().Width(colW).Render(col1)
	col2Box := lipgloss.NewStyle().Width(colW).Render(col2)

	body := lipgloss.JoinHorizontal(lipgloss.Top, col1Box, col2Box)

	footer := headerLabel.Render("dirt — David's virtual UI · libvirt TUI in the htop tradition")

	pane := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colBorder).
		Width(width - 2).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			title+"   "+subtitle,
			"",
			body,
			"",
			footer,
		))

	return pane
}

type helpSection struct {
	title string
	rows  []helpRow
}

type helpRow struct {
	key  string
	desc string
}

func renderHelpColumn(sections []helpSection) string {
	var b strings.Builder
	for i, sec := range sections {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(headerTitle.Render(sec.title))
		b.WriteString("\n")
		for _, row := range sec.rows {
			b.WriteString("  ")
			b.WriteString(keyHint.Render(padRight(row.key, 18)))
			b.WriteString(" ")
			b.WriteString(headerValue.Render(row.desc))
			b.WriteString("\n")
		}
	}
	return b.String()
}
