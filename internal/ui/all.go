package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/llcoolkm/dirt/internal/backend"
	"github.com/llcoolkm/dirt/internal/config"
	"github.com/llcoolkm/dirt/internal/lv"
)

// Aggregated-view column widths. Independent from the main list so
// changes here don't disturb :vm.
const (
	allHostW  = 14
	allNameW  = 22
	allStateW = 9
	allIPW    = 16
	allOSW    = 18
	allVCPUW  = 4
	allMemW   = 9
)

// allRow is a VM bundled with the nick of the host it lives on.
// The nick is the display key; backends are indexed by it.
type allRow struct {
	host string
	dom  lv.Domain
}

// ────────────────────────── Messages ──────────────────────────

// allBackendOpenedMsg arrives once per host as the connection
// completes (or fails). On success the client is stored in
// m.allBackends; on failure the err is stored in m.allErrs.
type allBackendOpenedMsg struct {
	nick   string
	uri    string
	client backend.Backend
	err    error
}

// allSnapshotMsg is one host's snapshot result, identified by nick.
type allSnapshotMsg struct {
	nick string
	uri  string
	snap *lv.Snapshot
	err  error
}

// ────────────────────────── Commands ──────────────────────────

// allOpenBackendsCmd fans out connection attempts for every host in
// the list that we don't already have a backend for. Each host's
// result arrives independently as an allBackendOpenedMsg so the UI
// fills in progressively.
func allOpenBackendsCmd(hosts []config.Host, have map[string]backend.Backend) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(hosts))
	for _, h := range hosts {
		if _, ok := have[h.Name]; ok {
			continue
		}
		h := h
		cmds = append(cmds, func() tea.Msg {
			c, err := lv.New(h.URI)
			return allBackendOpenedMsg{nick: h.Name, uri: h.URI, client: c, err: err}
		})
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// allRefreshCmd fans out a Snapshot() against every open backend in
// parallel. Each per-host result arrives as an allSnapshotMsg.
func allRefreshCmd(backends map[string]backend.Backend) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(backends))
	for nick, c := range backends {
		nick := nick
		c := c
		cmds = append(cmds, func() tea.Msg {
			snap, err := c.Snapshot()
			return allSnapshotMsg{nick: nick, uri: c.URI(), snap: snap, err: err}
		})
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// ────────────────────────── Aggregation ──────────────────────────

// allRows assembles the aggregated rows in stable order (host nick
// alphabetical, then VM name) and applies the active filter.
func (m Model) allRows() []allRow {
	nicks := make([]string, 0, len(m.allSnapshots))
	for nick := range m.allSnapshots {
		nicks = append(nicks, nick)
	}
	sort.Strings(nicks)

	rows := make([]allRow, 0, 64)
	for _, nick := range nicks {
		snap := m.allSnapshots[nick]
		if snap == nil {
			continue
		}
		domains := append([]lv.Domain(nil), snap.Domains...)
		sort.SliceStable(domains, func(i, j int) bool {
			return domains[i].Name < domains[j].Name
		})
		for _, d := range domains {
			if !matchesAllFilter(d, nick, m.filter) {
				continue
			}
			rows = append(rows, allRow{host: nick, dom: d})
		}
	}
	return rows
}

func matchesAllFilter(d lv.Domain, host, filter string) bool {
	f := strings.TrimSpace(filter)
	if f == "" {
		return true
	}
	hay := strings.ToLower(d.Name + " " + d.IP + " " + d.OS + " " + host)
	return strings.Contains(hay, strings.ToLower(f))
}

// ────────────────────────── Render ──────────────────────────

func (m Model) allView() string {
	width := m.contentWidth()

	// Title with quick connection summary.
	connected, failed := 0, 0
	for nick := range m.allBackends {
		if m.allErrs[nick] != nil {
			failed++
		} else {
			connected++
		}
	}
	for nick := range m.allErrs {
		if _, ok := m.allBackends[nick]; !ok {
			failed++
		}
	}
	title := headerTitle.Render("all hosts") +
		headerLabel.Render(fmt.Sprintf("   %d up · %d down", connected, failed))

	header := listHeaderRow.Render(" " + strings.Join([]string{
		padRight("HOST", allHostW),
		padRight("NAME", allNameW),
		padRight("STATE", allStateW),
		padRight("IP", allIPW),
		padRight("OS", allOSW),
		padLeft("vCPU", allVCPUW),
		padLeft("MEM", allMemW),
	}, "  "))

	rows := []string{header}

	if m.filtering || m.filter != "" {
		rows = append(rows, " "+headerLabel.Render(fmt.Sprintf("filter: %q", m.filter)))
	}

	all := m.allRows()
	if len(all) == 0 {
		dim := lipgloss.NewStyle().Foreground(colDimmed).Italic(true)
		if len(m.allBackends) == 0 && len(m.allErrs) == 0 {
			rows = append(rows, "", dim.Render("  connecting…"))
		} else if m.filter != "" {
			rows = append(rows, "", dim.Render("  no VMs match filter"))
		} else {
			rows = append(rows, "", dim.Render("  no VMs"))
		}
	} else {
		for i, r := range all {
			rows = append(rows, renderAllRow(r, i == m.allSel))
		}
	}

	// Per-host error footer — quietly note unreachable hosts so the
	// sysadmin sees at a glance which connections failed.
	if failed > 0 {
		nicks := make([]string, 0, len(m.allErrs))
		for nick, err := range m.allErrs {
			if err != nil {
				nicks = append(nicks, nick)
			}
		}
		sort.Strings(nicks)
		rows = append(rows, "")
		for _, nick := range nicks {
			rows = append(rows, "  "+errorStyle.Render(fmt.Sprintf("✗ %s: %s", nick, shortErr(m.allErrs[nick]))))
		}
	}

	pane := listBox.Width(width - borderWidth).Render(lipgloss.JoinVertical(lipgloss.Left,
		append([]string{title, ""}, rows...)...))

	bottom := allStatusBar(m, width)
	return lipgloss.JoinVertical(lipgloss.Left, pane, bottom)
}

func renderAllRow(r allRow, selected bool) string {
	d := r.dom

	state := d.State.String()
	stateStyle := stateShutoff
	switch d.State {
	case lv.StateRunning:
		stateStyle = stateRunning
	case lv.StatePaused:
		stateStyle = statePaused
	case lv.StateCrashed:
		stateStyle = stateCrashed
	}

	mem := "—"
	if d.MaxMemKB > 0 {
		mem = formatBytes(float64(d.MemoryKB) * 1024)
	}

	cells := []string{
		padRight(truncate(r.host, allHostW), allHostW),
		padRight(truncate(d.Name, allNameW), allNameW),
		padRight(state, allStateW),
		padRight(truncate(d.IP, allIPW), allIPW),
		padRight(truncate(d.OS, allOSW), allOSW),
		padLeft(fmt.Sprintf("%d", d.NrVCPU), allVCPUW),
		padLeft(mem, allMemW),
	}
	if selected {
		return rowSelected.Render(" " + strings.Join(cells, "  "))
	}
	fg := lipgloss.NewStyle().Foreground(colFG)
	rendered := []string{
		fg.Render(cells[0]),
		fg.Render(cells[1]),
		stateStyle.Render(padRight(state, allStateW)),
		fg.Render(cells[3]),
		fg.Render(cells[4]),
		fg.Render(cells[5]),
		fg.Render(cells[6]),
	}
	return " " + strings.Join(rendered, "  ")
}

func allStatusBar(m Model, width int) string {
	if m.filtering {
		return statusBar.Width(width).Render(" " + headerLabel.Render("filter: ") + m.filter +
			lipgloss.NewStyle().Foreground(colAccent).Render("█") +
			headerLabel.Render("   (enter to apply, esc to cancel)"))
	}
	return statusBar.Width(width).Render(" " +
		key("j/k") + " nav  " +
		key("/") + " filter  " +
		key("R") + " refresh  " +
		key(":") + " palette  " +
		key("esc") + " back")
}

// ────────────────────────── Mode entry / keys ──────────────────────────

// enterAllView switches to the aggregated view and kicks off any
// missing host connections plus an immediate refresh of those
// already open.
func (m Model) enterAllView() (Model, tea.Cmd) {
	m.mode = viewAll
	m.allSel = 0
	if m.allBackends == nil {
		m.allBackends = make(map[string]backend.Backend)
	}
	if m.allSnapshots == nil {
		m.allSnapshots = make(map[string]*lv.Snapshot)
	}
	if m.allErrs == nil {
		m.allErrs = make(map[string]error)
	}

	// Hosts must be loaded already (Init seeds the file). If empty,
	// fall back to the current m.client as the only known endpoint.
	hosts := m.hosts
	if len(hosts) == 0 && m.client != nil {
		hosts = []config.Host{{Name: m.client.Hostname(), URI: m.client.URI()}}
	}

	cmds := []tea.Cmd{
		allOpenBackendsCmd(hosts, m.allBackends),
	}
	if len(m.allBackends) > 0 {
		cmds = append(cmds, allRefreshCmd(m.allBackends))
	}
	return m, tea.Batch(cmds...)
}

// CloseAllBackends releases every backend opened for the aggregated
// view. Call from main.go's shutdown hook after tea.Program.Run
// returns so SSH-backed connections don't linger.
func (m Model) CloseAllBackends() {
	for _, c := range m.allBackends {
		if c != nil {
			c.Close()
		}
	}
}

// handleAllKey handles keypresses while in the aggregated view.
func (m Model) handleAllKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.commanding {
		return m.handleCommandKey(msg)
	}
	if m.filtering {
		return m.handleFilterKey(msg)
	}
	switch msg.String() {
	case "?":
		m.prevMode = m.mode
		m.mode = viewHelp
		return m, nil
	case "esc", "q":
		m.mode = viewMain
		return m, nil
	case "/":
		m.filtering = true
		m.filter = ""
		return m, nil
	case ":":
		m.commanding = true
		m.command = ""
		return m, nil
	case "R", "F5":
		// Re-attempt failed hosts and refresh any open ones.
		for nick, err := range m.allErrs {
			if err != nil {
				delete(m.allErrs, nick)
			}
		}
		cmds := []tea.Cmd{
			allOpenBackendsCmd(m.hosts, m.allBackends),
			allRefreshCmd(m.allBackends),
		}
		return m, tea.Batch(cmds...)
	}
	rows := m.allRows()
	if navSelect(msg.String(), &m.allSel, len(rows)) {
		return m, nil
	}
	return m, nil
}

func shortErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 80 {
		s = s[:77] + "…"
	}
	return s
}
