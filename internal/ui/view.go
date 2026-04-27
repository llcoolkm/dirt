package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
	case viewInfo:
		return m.infoView()
	case viewDetail:
		return m.detailView()
	case viewGraphs:
		return m.graphsView()
	case viewSnapshots:
		return m.snapshotsView()
	case viewNetworks:
		return m.networksView()
	case viewPools:
		return m.poolsView()
	case viewVolumes:
		return m.volumesView()
	case viewLeases:
		return m.leasesView()
	case viewJobs:
		return m.jobsView()
	case viewMigrate:
		return m.migrateView()
	case viewHosts:
		return m.hostsView()
	case viewColumns:
		return m.columnsView()
	}

	parts := []string{
		m.headerView(),
		m.listView(),
		m.statusView(),
	}
	out := lipgloss.JoinVertical(lipgloss.Left, parts...)

	// Hard-clip to terminal dimensions so overlong lines never cause
	// the terminal to wrap (Bubble Tea can't track those visual wraps,
	// leading to ghost duplicates on resize). Truncate wide lines first,
	// then clip to height.
	if m.width > 0 || m.height > 0 {
		lines := strings.Split(out, "\n")
		if m.width > 0 {
			for i, line := range lines {
				if ansi.StringWidth(line) > m.width {
					lines[i] = ansi.Truncate(line, m.width, "")
				}
			}
		}
		if m.height > 0 && len(lines) > m.height {
			lines = lines[:m.height]
		}
		out = strings.Join(lines, "\n")
	}
	return out
}

// splashView is shown during the first connection, before any snapshot has
// been received. Centred in the terminal so it does not jump on resize.
func (m Model) splashView() string {
	w := m.contentWidth()
	h := m.height
	if h == 0 {
		h = 24
	}

	logo := headerTitle.Render("dirt") + headerLabel.Render(" — libvirt TUI")
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

	// Command palette — show the typed buffer plus a filtered list
	// of available commands so the master need not memorise them.
	if m.commanding {
		prompt := keyHint.Render(":") + m.command + lipgloss.NewStyle().Foreground(colAccent).Render("█")
		hint := commandPaletteHint(m.command)
		return statusBar.Width(width).Render(" " + prompt + "    " + hint)
	}

	// Attach / detach device prompt.
	if m.attachStage > 0 {
		var prompt string
		cursor := lipgloss.NewStyle().Foreground(colAccent).Render("█")
		verb := m.attachVerb
		if verb == "" {
			verb = "attach"
		}
		switch m.attachStage {
		case 1:
			prompt = keyHint.Render(verb+" on "+m.attachDomain+": ") +
				key("d") + headerLabel.Render("isk  ") +
				key("n") + headerLabel.Render("ic  ") +
				key("esc") + headerLabel.Render(" cancel")
		case 2:
			if verb == "detach" {
				if m.attachType == "disk" {
					prompt = keyHint.Render("target dev: ") + m.attachParam1 + cursor +
						headerLabel.Render("   (e.g. vdb — enter to detach, esc cancel)")
				} else {
					prompt = keyHint.Render("MAC: ") + m.attachParam1 + cursor +
						headerLabel.Render("   (e.g. 52:54:00:… — enter to detach, esc cancel)")
				}
			} else if m.attachType == "disk" {
				prompt = keyHint.Render("disk path: ") + m.attachParam1 + cursor +
					headerLabel.Render("   (enter → target, esc cancel)")
			} else {
				prompt = keyHint.Render("network: ") + m.attachParam1 + cursor +
					headerLabel.Render("   (enter to attach, esc cancel)")
			}
		case 3:
			prompt = keyHint.Render("target: ") + m.attachParam2 + cursor +
				headerLabel.Render("   (e.g. vdb, vdc — enter to attach, esc cancel)")
		}
		return statusBar.Width(width).Render(" " + prompt)
	}

	// Clone name prompt.
	if m.cloneFrom {
		prompt := keyHint.Render("clone ") + headerValue.Render(m.cloneSrc) +
			headerLabel.Render(" → ") + m.cloneName +
			lipgloss.NewStyle().Foreground(colAccent).Render("█") +
			headerLabel.Render("   (enter to clone, esc to cancel)")
		return statusBar.Width(width).Render(" " + prompt)
	}

	// Typed confirmation for bulk-undefine above the ceiling.
	if m.confirmTyping {
		cursor := lipgloss.NewStyle().Foreground(colAccent).Render("█")
		prompt := errorStyle.Render(fmt.Sprintf(" ⚠ type “%s” to confirm (add “ delete” to also remove storage): ",
			m.confirmTypingExpect)) +
			m.confirmTypingInput + cursor
		return statusBar.Width(width).Render(prompt)
	}

	// Confirm dialog takes precedence after filter.
	if m.confirming {
		target := m.confirmName
		if m.confirmBulk {
			target = fmt.Sprintf("%d VMs", len(m.confirmTargets))
			if h := m.marksHiddenByFilter(); h > 0 {
				target = fmt.Sprintf("%d VMs (%d hidden by filter)", len(m.confirmTargets), h)
			}
		}
		if m.confirmAction == "undefine" {
			msg := errorStyle.Render(fmt.Sprintf(" ⚠ undefine %s? ", target)) +
				keyHint.Render("y") + statusBar.Render(" keep disks  ") +
				keyHint.Render("d") + statusBar.Render(" delete disks too  ") +
				statusBar.Render("any other key to cancel")
			return statusBar.Width(width).Render(msg)
		}
		msg := errorStyle.Render(fmt.Sprintf(" ⚠ %s %s? ", m.confirmAction, target)) +
			keyHint.Render("y") + statusBar.Render(" to confirm, any other key to cancel")
		return statusBar.Width(width).Render(msg)
	}

	// Transient flash message after an action.
	if m.flash != "" && time.Now().Before(m.flashUntil) {
		return statusBar.Width(width).Render(" " + flashStyle.Render(m.flash))
	}

	// Active background jobs take precedence over the key-hint line
	// so they're always visible. :jobs gives the full view.
	if seg := m.jobsStatusSegment(width - 2); seg != "" {
		return statusBar.Width(width).Render(withMarkIndicator(" "+seg, m, width))
	}

	return statusBar.Width(width).Render(withMarkIndicator(" "+m.shortHelp(), m, width))
}

// withMarkIndicator pins a vim-style tag to the right edge of the
// status bar. Shows a pending numeric prefix and/or the mark count.
// On narrow terminals the left side is truncated rather than the
// indicator — losing hints is less harmful than hiding the mode.
func withMarkIndicator(left string, m Model, width int) string {
	parts := []string{}
	if m.pendingCount > 0 {
		parts = append(parts, keyHint.Render(fmt.Sprintf("%d", m.pendingCount)))
	}
	if m.markCount() > 0 {
		parts = append(parts, markStyle.Render(fmt.Sprintf("✓ %d marked", m.markCount())))
	}
	if len(parts) == 0 {
		return left
	}
	tag := strings.Join(parts, "  ")
	tagW := ansi.StringWidth(tag)
	leftW := ansi.StringWidth(left)
	gap := width - leftW - tagW - 1
	if gap < 1 {
		budget := width - tagW - 2
		if budget < 0 {
			budget = 0
		}
		left = ansi.Truncate(left, budget, "")
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + tag + " "
}

// paletteCommand is one entry in the `:` command hint — a canonical
// name, a short description, and optional sub-arguments that become
// the live hint set after the master types the command name and a
// space (e.g. `:theme `).
type paletteCommand struct {
	name string
	desc string
	args []paletteArg
}

// paletteArg is a single sub-argument hint shown after `<name> `.
type paletteArg struct {
	name string
	desc string
}

// sortArgs / themeArgs / markArgs populate the sub-hint lists for
// commands that take a fixed vocabulary. Derived from authoritative
// sources (vmColumns / themes) at init time, not hand-maintained.
var (
	markArgs = []paletteArg{
		{"all", "mark every visible row"},
		{"invert", "flip marks on visible rows"},
		{"none", "clear all marks"},
	}
	sortArgs  = buildSortArgs()
	themeArgs = buildThemeArgs()
)

func buildSortArgs() []paletteArg {
	out := make([]paletteArg, 0, len(vmColumns))
	for _, c := range vmColumns {
		if c.sort == 0 {
			continue
		}
		out = append(out, paletteArg{name: c.id})
	}
	return out
}

func buildThemeArgs() []paletteArg {
	out := make([]paletteArg, 0, len(themes))
	for _, n := range themeNames() {
		out = append(out, paletteArg{name: n})
	}
	return out
}

// paletteCommands lists every `:`-command dirt accepts, in the order
// the hint should display them. Only canonical names are listed; the
// aliases (vms, snapshot, quit, …) are handled by execCommand itself.
var paletteCommands = []paletteCommand{
	{"vm", "VM list", nil},
	{"snap", "snapshots", nil},
	{"net", "networks", nil},
	{"pool", "pools", nil},
	{"host", "hosts", nil},
	{"perf", "graphs", nil},
	{"jobs", "background jobs", nil},
	{"columns", "show / hide table columns", nil},
	{"config", "edit config in $EDITOR", nil},
	{"save", "persist runtime preferences to config.yaml", nil},
	{"export", "export VM list to file", []paletteArg{
		{name: "csv"},
		{name: "json"},
	}},
	{"resume", "resume paused VM(s)", nil},
	{"mark", "mark [all|invert|none]", markArgs},
	{"sort", "sort [col] [desc]", sortArgs},
	{"theme", "theme [name]", themeArgs},
	{"group", "group VM list", []paletteArg{
		{name: "os"},
		{name: "state"},
		{name: "none"},
	}},
	{"help", "help", nil},
	{"q", "quit", nil},
}

// commandPaletteHint renders the dynamic menu shown next to the `:`
// prompt. Without a space it lists matching command names; once the
// master has typed `<name> ` it switches to that command's sub-arg
// menu (theme names, sort columns, mark subcommands). Unknown-
// command or no-match branches still show something so discovery
// never dead-ends.
func commandPaletteHint(prefix string) string {
	// Sub-arg mode: `<name>` (possibly partial sub-arg) with a space.
	if idx := strings.Index(prefix, " "); idx >= 0 {
		base := prefix[:idx]
		sub := strings.TrimLeft(prefix[idx+1:], " ")
		for _, c := range paletteCommands {
			if c.name == base && len(c.args) > 0 {
				return renderArgHints(base, sub, c.args)
			}
		}
		// The command has no sub-args — fall through to the "no
		// match" styling so the master sees the full palette again.
	}
	matches := paletteCommands[:0:0]
	for _, c := range paletteCommands {
		if strings.HasPrefix(c.name, prefix) {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		parts := make([]string, 0, len(paletteCommands))
		for _, c := range paletteCommands {
			parts = append(parts, headerLabel.Render(":"+c.name))
		}
		return errorStyle.Render("(no match) ") + strings.Join(parts, headerLabel.Render("  "))
	}
	parts := make([]string, 0, len(matches))
	for _, c := range matches {
		var label string
		if prefix != "" {
			label = keyHint.Render(":"+prefix) + headerLabel.Render(c.name[len(prefix):])
		} else {
			label = keyHint.Render(":") + headerLabel.Render(c.name)
		}
		parts = append(parts, label+" "+headerLabel.Render(c.desc))
	}
	return strings.Join(parts, headerLabel.Render("  ·  "))
}

// renderArgHints lists sub-args whose name starts with sub; on no
// match the full arg menu is shown dimly so the master can still
// discover valid choices.
func renderArgHints(base, sub string, args []paletteArg) string {
	matches := args[:0:0]
	for _, a := range args {
		if strings.HasPrefix(a.name, sub) {
			matches = append(matches, a)
		}
	}
	if len(matches) == 0 {
		parts := make([]string, 0, len(args))
		for _, a := range args {
			parts = append(parts, headerLabel.Render(a.name))
		}
		return keyHint.Render(":"+base+" ") +
			errorStyle.Render("(no match) ") +
			strings.Join(parts, headerLabel.Render("  "))
	}
	parts := make([]string, 0, len(matches))
	for _, a := range matches {
		var label string
		if sub != "" {
			label = keyHint.Render(sub) + headerLabel.Render(a.name[len(sub):])
		} else {
			label = headerLabel.Render(a.name)
		}
		if a.desc != "" {
			label += " " + headerLabel.Render(a.desc)
		}
		parts = append(parts, label)
	}
	return keyHint.Render(":"+base+" ") + strings.Join(parts, headerLabel.Render("  ·  "))
}

// shortHelp is the always-on key hint line. Each description is
// wrapped in statusBar style explicitly because lipgloss closes
// inner styled spans (key()) with a `\x1b[m` reset that breaks any
// outer fg — the descriptions would otherwise revert to the
// terminal default and look grey under coloured themes.
func (m Model) shortHelp() string {
	hint := func(k, d string) string { return key(k) + statusBar.Render(d) }
	hints := []string{
		hint("j/k", " nav"),
		hint("s", " start"),
		hint("S", " stop"),
		hint("D", " kill"),
		hint("R", " reboot"),
		hint("p", " pause"),
		hint("o", " ssh"),
		hint("M", " migrate"),
		hint("C", " clone"),
		hint("A", " attach"),
		hint("X", " detach"),
		hint("c", " console"),
		hint("v", " viewer"),
		hint("e", " edit"),
		hint("Enter", " info"),
		hint("x", " xml"),
		hint("U", " undefine"),
		hint("space", " mark"),
		hint(":", " cmd"),
		hint("/", " filter"),
		hint("?", " help"),
		hint("q", " quit"),
	}
	return strings.Join(hints, statusBar.Render("  "))
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
