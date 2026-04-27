package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/llcoolkm/dirt/internal/config"
	"github.com/llcoolkm/dirt/internal/lv"
)

// Hosts-view column widths.
const (
	hostsNameW    = 18
	hostsURIW     = 56
	hostsStatusW  = 13
	hostsDomainsW = 8
)

// probeState is the async state of a host probe. A probe fires a
// NewConnect→GetHostname→Close triple against the host's URI off the
// UI thread; the result arrives later as a hostProbedMsg.
type probeState int

const (
	probePending probeState = iota // probe has been issued, no answer yet
	probeOK                        // probe succeeded (hostname + domain count known)
	probeFailed                    // probe returned an error
)

// hostProbeStatus is the live result of probing a single host.
type hostProbeStatus struct {
	state    probeState
	hostname string
	domains  int
	err      error
}

// ──────────────────────────── Messages ──────────────────────────────────────

// hostsLoadedMsg carries the result of reading the hosts file.
type hostsLoadedMsg struct {
	list []config.Host
	err  error
}

// hostProbedMsg carries the result of probing a single host URI.
type hostProbedMsg struct {
	name   string
	status hostProbeStatus
}

// connectedMsg signals that an async host switch succeeded. The caller
// must replace m.client with the new one and close the previous.
type connectedMsg struct {
	client *lv.Client
	nick   string
	uri    string
}

// connectErrMsg signals that an async host switch failed. The existing
// m.client is untouched.
type connectErrMsg struct {
	uri string
	nick string
	err error
}

// ──────────────────────────── Commands ──────────────────────────────────────

// loadHostsListCmd reads (or seeds) the hosts file and returns the list.
// Named "loadHostsList" to avoid collision with loadHostCmd which loads
// the current hypervisor's HostInfo.
func loadHostsListCmd(initialURI string) tea.Cmd {
	return func() tea.Msg {
		list, err := config.SeedIfMissing(initialURI)
		return hostsLoadedMsg{list: list, err: err}
	}
}

// probeHostCmd opens a short-lived libvirt connection to the given host
// in a goroutine, asks for the hostname and running domain count, and
// sends a hostProbedMsg back. Bounded to ~3 seconds.
//
// Note: cgo-backed lv.New() cannot be interrupted — the goroutine will
// keep waiting until libvirt itself gives up. If the connection arrives
// after the timeout, we close the late client in a background goroutine
// so file descriptors and libvirt handles do not leak while dead
// qemu+ssh endpoints are repeatedly probed.
func probeHostCmd(h config.Host) tea.Cmd {
	return func() tea.Msg {
		type result struct {
			client *lv.Client
			status hostProbeStatus
		}
		done := make(chan result, 1)
		go func() {
			c, err := lv.New(h.URI)
			if err != nil {
				done <- result{status: hostProbeStatus{state: probeFailed, err: err}}
				return
			}
			status := hostProbeStatus{state: probeOK, hostname: c.Hostname()}
			if snap, err := c.Snapshot(); err == nil {
				for _, d := range snap.Domains {
					if d.State == lv.StateRunning {
						status.domains++
					}
				}
			}
			done <- result{client: c, status: status}
		}()
		select {
		case r := <-done:
			if r.client != nil {
				r.client.Close()
			}
			return hostProbedMsg{name: h.Name, status: r.status}
		case <-time.After(3 * time.Second):
			// Don't leak if the connection eventually succeeds.
			go func() {
				if r := <-done; r.client != nil {
					r.client.Close()
				}
			}()
			return hostProbedMsg{name: h.Name, status: hostProbeStatus{
				state: probeFailed,
				err:   fmt.Errorf("timeout after 3s"),
			}}
		}
	}
}

// editorCommand builds an *exec.Cmd from the user's $EDITOR setting,
// opening `path`. $EDITOR may include flags — `nvim -f`, `code --wait`,
// `emacs -nw` — which are split on whitespace and passed as separate
// argv entries. If $EDITOR is unset, falls back to `vi`.
func editorCommand(path string) *exec.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	fields := strings.Fields(editor)
	if len(fields) == 0 {
		fields = []string{"vi"}
	}
	args := append(fields[1:], path)
	return exec.Command(fields[0], args...)
}

// editHostsFileCmd suspends the TUI and execs $EDITOR on the hosts
// file. On return, it reloads the file so any edits made in-place take
// effect.
func editHostsFileCmd() tea.Cmd {
	return tea.ExecProcess(editorCommand(config.HostsPath()), func(err error) tea.Msg {
		// Whether the editor succeeded or not, reload so the view
		// reflects what's now on disk. A load error surfaces via the
		// normal hostsLoadedMsg error field.
		list, loadErr := config.LoadHosts()
		return hostsLoadedMsg{list: list, err: loadErr}
	})
}

// probeAllHostsCmd fires a probe for every host in the list. Each probe
// runs as its own tea.Cmd so results stream in as they arrive, filling
// the table progressively.
func probeAllHostsCmd(hosts []config.Host) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(hosts))
	for _, h := range hosts {
		cmds = append(cmds, probeHostCmd(h))
	}
	return tea.Batch(cmds...)
}

// connectCmd opens a new libvirt connection to uri off the UI thread,
// with a 5-second timeout. If the connection succeeds after the timeout
// fires, the late client is closed in a background goroutine so no file
// descriptors leak. On success, returns a connectedMsg; on failure or
// timeout, a connectErrMsg.
func connectCmd(uri, nick string) tea.Cmd {
	return func() tea.Msg {
		type result struct {
			c   *lv.Client
			err error
		}
		done := make(chan result, 1)
		go func() {
			c, err := lv.New(uri)
			done <- result{c: c, err: err}
		}()
		select {
		case r := <-done:
			if r.err != nil {
				return connectErrMsg{uri: uri, nick: nick, err: r.err}
			}
			return connectedMsg{client: r.c, nick: nick, uri: uri}
		case <-time.After(5 * time.Second):
			// Don't leak if the connection eventually succeeds.
			go func() {
				if r := <-done; r.c != nil {
					r.c.Close()
				}
			}()
			return connectErrMsg{uri: uri, nick: nick, err: fmt.Errorf("connect timeout after 5s")}
		}
	}
}

// ──────────────────────────── Render ────────────────────────────────────────

// hostsView renders the host list.
func (m Model) hostsView() string {
	width := m.contentWidth()

	title := headerTitle.Render("hosts") +
		headerLabel.Render("   ") +
		headerLabel.Render("config: ") + headerValue.Render(config.HostsPath())

	active := m.hostsSortIdx
	desc := m.hostsSortDesc
	header := listHeaderRow.Render(" " + strings.Join([]string{
		padRight(arrowedHeader("NAME", active == hostColName, desc), hostsNameW),
		padRight(arrowedHeader("URI", active == hostColURI, desc), hostsURIW),
		padRight(arrowedHeader("STATUS", active == hostColStatus, desc), hostsStatusW),
		padLeft(arrowedHeader("DOMAINS", active == hostColDomains, desc), hostsDomainsW),
	}, "  "))

	rows := []string{header}
	if m.hostsErr != nil {
		rows = append(rows, "", errorStyle.Render("  error: "+m.hostsErr.Error()))
	} else if len(m.hosts) == 0 {
		rows = append(rows, "", lipgloss.NewStyle().Foreground(colDimmed).Italic(true).Render("  no hosts — press a to add one, or e to edit the hosts file"))
	} else {
		sorted := m.sortedHosts()
		for i, h := range sorted {
			rows = append(rows, renderHostRow(h, m.hostsProbe[h.Name], m.client.URI(), i == m.hostsSel))
		}
	}

	pane := listBox.Width(width - borderWidth).Render(lipgloss.JoinVertical(lipgloss.Left,
		append([]string{title, ""}, rows...)...))

	bottom := hostsStatusBar(m, width)
	return lipgloss.JoinVertical(lipgloss.Left, pane, bottom)
}

// renderHostRow is one row of the host table. The "current" host (the
// one dirt is actively connected to) is rendered in green to mirror the
// running state indicator elsewhere.
func renderHostRow(h config.Host, p hostProbeStatus, currentURI string, selected bool) string {
	statusStr, statusStyle := probeDisplay(p, h.URI == currentURI)
	domainsStr := "—"
	if p.state == probeOK {
		domainsStr = fmt.Sprintf("%d", p.domains)
	}

	if selected {
		row := strings.Join([]string{
			padRight(truncate(h.Name, hostsNameW), hostsNameW),
			padRight(truncate(h.URI, hostsURIW), hostsURIW),
			padRight(statusStr, hostsStatusW),
			padLeft(domainsStr, hostsDomainsW),
		}, "  ")
		return rowSelected.Render(" " + row)
	}

	fg := lipgloss.NewStyle().Foreground(colFG)
	cols := []string{
		fg.Render(padRight(truncate(h.Name, hostsNameW), hostsNameW)),
		fg.Render(padRight(truncate(h.URI, hostsURIW), hostsURIW)),
		statusStyle.Render(padRight(statusStr, hostsStatusW)),
		fg.Render(padLeft(domainsStr, hostsDomainsW)),
	}
	return " " + strings.Join(cols, "  ")
}

// probeDisplay returns the short status label and its lipgloss style for
// a given probe result. Marks the currently-connected host as "current".
func probeDisplay(p hostProbeStatus, isCurrent bool) (string, lipgloss.Style) {
	if isCurrent {
		return "• connected", stateRunning
	}
	switch p.state {
	case probeOK:
		return "reachable", stateRunning
	case probeFailed:
		return "unreachable", stateCrashed
	default:
		return "probing…", stateShutoff
	}
}

// hostsStatusBar renders the bottom line for the hosts view — text
// input prompts take priority, then confirm, then flash, then hints.
func hostsStatusBar(m Model, width int) string {
	cursor := lipgloss.NewStyle().Foreground(colAccent).Render("█")
	if m.hostInputStage == 1 {
		prompt := keyHint.Render("name: ") + m.hostInputName + cursor +
			headerLabel.Render("   (enter to continue, esc to cancel)")
		return statusBar.Width(width).Render(" " + prompt)
	}
	if m.hostInputStage == 2 {
		prompt := keyHint.Render("uri:  ") + m.hostInputURI + cursor +
			headerLabel.Render("   (enter to save, esc to cancel)")
		return statusBar.Width(width).Render(" " + prompt)
	}
	if m.confirming {
		msg := errorStyle.Render(fmt.Sprintf(" ⚠ remove host “%s”? ", m.confirmName)) +
			keyHint.Render("y") + statusBar.Render(" to confirm, any other key to cancel")
		return statusBar.Width(width).Render(msg)
	}
	if m.flash != "" && time.Now().Before(m.flashUntil) {
		return statusBar.Width(width).Render(" " + flashStyle.Render(m.flash))
	}
	// Compact the status bar for narrow terminals — drop the verbose
	// action names first, then the less-essential keys, before it wraps.
	full := " " + key("j/k") + " nav  " + key("Enter") + " connect  " +
		key("a") + " add  " + key("e") + " edit file  " +
		key("R") + " re-probe  " + key("D") + " remove  " +
		key("esc") + " back"
	medium := " " + key("j/k") + " nav  " + key("Enter") + " connect  " +
		key("a") + " add  " + key("R") + " re-probe  " + key("esc") + " back"
	short := " " + key("Enter") + " connect  " + key("a") + " add  " + key("esc") + " back"
	hint := full
	if width < 80 {
		hint = short
	} else if width < 110 {
		hint = medium
	}
	return statusBar.Width(width).Render(hint)
}

// ──────────────────────────── Key handling ──────────────────────────────────

// handleHostsKey handles keys while in the hosts view.
func (m Model) handleHostsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.hostInputStage > 0 {
		return m.handleHostInputKey(msg)
	}
	if m.confirming {
		return m.handleHostsConfirmKey(msg)
	}
	switch msg.String() {
	case "?":
		m.prevMode = m.mode
		m.mode = viewHelp
		return m, nil
	case "esc", "q":
		m.mode = viewMain
		return m, nil
	}
	if navSelect(msg.String(), &m.hostsSel, len(m.hosts)) {
		return m, nil
	}
	switch msg.String() {
	case "enter":
		if h, ok := m.currentHostEntry(); ok {
			return m.connectToHost(h.URI, h.Name)
		}
		return m, nil
	case "R", "F5":
		return m, probeAllHostsCmd(m.hosts)
	case "a":
		// Begin the two-step add-host input flow.
		m.hostInputStage = 1
		m.hostInputName = ""
		m.hostInputURI = ""
		return m, nil
	case "e":
		// Open the hosts file in $EDITOR. Reloads on return.
		return m, editHostsFileCmd()
	case "D", "x":
		if h, ok := m.currentHostEntry(); ok {
			m.confirming = true
			m.confirmAction = "remove-host"
			m.confirmName = h.Name
		}
		return m, nil
	}
	return m, nil
}

// handleHostInputKey drives the two-step add-host text input. Stage 1
// collects the nickname; stage 2 collects the URI. Enter advances /
// saves; Esc cancels at any stage.
func (m Model) handleHostInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.hostInputStage = 0
		m.hostInputName = ""
		m.hostInputURI = ""
		return m, nil
	case "enter":
		switch m.hostInputStage {
		case 1:
			name := strings.TrimSpace(m.hostInputName)
			if name == "" {
				m.flashf("✗ name cannot be empty")
				m.hostInputStage = 0
				return m, nil
			}
			for _, h := range m.hosts {
				if h.Name == name {
					m.flashf("✗ host %q already exists", name)
					m.hostInputStage = 0
					m.hostInputName = ""
					return m, nil
				}
			}
			m.hostInputName = name
			m.hostInputStage = 2
			return m, nil
		case 2:
			uri := strings.TrimSpace(m.hostInputURI)
			if uri == "" {
				m.flashf("✗ uri cannot be empty")
				m.hostInputStage = 0
				return m, nil
			}
			newHost := config.Host{Name: m.hostInputName, URI: uri}
			m.hosts = append(m.hosts, newHost)
			if err := config.SaveHosts(m.hosts); err != nil {
				m.flashf("✗ save hosts: %v", err)
				m.hostInputStage = 0
				return m, nil
			}
			m.flashf("✓ added host %s", newHost.Name)
			m.hostInputStage = 0
			m.hostInputName = ""
			m.hostInputURI = ""
			return m, probeHostCmd(newHost)
		}
		return m, nil
	case "backspace":
		switch m.hostInputStage {
		case 1:
			m.hostInputName = runeBackspace(m.hostInputName)
		case 2:
			m.hostInputURI = runeBackspace(m.hostInputURI)
		}
		return m, nil
	default:
		s := msg.String()
		if len(s) == 1 {
			switch m.hostInputStage {
			case 1:
				m.hostInputName += s
			case 2:
				m.hostInputURI += s
			}
		}
		return m, nil
	}
}

// handleHostsConfirmKey handles y/n confirmation for host removal.
func (m Model) handleHostsConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	action := m.confirmAction
	name := m.confirmName
	m.confirming = false
	m.confirmAction = ""
	m.confirmName = ""

	if msg.String() != "y" || action != "remove-host" {
		return m, nil
	}

	// Filter out the named host and save.
	kept := make([]config.Host, 0, len(m.hosts))
	for _, h := range m.hosts {
		if h.Name != name {
			kept = append(kept, h)
		}
	}
	if err := config.SaveHosts(kept); err != nil {
		m.flashf("✗ save hosts: %v", err)
		return m, nil
	}
	m.hosts = kept
	if m.hostsSel >= len(kept) {
		m.hostsSel = len(kept) - 1
	}
	if m.hostsSel < 0 {
		m.hostsSel = 0
	}
	m.flashf("✓ removed host %s", name)
	return m, nil
}

// currentHostEntry returns the host at the current selection, if any.
func (m Model) currentHostEntry() (config.Host, bool) {
	if m.hostsSel < 0 || m.hostsSel >= len(m.hosts) {
		return config.Host{}, false
	}
	return m.hosts[m.hostsSel], true
}

// connectToHost starts an async connect to uri. If the current client
// is already on that URI, it flashes a notice and does nothing. The new
// connection replaces the current one when connectedMsg arrives.
func (m Model) connectToHost(uri, nick string) (Model, tea.Cmd) {
	if m.client != nil && m.client.URI() == uri {
		m.flashf("already connected to %s", nick)
		return m, nil
	}
	m.flashf("connecting to %s…", nick)
	return m, connectCmd(uri, nick)
}

// applyConnected replaces m.client with the newly-opened client and
// resets all per-host state so the next tick rebuilds the table, header,
// and history from the new hypervisor.
func (m Model) applyConnected(msg connectedMsg) (Model, tea.Cmd) {
	if old := m.client; old != nil {
		old.Close()
	}
	m.client = msg.client

	// Per-host state: everything derived from the old connection is now
	// stale and must be cleared. User-level preferences (sort column,
	// refresh interval) are kept.
	m.snap = nil
	m.err = nil
	m.history = make(map[string]*domHistory)
	m.swap = make(map[string]lv.SwapInfo)
	m.guestUptime = make(map[string]lv.GuestUptime)
	m.host = lv.HostInfo{}
	m.hostStats = lv.HostStats{}
	m.hostCPUPct = 0
	m.hostCPUHist = nil
	m.hostHasStats = false
	m.selected = 0
	m.offset = 0
	m.filter = ""
	m.filtering = false

	m.flashf("✓ connected to %s", msg.nick)

	// Kick off a fresh load cycle on the new client. If we're still in
	// the hosts view, re-probe so the table's "connected" marker moves.
	cmds := []tea.Cmd{loadCmd(m.client), loadHostCmd(m.client), loadHostStatsCmd(m.client)}
	if m.mode == viewHosts {
		cmds = append(cmds, probeAllHostsCmd(m.hosts))
	} else {
		m.mode = viewMain
	}
	return m, tea.Batch(cmds...)
}
