// Package ui implements dirt's Bubble Tea UI.
package ui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/llcoolkm/dirt/internal/lv"
)

// defaultRefreshInterval is the fall-back tick rate when none is configured.
const defaultRefreshInterval = 2 * time.Second

// swapTTL is how long a fetched SwapInfo is considered fresh enough to skip
// re-querying QGA. We refresh on tick if older than this.
const swapTTL = 5 * time.Second

// Model is the root Bubble Tea model.
type Model struct {
	client *lv.Client

	// refreshInterval controls the snapshot tick rate. Set via WithRefreshInterval.
	refreshInterval time.Duration

	snap *lv.Snapshot
	err  error

	// History keyed by domain UUID — survives across refreshes.
	history map[string]*domHistory

	// QGA-backed swap info, keyed by domain name.
	swap map[string]lv.SwapInfo

	// host info — fetched once at startup, immutable thereafter.
	host lv.HostInfo

	// Layout.
	width, height int

	// Selection (index into the filtered, sorted list).
	selected int
	offset   int // first visible row

	// Filter mode.
	filtering bool
	filter    string

	// Detail mode.
	detailMode      bool
	detailXML       string
	detailLines     []string // cached line-split of detailXML
	detailScroll    int
	detailSearch    string // active search query (empty = none)
	detailSearching bool   // currently typing into the / prompt
	detailMatches   []int  // line indices matching detailSearch
	detailMatchIdx  int    // index into detailMatches for current cursor

	// Confirm dialog (for destructive actions).
	confirming    bool
	confirmAction string // e.g. "destroy"
	confirmName   string

	// Transient flash message in the status bar.
	flash      string
	flashUntil time.Time

	// Help modal.
	showHelp bool

	// Column sort. sortColumn indexes into the list's sortable columns.
	sortColumn sortColumn
	sortDesc   bool
}

// sortColumn enumerates the sortable columns in the VM list.
type sortColumn int

const (
	sortByName sortColumn = iota
	sortByState
	sortByVCPU
	sortByMem
	sortByCPU
)

func (s sortColumn) String() string {
	switch s {
	case sortByName:
		return "name"
	case sortByState:
		return "state"
	case sortByVCPU:
		return "vCPU"
	case sortByMem:
		return "MEM"
	case sortByCPU:
		return "CPU%"
	}
	return "?"
}

// New constructs a fresh Model bound to the given libvirt client.
func New(c *lv.Client) Model {
	return Model{
		client:          c,
		refreshInterval: defaultRefreshInterval,
		history:         make(map[string]*domHistory),
		swap:            make(map[string]lv.SwapInfo),
		sortColumn:      sortByState, // running first by default
	}
}

// WithRefreshInterval returns a copy of the model with the snapshot tick rate
// set to d. Values below 200ms are clamped to 200ms to protect libvirt.
func (m Model) WithRefreshInterval(d time.Duration) Model {
	if d < 200*time.Millisecond {
		d = 200 * time.Millisecond
	}
	m.refreshInterval = d
	return m
}

// ──────────────────────────── Tea messages ───────────────────────────────────

type tickMsg time.Time

type snapshotMsg struct {
	snap *lv.Snapshot
	err  error
}

type actionResultMsg struct {
	action string
	name   string
	err    error
}

type detailLoadedMsg struct {
	name string
	xml  string
	err  error
}

type hostLoadedMsg struct {
	host lv.HostInfo
	err  error
}

type swapMsg struct {
	name string
	info lv.SwapInfo
}

// ──────────────────────────── Commands ───────────────────────────────────────

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func loadCmd(c *lv.Client) tea.Cmd {
	return func() tea.Msg {
		snap, err := c.Snapshot()
		return snapshotMsg{snap: snap, err: err}
	}
}

func actionCmd(c *lv.Client, action, name string, fn func(string) error) tea.Cmd {
	return func() tea.Msg {
		return actionResultMsg{action: action, name: name, err: fn(name)}
	}
}

func loadDetailCmd(c *lv.Client, name string) tea.Cmd {
	return func() tea.Msg {
		x, err := c.XMLDesc(name)
		return detailLoadedMsg{name: name, xml: x, err: err}
	}
}

func loadHostCmd(c *lv.Client) tea.Cmd {
	return func() tea.Msg {
		h, err := c.Host()
		return hostLoadedMsg{host: h, err: err}
	}
}

func swapCmd(c *lv.Client, name string) tea.Cmd {
	return func() tea.Msg {
		return swapMsg{name: name, info: c.Swap(name)}
	}
}

// ──────────────────────────── Init ───────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadCmd(m.client), loadHostCmd(m.client), tickCmd(m.refreshInterval))
}

// ──────────────────────────── Update ─────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		return m, tea.Batch(loadCmd(m.client), tickCmd(m.refreshInterval))

	case snapshotMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.snap = msg.snap
		m.updateHistory()
		m.boundSelection()
		return m, m.maybeFetchSwap()

	case swapMsg:
		m.swap[msg.name] = msg.info
		return m, nil

	case hostLoadedMsg:
		if msg.err == nil {
			m.host = msg.host
		}
		return m, nil

	case actionResultMsg:
		if msg.err != nil {
			m.flashf("✗ %s %s: %v", msg.action, msg.name, msg.err)
		} else {
			m.flashf("✓ %s %s", msg.action, msg.name)
		}
		// Refresh immediately after a successful action.
		return m, loadCmd(m.client)

	case detailLoadedMsg:
		if msg.err != nil {
			m.flashf("✗ load detail %s: %v", msg.name, msg.err)
			m.detailMode = false
			return m, nil
		}
		m.detailXML = msg.xml
		m.detailLines = strings.Split(msg.xml, "\n")
		m.detailScroll = 0
		m.detailSearch = ""
		m.detailMatches = nil
		m.detailMatchIdx = 0
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// handleKey routes keypresses based on current mode.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.confirming:
		return m.handleConfirmKey(msg)
	case m.filtering:
		return m.handleFilterKey(msg)
	case m.detailMode:
		return m.handleDetailKey(msg)
	default:
		return m.handleNormalKey(msg)
	}
}

func (m Model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Help modal swallows everything except its own dismiss keys.
	if m.showHelp {
		switch msg.String() {
		case "?", "esc", "q":
			m.showHelp = false
		}
		return m, nil
	}

	doms := m.visibleDomains()

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "?":
		m.showHelp = true
		return m, nil

	// Sort by column. Same key again toggles direction.
	case "1":
		m.toggleSort(sortByName)
		return m, nil
	case "2":
		m.toggleSort(sortByState)
		return m, nil
	case "3":
		m.toggleSort(sortByVCPU)
		return m, nil
	case "4":
		m.toggleSort(sortByMem)
		return m, nil
	case "5":
		m.toggleSort(sortByCPU)
		return m, nil

	case "j", "down":
		if m.selected < len(doms)-1 {
			m.selected++
		}
		return m, m.maybeFetchSwap()

	case "k", "up":
		if m.selected > 0 {
			m.selected--
		}
		return m, m.maybeFetchSwap()

	case "g", "home":
		m.selected = 0
		return m, nil

	case "G", "end":
		if len(doms) > 0 {
			m.selected = len(doms) - 1
		}
		return m, nil

	case "ctrl+d", "pgdown":
		m.selected += 10
		if m.selected >= len(doms) {
			m.selected = len(doms) - 1
		}
		return m, nil

	case "ctrl+u", "pgup":
		m.selected -= 10
		if m.selected < 0 {
			m.selected = 0
		}
		return m, nil

	case "/":
		m.filtering = true
		m.filter = ""
		return m, nil

	case "esc":
		m.filter = ""
		return m, nil

	case "enter", "d":
		if d, ok := m.currentDomain(); ok {
			m.detailMode = true
			m.detailXML = "(loading…)"
			return m, loadDetailCmd(m.client, d.Name)
		}
		return m, nil

	// ── Lifecycle actions ──
	case "s":
		if d, ok := m.currentDomain(); ok && d.State != lv.StateRunning {
			return m, actionCmd(m.client, "start", d.Name, m.client.Start)
		}
		return m, nil

	case "S":
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning {
			return m, actionCmd(m.client, "shutdown", d.Name, m.client.Shutdown)
		}
		return m, nil

	case "D":
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning {
			m.confirming = true
			m.confirmAction = "destroy"
			m.confirmName = d.Name
		}
		return m, nil

	case "r":
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning {
			return m, actionCmd(m.client, "reboot", d.Name, m.client.Reboot)
		}
		return m, nil

	case "p":
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning {
			return m, actionCmd(m.client, "pause", d.Name, m.client.Suspend)
		}
		return m, nil

	case "R":
		if d, ok := m.currentDomain(); ok && d.State == lv.StatePaused {
			return m, actionCmd(m.client, "resume", d.Name, m.client.Resume)
		}
		return m, nil

	case "c":
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning {
			return m, m.runConsole(d.Name)
		}
		return m, nil

	case "e":
		if d, ok := m.currentDomain(); ok {
			return m, m.runEdit(d.Name)
		}
		return m, nil

	case "x":
		if d, ok := m.currentDomain(); ok && d.State != lv.StateRunning {
			m.confirming = true
			m.confirmAction = "undefine"
			m.confirmName = d.Name
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filter = ""
		m.selected = 0
		return m, nil
	case "enter":
		m.filtering = false
		m.selected = 0
		return m, nil
	case "backspace":
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
		}
		return m, nil
	default:
		// Append printable chars only.
		if len(msg.String()) == 1 {
			m.filter += msg.String()
		}
		return m, nil
	}
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Search input mode takes over all printable keys until enter or esc.
	if m.detailSearching {
		return m.handleDetailSearchKey(msg)
	}

	bodyH := m.detailBodyHeight()
	maxScroll := len(m.detailLines) - bodyH
	if maxScroll < 0 {
		maxScroll = 0
	}

	switch msg.String() {
	case "esc", "q":
		// If a search is active, esc clears it first; second esc/q exits.
		if m.detailSearch != "" {
			m.detailSearch = ""
			m.detailMatches = nil
			m.detailMatchIdx = 0
			return m, nil
		}
		m.detailMode = false
		return m, nil

	case "enter":
		m.detailMode = false
		return m, nil

	case "j", "down":
		if m.detailScroll < maxScroll {
			m.detailScroll++
		}
		return m, nil

	case "k", "up":
		if m.detailScroll > 0 {
			m.detailScroll--
		}
		return m, nil

	case "pgdown", "right", "ctrl+f", " ":
		m.detailScroll += bodyH
		if m.detailScroll > maxScroll {
			m.detailScroll = maxScroll
		}
		return m, nil

	case "pgup", "left", "ctrl+b":
		m.detailScroll -= bodyH
		if m.detailScroll < 0 {
			m.detailScroll = 0
		}
		return m, nil

	case "g", "home":
		m.detailScroll = 0
		return m, nil

	case "G", "end":
		m.detailScroll = maxScroll
		return m, nil

	case "/":
		m.detailSearching = true
		m.detailSearch = ""
		m.detailMatches = nil
		m.detailMatchIdx = 0
		return m, nil

	case "n":
		m.nextDetailMatch()
		return m, nil

	case "N":
		m.prevDetailMatch()
		return m, nil
	}
	return m, nil
}

// handleDetailSearchKey handles keypresses while typing into the / prompt.
// Search runs incrementally — every keystroke re-runs applyDetailSearch so the
// viewport jumps to the first match and highlights update as the master types.
func (m Model) handleDetailSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.detailSearching = false
		m.detailSearch = ""
		m.detailMatches = nil
		m.detailMatchIdx = 0
		return m, nil

	case "enter":
		// Lock in the current search and leave the prompt; matches stay.
		m.detailSearching = false
		return m, nil

	case "backspace":
		if len(m.detailSearch) > 0 {
			m.detailSearch = m.detailSearch[:len(m.detailSearch)-1]
			m.applyDetailSearch(m.detailSearch)
		}
		return m, nil

	default:
		if len(msg.String()) == 1 {
			m.detailSearch += msg.String()
			m.applyDetailSearch(m.detailSearch)
		}
		return m, nil
	}
}

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		name := m.confirmName
		action := m.confirmAction
		m.confirming = false
		m.confirmName = ""
		m.confirmAction = ""
		switch action {
		case "destroy":
			return m, actionCmd(m.client, "destroy", name, m.client.Destroy)
		case "undefine":
			return m, actionCmd(m.client, "undefine", name, m.client.Undefine)
		}
		return m, nil
	default:
		m.confirming = false
		m.confirmName = ""
		m.confirmAction = ""
		m.flash = "cancelled"
		m.flashUntil = time.Now().Add(2 * time.Second)
		return m, nil
	}
}

// ──────────────────────────── Helpers ────────────────────────────────────────

// updateHistory rolls each domain's stats forward by one sample, garbage-collects
// histories for domains that no longer exist, and resets history for stopped VMs.
func (m *Model) updateHistory() {
	if m.snap == nil {
		return
	}
	seen := make(map[string]bool, len(m.snap.Domains))
	for _, d := range m.snap.Domains {
		seen[d.UUID] = true
		h, ok := m.history[d.UUID]
		if !ok {
			h = &domHistory{}
			m.history[d.UUID] = h
		}
		if d.State != lv.StateRunning {
			h.reset()
			continue
		}
		h.update(d)
	}
	for uuid := range m.history {
		if !seen[uuid] {
			delete(m.history, uuid)
		}
	}
}

// visibleDomains returns the filtered + sorted slice currently shown in the table.
func (m Model) visibleDomains() []lv.Domain {
	if m.snap == nil {
		return nil
	}
	out := make([]lv.Domain, 0, len(m.snap.Domains))
	f := strings.ToLower(strings.TrimSpace(m.filter))
	for _, d := range m.snap.Domains {
		if f != "" && !strings.Contains(strings.ToLower(d.Name), f) {
			continue
		}
		out = append(out, d)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return m.lessDomain(out[i], out[j])
	})
	return out
}

// lessDomain implements the active column sort order.
func (m Model) lessDomain(a, b lv.Domain) bool {
	flip := func(v bool) bool {
		if m.sortDesc {
			return !v
		}
		return v
	}
	switch m.sortColumn {
	case sortByName:
		return flip(a.Name < b.Name)
	case sortByState:
		// State sort: running first, then by enum order, then by name.
		if a.State != b.State {
			if a.State == lv.StateRunning {
				return flip(true)
			}
			if b.State == lv.StateRunning {
				return flip(false)
			}
			return flip(a.State < b.State)
		}
		return a.Name < b.Name
	case sortByVCPU:
		if a.NrVCPU != b.NrVCPU {
			return flip(a.NrVCPU > b.NrVCPU)
		}
		return a.Name < b.Name
	case sortByMem:
		if a.MaxMemKB != b.MaxMemKB {
			return flip(a.MaxMemKB > b.MaxMemKB)
		}
		return a.Name < b.Name
	case sortByCPU:
		ha := m.history[a.UUID]
		hb := m.history[b.UUID]
		va, vb := 0.0, 0.0
		if ha != nil {
			va = ha.currentCPU()
		}
		if hb != nil {
			vb = hb.currentCPU()
		}
		if va != vb {
			return flip(va > vb)
		}
		return a.Name < b.Name
	}
	return a.Name < b.Name
}

// toggleSort switches to the given column, or flips direction if already on it.
func (m *Model) toggleSort(col sortColumn) {
	if m.sortColumn == col {
		m.sortDesc = !m.sortDesc
	} else {
		m.sortColumn = col
		m.sortDesc = false
	}
}

// currentDomain returns the currently selected domain, if any.
func (m Model) currentDomain() (lv.Domain, bool) {
	doms := m.visibleDomains()
	if len(doms) == 0 || m.selected < 0 || m.selected >= len(doms) {
		return lv.Domain{}, false
	}
	return doms[m.selected], true
}

func (m *Model) boundSelection() {
	doms := m.visibleDomains()
	if len(doms) == 0 {
		m.selected = 0
		return
	}
	if m.selected >= len(doms) {
		m.selected = len(doms) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

func (m *Model) flashf(format string, args ...any) {
	m.flash = fmt.Sprintf(format, args...)
	m.flashUntil = time.Now().Add(3 * time.Second)
}

// detailBodyHeight returns the number of XML lines visible in the detail pane.
// Mirrors the layout used by view.detailView so paging math stays consistent.
func (m Model) detailBodyHeight() int {
	h := m.height
	if h == 0 {
		h = 24
	}
	// title row + blank + position row + bottom prompt + 2 border + 2 padding
	bodyH := h - 7
	if bodyH < 5 {
		bodyH = 5
	}
	return bodyH
}

// applyDetailSearch finds every line containing the (case-insensitive) query
// and centres the viewport on the first match.
func (m *Model) applyDetailSearch(query string) {
	m.detailSearch = query
	m.detailMatches = nil
	m.detailMatchIdx = 0
	if query == "" {
		return
	}
	q := strings.ToLower(query)
	for i, line := range m.detailLines {
		if strings.Contains(strings.ToLower(line), q) {
			m.detailMatches = append(m.detailMatches, i)
		}
	}
	if len(m.detailMatches) > 0 {
		m.scrollToDetailMatch()
	}
}

func (m *Model) nextDetailMatch() {
	if len(m.detailMatches) == 0 {
		return
	}
	m.detailMatchIdx = (m.detailMatchIdx + 1) % len(m.detailMatches)
	m.scrollToDetailMatch()
}

func (m *Model) prevDetailMatch() {
	if len(m.detailMatches) == 0 {
		return
	}
	m.detailMatchIdx--
	if m.detailMatchIdx < 0 {
		m.detailMatchIdx = len(m.detailMatches) - 1
	}
	m.scrollToDetailMatch()
}

// scrollToDetailMatch centres the current search match in the viewport.
func (m *Model) scrollToDetailMatch() {
	if len(m.detailMatches) == 0 {
		return
	}
	line := m.detailMatches[m.detailMatchIdx]
	bodyH := m.detailBodyHeight()
	target := line - bodyH/2
	maxScroll := len(m.detailLines) - bodyH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if target < 0 {
		target = 0
	}
	if target > maxScroll {
		target = maxScroll
	}
	m.detailScroll = target
}

// maybeFetchSwap returns a Cmd that queries QGA for the highlighted VM's swap
// usage, but only if the cached value is stale or missing. Returns nil if the
// VM isn't running or has no name. The Cmd runs asynchronously.
func (m Model) maybeFetchSwap() tea.Cmd {
	d, ok := m.currentDomain()
	if !ok || d.State != lv.StateRunning {
		return nil
	}
	cached, has := m.swap[d.Name]
	if has && time.Since(cached.FetchedAt) < swapTTL {
		return nil
	}
	return swapCmd(m.client, d.Name)
}

// runConsole suspends the Bubble Tea program, execs `virsh console <name>`,
// then resumes the TUI when the user detaches with Ctrl+].
func (m Model) runConsole(name string) tea.Cmd {
	return tea.ExecProcess(exec.Command("virsh", "-c", m.client.URI(), "console", name), func(err error) tea.Msg {
		if err != nil {
			return actionResultMsg{action: "console", name: name, err: err}
		}
		// Reset terminal stdin echo just in case virsh left it weird.
		_ = os.Stdin.Sync()
		return actionResultMsg{action: "console", name: name}
	})
}

// runEdit suspends the Bubble Tea program and execs `virsh edit <name>`,
// which opens $EDITOR on the live XML. Resumes when the editor exits.
func (m Model) runEdit(name string) tea.Cmd {
	return tea.ExecProcess(exec.Command("virsh", "-c", m.client.URI(), "edit", name), func(err error) tea.Msg {
		if err != nil {
			return actionResultMsg{action: "edit", name: name, err: err}
		}
		return actionResultMsg{action: "edit", name: name}
	})
}
