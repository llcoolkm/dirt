package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// View is the root render — composes header, list, and status bar,
// or shows the detail / help / splash overlays as appropriate.
func (m Model) View() string {
	if m.err != nil && m.snap == nil {
		return errorStyle.Render("dirt: "+m.err.Error()) + "\n"
	}

	// First-tick splash before any snapshot has landed.
	if m.snap == nil {
		return m.splashView()
	}

	switch m.mode {
	case viewHelp:
		return m.helpView()
	case viewDetail:
		return m.detailView()
	case viewSnapshots:
		return m.snapshotsView()
	case viewNetworks:
		return m.networksView()
	case viewPools:
		return m.poolsView()
	case viewVolumes:
		return m.volumesView()
	}

	parts := []string{
		m.headerView(),
		m.listView(),
		m.statusView(),
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// splashView is shown during the first connection, before any snapshot has
// been received. Centred in the terminal so it does not jump on resize.
func (m Model) splashView() string {
	w := m.contentWidth()
	h := m.height
	if h == 0 {
		h = 24
	}

	logo := headerTitle.Render("dirt") + headerLabel.Render(" — David's virtual UI")
	sub := headerLabel.Render("connecting to libvirt") +
		flashStyle.Render("…")

	body := lipgloss.JoinVertical(lipgloss.Center, logo, "", sub)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colBorder).
		Padding(1, 4).
		Render(body)

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

// statusView renders the bottom status bar — flash, filter prompt, or key hints.
func (m Model) statusView() string {
	width := m.contentWidth()

	// Filter prompt takes precedence.
	if m.filtering {
		prompt := keyHint.Render("/") + " " + m.filter + lipgloss.NewStyle().Foreground(colAccent).Render("█")
		return statusBar.Width(width).Render(" " + prompt)
	}

	// Command palette.
	if m.commanding {
		prompt := keyHint.Render(":") + m.command + lipgloss.NewStyle().Foreground(colAccent).Render("█") +
			headerLabel.Render("    (try snap, vm, help, q)")
		return statusBar.Width(width).Render(" " + prompt)
	}

	// Confirm dialog takes precedence after filter.
	if m.confirming {
		msg := errorStyle.Render(fmt.Sprintf(" ⚠ %s %s? ", m.confirmAction, m.confirmName)) +
			keyHint.Render("y") + statusBar.Render(" to confirm, any other key to cancel")
		return statusBar.Width(width).Render(msg)
	}

	// Transient flash message after an action.
	if m.flash != "" && time.Now().Before(m.flashUntil) {
		return statusBar.Width(width).Render(" " + flashStyle.Render(m.flash))
	}

	return statusBar.Width(width).Render(" " + m.shortHelp())
}

// shortHelp is the always-on key hint line.
func (m Model) shortHelp() string {
	hints := []string{
		key("j/k") + " nav",
		key("s") + " start",
		key("S") + " stop",
		key("D") + " kill",
		key("r") + " reboot",
		key("c") + " console",
		key("e") + " edit",
		key("Enter") + " detail",
		key(":") + " cmd",
		key("/") + " filter",
		key("?") + " help",
		key("q") + " quit",
	}
	return statusBar.Render(strings.Join(hints, "  "))
}


// detailView shows the live XML of the selected domain (scrollable, searchable).
func (m Model) detailView() string {
	width := m.contentWidth()
	bodyH := m.detailBodyHeight()

	d, ok := m.currentDomain()
	totalLines := len(m.detailLines)
	maxScroll := totalLines - bodyH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.detailScroll > maxScroll {
		m.detailScroll = maxScroll
	}

	// Title row: name + position + search status + hint.
	name := ""
	if ok {
		name = d.Name
	}
	titleLeft := headerTitle.Render("detail: ") + headerValue.Render(name)

	// Position indicator: showing which lines are visible.
	end := m.detailScroll + bodyH
	if end > totalLines {
		end = totalLines
	}
	pct := 0
	if totalLines > 0 {
		pct = int(float64(end) / float64(totalLines) * 100)
	}
	pos := fmt.Sprintf("lines %d-%d/%d (%d%%)", m.detailScroll+1, end, totalLines, pct)
	posStr := headerLabel.Render("  ·  ") + headerValue.Render(pos)

	// Search status (if a search is active or we have matches).
	searchStr := ""
	if len(m.detailMatches) > 0 {
		searchStr = headerLabel.Render("  ·  ") +
			keyHint.Render("match ") +
			headerValue.Render(fmt.Sprintf("%d/%d", m.detailMatchIdx+1, len(m.detailMatches))) +
			headerLabel.Render(" “") + headerValue.Render(m.detailSearch) + headerLabel.Render("”")
	} else if m.detailSearch != "" && !m.detailSearching {
		searchStr = headerLabel.Render("  ·  ") +
			errorStyle.Render("no matches for “"+m.detailSearch+"”")
	}

	hint := headerLabel.Render("  ·  ") + keyHint.Render("?") + headerLabel.Render(" /:search  n/N:next/prev  pgup/dn:page  g/G:top/bot  esc:back")

	titleRow := titleLeft + posStr + searchStr
	// Hint goes on its own line if title is already long.
	titleRow = titleRow + hint

	// Body: render visible lines with search highlights.
	bodyLines := make([]string, 0, bodyH)
	currentMatchLine := -1
	if len(m.detailMatches) > 0 {
		currentMatchLine = m.detailMatches[m.detailMatchIdx]
	}
	for i := m.detailScroll; i < end; i++ {
		line := m.detailLines[i]
		if m.detailSearch != "" {
			line = highlightMatches(line, m.detailSearch, i == currentMatchLine)
		}
		bodyLines = append(bodyLines, line)
	}
	// Pad with blank lines so the box always has a consistent height.
	for len(bodyLines) < bodyH {
		bodyLines = append(bodyLines, "")
	}
	body := strings.Join(bodyLines, "\n")

	// Bottom prompt: shows the / search input when active, blank otherwise.
	prompt := ""
	if m.detailSearching {
		prompt = keyHint.Render("/") + " " + m.detailSearch + flashStyle.Render("█")
	}

	pane := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colBorder).
		Width(width - 2).
		Padding(0, 1).
		Render(lipgloss.JoinVertical(lipgloss.Left, titleRow, "", body, prompt))

	return pane
}

// highlightMatches returns the line with every case-insensitive occurrence of
// query wrapped in the match style. The "current" match (when current=true)
// gets the stronger matchCurrentStyle on the first occurrence in the line.
func highlightMatches(line, query string, current bool) string {
	if query == "" {
		return line
	}
	lq := strings.ToLower(query)
	ll := strings.ToLower(line)

	var b strings.Builder
	i := 0
	first := true
	for i < len(line) {
		idx := strings.Index(ll[i:], lq)
		if idx < 0 {
			b.WriteString(line[i:])
			break
		}
		b.WriteString(line[i : i+idx])
		match := line[i+idx : i+idx+len(query)]
		if current && first {
			b.WriteString(matchCurrentStyle.Render(match))
			first = false
		} else {
			b.WriteString(matchStyle.Render(match))
		}
		i += idx + len(query)
	}
	return b.String()
}

func key(s string) string { return keyHint.Render(s) }
