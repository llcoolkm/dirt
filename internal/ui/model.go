// Package ui implements dirt's Bubble Tea UI.
package ui

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/llcoolkm/dirt/internal/config"
	"github.com/llcoolkm/dirt/internal/lv"
)

// defaultRefreshInterval is the fall-back tick rate when none is configured.
const defaultRefreshInterval = 1 * time.Second

// viewMode is the high-level UI state — which screen the user is currently on.
type viewMode int

const (
	viewMain      viewMode = iota // VM list (default)
	viewInfo                      // structured per-VM info pane
	viewDetail                    // raw XML detail of selected VM
	viewGraphs                    // performance graphs for selected VM
	viewHelp                      // help modal
	viewSnapshots                 // snapshots of selected VM
	viewNetworks                  // libvirt networks
	viewPools                     // storage pools
	viewVolumes                   // volumes inside selected pool
	viewLeases                    // DHCP leases of selected network
	viewHosts                     // list of known libvirt endpoints
)

// swapTTL is how long a fetched SwapInfo is considered fresh enough to skip
// re-querying QGA. We refresh on tick if older than this.
const swapTTL = 5 * time.Second

// guestUptimeTTL controls how often we re-query QGA for guest uptime. Uptime
// changes monotonically (or resets on reboot), so a slower refresh is fine.
const guestUptimeTTL = 10 * time.Second

// Model is the root Bubble Tea model.
type Model struct {
	client *lv.Client

	// refreshInterval controls the snapshot tick rate. Set via WithRefreshInterval.
	refreshInterval time.Duration

	// mode is the current high-level UI state.
	mode viewMode

	// prevMode remembers the view we were in before opening the help modal,
	// so dismissing help returns the user to where they came from.
	prevMode viewMode

	snap *lv.Snapshot
	err  error

	// History keyed by domain UUID — survives across refreshes.
	history map[string]*domHistory

	// QGA-backed swap info, keyed by domain name.
	swap map[string]lv.SwapInfo

	// QGA-backed guest uptime info, keyed by domain name. Used to detect
	// in-guest reboots that the host-side qemu process start time misses.
	guestUptime map[string]lv.GuestUptime

	// host info — fetched once at startup, immutable thereafter.
	host lv.HostInfo

	// host dynamic stats — refreshed every tick. We keep the most recent
	// sample so we can compute CPU% as a delta on the next arrival.
	hostStats    lv.HostStats
	hostCPUPct   float64
	hostCPUHist  []float64
	hostHasStats bool

	// Layout.
	width, height int

	// Selection (index into the filtered, sorted list).
	selected int
	offset   int // first visible row

	// Filter mode (for the main VM list).
	filtering bool
	filter    string

	// Detail view state.
	detailFor       string   // domain name we're showing detail for
	detailXML       string
	detailLines     []string // cached line-split of detailXML
	detailScroll    int
	detailSearch    string   // active search query (empty = none)
	detailSearching bool     // currently typing into the / prompt
	detailMatches   []int    // line indices matching detailSearch
	detailMatchIdx  int      // index into detailMatches for current cursor

	// Info view state (structured per-VM panel, Enter target).
	infoFor    string
	info       lv.DomainInfo
	infoErr    error
	infoScroll int

	// Graphs view: which sub-tab is active (0=CPU, 1=MEM, 2=DISK, 3=NET).
	graphsTab   int
	graphsCache string // pre-rendered body, rebuilt on tick or tab change
	graphsDirty bool   // true when the cache needs rebuilding

	// Snapshot view state.
	snapshotsFor  string              // domain name we're showing snapshots for
	snapshots     []lv.DomainSnapshot // current list
	snapshotsErr  error               // last load error
	snapshotsSel  int                 // selected snapshot index
	snapshotInput bool                // typing a name for a new snapshot
	snapshotName  string              // text being typed for the new name

	// Networks view state.
	networks    []lv.Network
	networksSel int
	networksErr error

	// DHCP leases view state (drill-down from networks).
	leases    []lv.DHCPLease
	leasesFor string // network name
	leasesErr error

	// Storage pools view state.
	pools    []lv.StoragePool
	poolsSel int
	poolsErr error

	// Storage volumes view state (drill-down from pools).
	volumes    []lv.StorageVolume
	volumesFor string // pool name
	volumesSel int
	volumesErr error

	// Hosts view state — the multi-host list and async probe results.
	hosts      []config.Host
	hostsSel   int
	hostsErr   error
	hostsProbe map[string]hostProbeStatus

	// Hosts view: two-step text input for the "a" (add host) flow.
	// Stage 0 = idle, 1 = typing the nickname, 2 = typing the URI.
	hostInputStage int
	hostInputName  string
	hostInputURI   string

	// Command palette state (entered via `:`).
	commanding bool   // currently typing a `:` command
	command    string // text being typed

	// Confirm dialog (for destructive actions).
	confirming    bool
	confirmAction string // e.g. "destroy"
	confirmName   string

	// Transient flash message in the status bar.
	flash      string
	flashUntil time.Time

	// Column sort. sortColumn indexes into the list's sortable columns.
	sortColumn sortColumn
	sortDesc   bool

	// activeColumns is the VM-list column slice honouring the user's
	// config.yaml column-visibility preferences. Always a subset of
	// vmColumns; required columns (NAME, STATE, IP) are always present.
	activeColumns []column
}

// sortColumn enumerates the sortable columns in the VM list. The order
// matches the visible column order so that the number-key bindings (1..9)
// line up with the columns the master sees.
type sortColumn int

const (
	sortByName sortColumn = iota + 1 // 1
	sortByState                       // 2
	sortByIP                          // 3
	sortByOS                          // 4
	sortByVCPU                        // 5
	sortByMem                         // 6
	sortByMemPct                      // 7
	sortByCPU                         // 8
	sortByUptime                      // 9
)

func (s sortColumn) String() string {
	switch s {
	case sortByName:
		return "name"
	case sortByState:
		return "state"
	case sortByIP:
		return "IP"
	case sortByOS:
		return "OS"
	case sortByVCPU:
		return "vCPU"
	case sortByMem:
		return "MEM"
	case sortByMemPct:
		return "MEM%"
	case sortByCPU:
		return "CPU%"
	case sortByUptime:
		return "uptime"
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
		guestUptime:     make(map[string]lv.GuestUptime),
		hostsProbe:      make(map[string]hostProbeStatus),
		sortColumn:      sortByState, // running first by default
		activeColumns:   vmColumns,
	}
}

// WithConfig applies user-level config.yaml preferences to the model:
// default sort column + direction and column visibility. Refresh
// interval is applied separately via WithRefreshInterval so the CLI
// --refresh flag can override the config without a second lookup.
func (m Model) WithConfig(cfg config.Config) Model {
	m.sortColumn = sortColumnFromID(cfg.List.SortBy)
	m.sortDesc = cfg.List.SortReverse
	m.activeColumns = filterActiveColumns(vmColumns, cfg.List.Columns)
	ApplyTheme(cfg.Theme)
	return m
}

// (sort enum values now start at 1, so the zero value of sortColumn is invalid;
// New always sets it to sortByState above.)


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

// Every async message carries the URI of the libvirt client that produced
// it, so late results from a previous host switch can be identified and
// discarded in Update() rather than corrupting current state.

type snapshotMsg struct {
	uri  string
	snap *lv.Snapshot
	err  error
}

type actionResultMsg struct {
	uri    string
	action string
	name   string
	err    error
}

type detailLoadedMsg struct {
	uri  string
	name string
	xml  string
	err  error
}

type hostLoadedMsg struct {
	uri  string
	host lv.HostInfo
	err  error
}

type hostStatsLoadedMsg struct {
	uri   string
	stats lv.HostStats
	err   error
}

type snapshotsLoadedMsg struct {
	uri    string
	domain string
	list   []lv.DomainSnapshot
	err    error
}

type networksLoadedMsg struct {
	uri  string
	list []lv.Network
	err  error
}

type leasesLoadedMsg struct {
	uri     string
	netName string
	list    []lv.DHCPLease
	err     error
}

type poolsLoadedMsg struct {
	uri  string
	list []lv.StoragePool
	err  error
}

type volumesLoadedMsg struct {
	uri  string
	pool string
	list []lv.StorageVolume
	err  error
}

type swapMsg struct {
	uri  string
	name string
	info lv.SwapInfo
}

type guestUptimeMsg struct {
	uri  string
	name string
	info lv.GuestUptime
}

// ──────────────────────────── Commands ───────────────────────────────────────

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func loadCmd(c *lv.Client) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		snap, err := c.Snapshot()
		return snapshotMsg{uri: uri, snap: snap, err: err}
	}
}

func actionCmd(c *lv.Client, action, name string, fn func(string) error) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		return actionResultMsg{uri: uri, action: action, name: name, err: fn(name)}
	}
}

func loadDetailCmd(c *lv.Client, name string) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		x, err := c.XMLDesc(name)
		return detailLoadedMsg{uri: uri, name: name, xml: x, err: err}
	}
}

func loadHostCmd(c *lv.Client) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		h, err := c.Host()
		return hostLoadedMsg{uri: uri, host: h, err: err}
	}
}

func loadHostStatsCmd(c *lv.Client) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		s, err := c.HostStats()
		return hostStatsLoadedMsg{uri: uri, stats: s, err: err}
	}
}

func loadSnapshotsCmd(c *lv.Client, domain string) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		list, err := c.ListSnapshots(domain)
		return snapshotsLoadedMsg{uri: uri, domain: domain, list: list, err: err}
	}
}

func loadNetworksCmd(c *lv.Client) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		list, err := c.ListNetworks()
		return networksLoadedMsg{uri: uri, list: list, err: err}
	}
}

func loadLeasesCmd(c *lv.Client, netName string) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		list, err := c.ListDHCPLeases(netName)
		return leasesLoadedMsg{uri: uri, netName: netName, list: list, err: err}
	}
}

func loadPoolsCmd(c *lv.Client) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		list, err := c.ListStoragePools()
		return poolsLoadedMsg{uri: uri, list: list, err: err}
	}
}

func loadVolumesCmd(c *lv.Client, pool string) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		list, err := c.ListVolumes(pool)
		return volumesLoadedMsg{uri: uri, pool: pool, list: list, err: err}
	}
}

func swapCmd(c *lv.Client, name string) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		return swapMsg{uri: uri, name: name, info: c.Swap(name)}
	}
}

func guestUptimeCmd(c *lv.Client, name string) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		return guestUptimeMsg{uri: uri, name: name, info: c.QueryGuestUptime(name)}
	}
}

// ──────────────────────────── Init ───────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadCmd(m.client),
		loadHostCmd(m.client),
		loadHostStatsCmd(m.client),
		// Seed (if missing) and load the hosts file so :host add does not
		// overwrite an existing one and the list is ready on first open.
		loadHostsListCmd(m.client.URI()),
		tickCmd(m.refreshInterval),
	)
}

// ──────────────────────────── Update ─────────────────────────────────────────

// stale reports whether a message from client uri is still relevant.
// After a host switch the old client's async results must be dropped so
// they don't corrupt state for the new connection.
func (m Model) stale(uri string) bool {
	if uri == "" || m.client == nil {
		return false
	}
	return uri != m.client.URI()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		return m, tea.Batch(
			loadCmd(m.client),
			loadHostStatsCmd(m.client),
			tickCmd(m.refreshInterval),
		)

	case snapshotMsg:
		if m.stale(msg.uri) {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.snap = msg.snap
		m.updateHistory()
		m.boundSelection()
		if m.mode == viewGraphs {
			m.rebuildGraphsCache()
		}
		return m, tea.Batch(m.maybeFetchSwap(), m.maybeFetchGuestUptime())

	case swapMsg:
		if m.stale(msg.uri) || m.snap == nil {
			return m, nil
		}
		m.swap[msg.name] = msg.info
		// Record swap used% in the domain's history for the MEM graphs.
		if msg.info.Available && msg.info.HasSwap && msg.info.TotalKB > 0 {
			for _, d := range m.snap.Domains {
				if d.Name == msg.name {
					if h := m.history[d.UUID]; h != nil {
						pct := float64(msg.info.UsedKB) / float64(msg.info.TotalKB) * 100
						h.swapUsedPct = appendCap(h.swapUsedPct, pct, historyWindow)
					}
					break
				}
			}
		}
		return m, nil

	case guestUptimeMsg:
		if m.stale(msg.uri) {
			return m, nil
		}
		m.guestUptime[msg.name] = msg.info
		return m, nil

	case hostLoadedMsg:
		if m.stale(msg.uri) {
			return m, nil
		}
		if msg.err == nil {
			m.host = msg.host
		}
		return m, nil

	case hostStatsLoadedMsg:
		if m.stale(msg.uri) {
			return m, nil
		}
		if msg.err == nil {
			// Compute CPU% from delta against the *immediately* previous sample
			// (which is currently in m.hostStats — we have not overwritten it yet).
			if m.hostHasStats {
				dTotal := float64(msg.stats.CPUTotal() - m.hostStats.CPUTotal())
				dActive := float64(msg.stats.CPUActive() - m.hostStats.CPUActive())
				if dTotal > 0 {
					m.hostCPUPct = dActive / dTotal * 100
					m.hostCPUHist = appendCap(m.hostCPUHist, m.hostCPUPct, historyWindow)
				}
			}
			m.hostStats = msg.stats
			m.hostHasStats = true
		}
		return m, nil

	case snapshotsLoadedMsg:
		// Discard stale results that arrive after the user switched VMs
		// or hosts.
		if m.stale(msg.uri) || msg.domain != m.snapshotsFor {
			return m, nil
		}
		// Sort into parent-first DFS order so the selection index
		// matches what the user sees in the tree rendering. The
		// prefix strings are recomputed at render time from the
		// already-sorted slice.
		sorted, _ := sortSnapshotsAsTree(msg.list)
		m.snapshots = sorted
		m.snapshotsErr = msg.err
		if m.snapshotsSel >= len(m.snapshots) {
			m.snapshotsSel = len(m.snapshots) - 1
		}
		if m.snapshotsSel < 0 {
			m.snapshotsSel = 0
		}
		return m, nil

	case networksLoadedMsg:
		if m.stale(msg.uri) {
			return m, nil
		}
		m.networks = msg.list
		m.networksErr = msg.err
		if m.networksSel >= len(m.networks) {
			m.networksSel = len(m.networks) - 1
		}
		if m.networksSel < 0 {
			m.networksSel = 0
		}
		return m, nil

	case leasesLoadedMsg:
		if m.stale(msg.uri) || msg.netName != m.leasesFor {
			return m, nil
		}
		m.leases = msg.list
		m.leasesErr = msg.err
		return m, nil

	case poolsLoadedMsg:
		if m.stale(msg.uri) {
			return m, nil
		}
		m.pools = msg.list
		m.poolsErr = msg.err
		if m.poolsSel >= len(m.pools) {
			m.poolsSel = len(m.pools) - 1
		}
		if m.poolsSel < 0 {
			m.poolsSel = 0
		}
		return m, nil

	case volumesLoadedMsg:
		// Discard stale results that arrive after the user switched pools
		// or hosts.
		if m.stale(msg.uri) || msg.pool != m.volumesFor {
			return m, nil
		}
		m.volumes = msg.list
		m.volumesErr = msg.err
		if m.volumesSel >= len(m.volumes) {
			m.volumesSel = len(m.volumes) - 1
		}
		if m.volumesSel < 0 {
			m.volumesSel = 0
		}
		return m, nil

	case hostsLoadedMsg:
		m.hosts = msg.list
		m.hostsErr = msg.err
		if m.hostsSel >= len(m.hosts) {
			m.hostsSel = len(m.hosts) - 1
		}
		if m.hostsSel < 0 {
			m.hostsSel = 0
		}
		// Probe immediately so the status column fills in.
		if msg.err == nil && len(m.hosts) > 0 {
			return m, probeAllHostsCmd(m.hosts)
		}
		return m, nil

	case hostProbedMsg:
		if m.hostsProbe == nil {
			m.hostsProbe = make(map[string]hostProbeStatus)
		}
		m.hostsProbe[msg.name] = msg.status
		return m, nil

	case connectedMsg:
		var cmd tea.Cmd
		m, cmd = m.applyConnected(msg)
		return m, cmd

	case connectErrMsg:
		m.flashf("✗ connect %s: %v", msg.nick, msg.err)
		return m, nil

	case actionResultMsg:
		if m.stale(msg.uri) {
			return m, nil
		}
		if msg.err != nil {
			m.flashf("✗ %s %s: %v", msg.action, msg.name, msg.err)
		} else if msg.action == "pause" {
			m.flashf("✓ paused %s — press p to resume", msg.name)
		} else {
			m.flashf("✓ %s %s", msg.action, msg.name)
		}
		// Refresh immediately after a successful action. The reload depends
		// on which view the user is currently in.
		cmds := []tea.Cmd{loadCmd(m.client)}
		switch m.mode {
		case viewSnapshots:
			if m.snapshotsFor != "" {
				cmds = append(cmds, loadSnapshotsCmd(m.client, m.snapshotsFor))
			}
		case viewNetworks:
			cmds = append(cmds, loadNetworksCmd(m.client))
		case viewPools:
			cmds = append(cmds, loadPoolsCmd(m.client))
		case viewVolumes:
			if m.volumesFor != "" {
				cmds = append(cmds, loadVolumesCmd(m.client, m.volumesFor))
			}
		case viewInfo:
			if m.infoFor != "" {
				cmds = append(cmds, loadInfoCmd(m.client, m.infoFor))
			}
		}
		return m, tea.Batch(cmds...)

	case detailLoadedMsg:
		// Discard stale results from a previously-opened detail view or host.
		if m.stale(msg.uri) || msg.name != m.detailFor {
			return m, nil
		}
		if msg.err != nil {
			m.flashf("✗ load detail %s: %v", msg.name, msg.err)
			m.mode = viewMain
			return m, nil
		}
		m.detailXML = msg.xml
		m.detailLines = strings.Split(msg.xml, "\n")
		m.detailScroll = 0
		m.detailSearch = ""
		m.detailMatches = nil
		m.detailMatchIdx = 0
		return m, nil

	case infoLoadedMsg:
		// Discard results from a previously-opened info view or host.
		if m.stale(msg.uri) || msg.name != m.infoFor {
			return m, nil
		}
		m.info = msg.info
		m.infoErr = msg.err
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		ret, cmd := m.handleKey(msg)
		rm := ret.(Model)
		if rm.graphsDirty && rm.mode == viewGraphs {
			rm.rebuildGraphsCache()
		}
		return rm, cmd
	}
	return m, nil
}

// handleKey routes keypresses based on current mode. Mode-specific handlers
// run first because they own their own confirm/input sub-states (e.g. snapshot
// view has its own confirm dialog for revert/delete). The global confirming /
// filtering / commanding states only apply to the main VM list view.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Tab cycles through the top-level views, but not while a text
	// input is active (filter, command palette, snapshot name, host
	// input, detail search). Shift-Tab walks the same ring backwards.
	if !m.isTextInputting() {
		switch msg.String() {
		case "tab":
			return m.cycleMode()
		case "shift+tab":
			return m.cycleModeReverse()
		}
	}

	switch m.mode {
	case viewInfo:
		return m.handleInfoKey(msg)
	case viewDetail:
		return m.handleDetailKey(msg)
	case viewGraphs:
		return m.handleGraphsKey(msg)
	case viewSnapshots:
		return m.handleSnapshotsKey(msg)
	case viewNetworks:
		return m.handleNetworksKey(msg)
	case viewPools:
		return m.handlePoolsKey(msg)
	case viewVolumes:
		return m.handleVolumesKey(msg)
	case viewLeases:
		return m.handleLeasesKey(msg)
	case viewHosts:
		return m.handleHostsKey(msg)
	}
	switch {
	case m.confirming:
		return m.handleConfirmKey(msg)
	case m.filtering:
		return m.handleFilterKey(msg)
	case m.commanding:
		return m.handleCommandKey(msg)
	default:
		return m.handleNormalKey(msg)
	}
}

func (m Model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Help modal swallows everything except its own dismiss keys.
	if m.mode == viewHelp {
		switch msg.String() {
		case "?", "esc", "q":
			// Return to the view we came from. Fall back to viewMain if
			// somehow we got here without a previous mode set.
			if m.prevMode != viewHelp && m.prevMode != 0 {
				m.mode = m.prevMode
			} else {
				m.mode = viewMain
			}
			m.prevMode = viewMain
		}
		return m, nil
	}

	doms := m.visibleDomains()

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "?":
		m.prevMode = m.mode
		m.mode = viewHelp
		return m, nil

	case ":":
		m.commanding = true
		m.command = ""
		return m, nil

	// Sort by column. Same key again toggles direction. Numbers are aligned
	// with the visible column order in the list table.
	case "1":
		m.toggleSort(sortByName)
		return m, nil
	case "2":
		m.toggleSort(sortByState)
		return m, nil
	case "3":
		m.toggleSort(sortByIP)
		return m, nil
	case "4":
		m.toggleSort(sortByOS)
		return m, nil
	case "5":
		m.toggleSort(sortByVCPU)
		return m, nil
	case "6":
		m.toggleSort(sortByMem)
		return m, nil
	case "7":
		m.toggleSort(sortByMemPct)
		return m, nil
	case "8":
		m.toggleSort(sortByCPU)
		return m, nil
	case "9":
		m.toggleSort(sortByUptime)
		return m, nil

	case "j", "down":
		if m.selected < len(doms)-1 {
			m.selected++
		}
		return m, tea.Batch(m.maybeFetchSwap(), m.maybeFetchGuestUptime())

	case "k", "up":
		if m.selected > 0 {
			m.selected--
		}
		return m, tea.Batch(m.maybeFetchSwap(), m.maybeFetchGuestUptime())

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

	case "enter":
		// Structured info pane. Raw XML is one keypress away via "x".
		if d, ok := m.currentDomain(); ok {
			m.mode = viewInfo
			m.infoFor = d.Name
			m.info = lv.DomainInfo{Name: d.Name}
			m.infoErr = nil
			m.infoScroll = 0
			return m, loadInfoCmd(m.client, d.Name)
		}
		return m, nil

	// ── Lifecycle actions ──
	case "s":
		if d, ok := m.currentDomain(); ok && d.State != lv.StateRunning {
			return m, actionCmd(m.client, "start", d.Name, m.client.Start)
		}
		return m, nil

	case "S":
		// Graceful shutdown is destructive enough to warrant a confirmation —
		// a busy guest losing power can corrupt state.
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning {
			m.confirming = true
			m.confirmAction = "shutdown"
			m.confirmName = d.Name
		}
		return m, nil

	case "D":
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning {
			m.confirming = true
			m.confirmAction = "destroy"
			m.confirmName = d.Name
		}
		return m, nil

	case "R":
		// Reboot is destructive — always confirmed.
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning {
			m.confirming = true
			m.confirmAction = "reboot"
			m.confirmName = d.Name
		}
		return m, nil

	case "p":
		// Toggle: pause a running VM, resume a paused one.
		if d, ok := m.currentDomain(); ok {
			switch d.State {
			case lv.StateRunning:
				m.confirming = true
				m.confirmAction = "pause"
				m.confirmName = d.Name
			case lv.StatePaused:
				return m, actionCmd(m.client, "resume", d.Name, m.client.Resume)
			}
		}
		return m, nil

	case "o":
		// SSH into the guest. Requires a validated IP address — reject
		// anything net.ParseIP does not accept, to stop a hostile DHCP
		// lease / ARP entry from smuggling ssh options.
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning && d.IP != "" {
			if net.ParseIP(d.IP) == nil {
				m.flashf("✗ ssh %s: refusing invalid IP %q", d.Name, d.IP)
				return m, nil
			}
			return m, m.runSSH(d.IP)
		}
		return m, nil

	case "c":
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning {
			return m, m.runConsole(d.Name)
		}
		return m, nil

	case "v":
		// Graphical console via virt-viewer — Linux AND Windows friendly.
		// Launched detached so dirt remains usable while the window is open.
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning {
			return m, m.runViewer(d.Name)
		}
		return m, nil

	case "e":
		if d, ok := m.currentDomain(); ok {
			return m, m.runEdit(d.Name)
		}
		return m, nil

	case "x":
		// Open the raw XML detail view.
		if d, ok := m.currentDomain(); ok {
			m.mode = viewDetail
			m.detailFor = d.Name
			m.detailXML = "(loading…)"
			m.detailLines = []string{m.detailXML}
			return m, loadDetailCmd(m.client, d.Name)
		}
		return m, nil

	case "U":
		// Undefine a stopped VM. Destructive → always confirmed.
		// (Was on lowercase x until we promoted x to open the XML view.)
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
		m.filter = runeBackspace(m.filter)
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
		m.mode = viewMain
		return m, nil

	case "enter":
		m.mode = viewMain
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

	case "?":
		m.prevMode = m.mode
		m.mode = viewHelp
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
		if m.detailSearch != "" {
			m.detailSearch = runeBackspace(m.detailSearch)
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
		case "reboot":
			return m, actionCmd(m.client, "reboot", name, m.client.Reboot)
		case "shutdown":
			return m, actionCmd(m.client, "shutdown", name, m.client.Shutdown)
		case "pause":
			return m, actionCmd(m.client, "pause", name, m.client.Suspend)
		case "stop-net":
			return m, networkActionCmd(m.client, "stop network", name, m.client.StopNetwork)
		case "stop-pool":
			return m, networkActionCmd(m.client, "stop pool", name, m.client.StopPool)
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
	case sortByIP:
		if a.IP != b.IP {
			return flip(a.IP < b.IP)
		}
		return a.Name < b.Name
	case sortByOS:
		if a.OS != b.OS {
			return flip(a.OS < b.OS)
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
	case sortByMemPct:
		va, _ := domainMemUsedPct(a)
		vb, _ := domainMemUsedPct(b)
		if va != vb {
			return flip(va > vb)
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
	case sortByUptime:
		// Use the most accurate uptime source we have for each side.
		ha := m.history[a.UUID]
		hb := m.history[b.UUID]
		ua, _ := effectiveUptime(a, ha, m.guestUptime[a.Name])
		ub, _ := effectiveUptime(b, hb, m.guestUptime[b.Name])
		if ua != ub {
			return flip(ua > ub)
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

// rebuildGraphsCache pre-renders the current graphs tab body so that
// View() can return it instantly without running ntcharts on every frame.
func (m *Model) rebuildGraphsCache() {
	d, ok := m.currentDomain()
	if !ok {
		m.graphsCache = ""
		return
	}
	h := m.history[d.UUID]
	if h == nil {
		h = &domHistory{}
	}
	w := m.contentWidth() - 4
	if w < 40 {
		w = 40
	}
	switch m.graphsTab {
	case graphTabCPU:
		m.graphsCache = renderCPUTab(h, w, m.refreshInterval)
	case graphTabMEM:
		m.graphsCache = renderMEMTab(h, w, m.refreshInterval)
	case graphTabDISK:
		m.graphsCache = renderDISKTab(h, w, m.refreshInterval)
	case graphTabNET:
		m.graphsCache = renderNETTab(h, w, m.refreshInterval)
	}
	m.graphsDirty = false
}

// handleCommandKey handles keypresses while typing a `:` command.
func (m Model) handleCommandKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.commanding = false
		m.command = ""
		return m, nil
	case "enter":
		cmd := strings.TrimSpace(m.command)
		m.commanding = false
		m.command = ""
		return m.execCommand(cmd)
	case "backspace":
		m.command = runeBackspace(m.command)
		return m, nil
	default:
		if len(msg.String()) == 1 {
			m.command += msg.String()
		}
		return m, nil
	}
}

// execCommand interprets a `:` command and switches view mode.
func (m Model) execCommand(cmd string) (Model, tea.Cmd) {
	switch cmd {
	case "":
		return m, nil
	case "q", "quit":
		return m, tea.Quit
	case "h", "help":
		m.mode = viewHelp
		return m, nil
	case "vm", "vms", "domain", "domains":
		m.mode = viewMain
		return m, nil
	case "snap", "snapshot", "snapshots":
		d, ok := m.currentDomain()
		if !ok {
			m.flashf("no domain selected")
			return m, nil
		}
		m.mode = viewSnapshots
		m.snapshotsFor = d.Name
		m.snapshotsSel = 0
		m.snapshots = nil
		return m, loadSnapshotsCmd(m.client, d.Name)
	case "net", "network", "networks":
		m.mode = viewNetworks
		m.networksSel = 0
		m.networks = nil
		return m, loadNetworksCmd(m.client)
	case "pool", "pools":
		m.mode = viewPools
		m.poolsSel = 0
		m.pools = nil
		return m, loadPoolsCmd(m.client)
	case "host", "hosts":
		m.mode = viewHosts
		m.hostsSel = 0
		return m, loadHostsListCmd(m.client.URI())
	case "perf", "graph", "graphs":
		if _, ok := m.currentDomain(); !ok {
			m.flashf("no domain selected")
			return m, nil
		}
		m.mode = viewGraphs
		m.graphsDirty = true
		return m, nil
	}
	m.flashf("unknown command: %s", cmd)
	return m, nil
}

// handleSnapshotsKey handles keypresses while in the snapshots view.
func (m Model) handleSnapshotsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirming {
		return m.handleSnapshotConfirmKey(msg)
	}
	if m.snapshotInput {
		return m.handleSnapshotInputKey(msg)
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
	if navSelect(msg.String(), &m.snapshotsSel, len(m.snapshots)) {
		return m, nil
	}
	switch msg.String() {
	case "c":
		// Begin "create" input prompt.
		m.snapshotInput = true
		m.snapshotName = ""
		return m, nil
	case "r":
		// Revert (with confirm).
		if s, ok := m.currentSnapshot(); ok {
			m.confirming = true
			m.confirmAction = "revert"
			m.confirmName = s.Name
		}
		return m, nil
	case "D", "x":
		// Delete (with confirm).
		if s, ok := m.currentSnapshot(); ok {
			m.confirming = true
			m.confirmAction = "delete-snap"
			m.confirmName = s.Name
		}
		return m, nil
	case "R", "F5":
		return m, loadSnapshotsCmd(m.client, m.snapshotsFor)
	}
	return m, nil
}

func (m Model) handleSnapshotInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.snapshotInput = false
		m.snapshotName = ""
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.snapshotName)
		m.snapshotInput = false
		m.snapshotName = ""
		if name == "" {
			m.flashf("✗ snapshot name cannot be empty")
			return m, nil
		}
		domain := m.snapshotsFor
		uri := m.client.URI()
		return m, tea.Batch(
			func() tea.Msg {
				err := m.client.CreateSnapshot(domain, name, "")
				return actionResultMsg{uri: uri, action: "create-snap", name: name, err: err}
			},
		)
	case "backspace":
		// Snapshot names are ASCII-only (isValidSnapshotChar), but use
		// runeBackspace for consistency with other input handlers.
		m.snapshotName = runeBackspace(m.snapshotName)
		return m, nil
	default:
		// Only accept characters QEMU's snapshot job IDs allow:
		// [A-Za-z0-9._-]. Anything else (notably space) is silently
		// ignored — typing it does nothing visible, which keeps the
		// user from accidentally creating a name libvirt will reject.
		s := msg.String()
		if len(s) == 1 && isValidSnapshotChar(s[0]) {
			m.snapshotName += s
		}
		return m, nil
	}
}

// isValidSnapshotChar reports whether b is allowed in a libvirt/qemu snapshot
// name. Matches QEMU's internal job-ID grammar [A-Za-z0-9._-].
func isValidSnapshotChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	case b >= '0' && b <= '9':
		return true
	case b == '_' || b == '-' || b == '.':
		return true
	}
	return false
}

func (m Model) handleSnapshotConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		name := m.confirmName
		action := m.confirmAction
		domain := m.snapshotsFor
		m.confirming = false
		m.confirmName = ""
		m.confirmAction = ""
		uri := m.client.URI()
		switch action {
		case "revert":
			return m, func() tea.Msg {
				err := m.client.RevertSnapshot(domain, name)
				return actionResultMsg{uri: uri, action: "revert", name: name, err: err}
			}
		case "delete-snap":
			return m, func() tea.Msg {
				err := m.client.DeleteSnapshot(domain, name)
				return actionResultMsg{uri: uri, action: "delete-snap", name: name, err: err}
			}
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

func (m Model) currentSnapshot() (lv.DomainSnapshot, bool) {
	if m.snapshotsSel < 0 || m.snapshotsSel >= len(m.snapshots) {
		return lv.DomainSnapshot{}, false
	}
	return m.snapshots[m.snapshotsSel], true
}

// handleNetworksKey handles keys while in the networks view.
func (m Model) handleNetworksKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirming {
		return m.handleConfirmKey(msg)
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
	if navSelect(msg.String(), &m.networksSel, len(m.networks)) {
		return m, nil
	}
	switch msg.String() {
	case "s":
		if n, ok := m.currentNetwork(); ok && !n.Active {
			return m, networkActionCmd(m.client, "start", n.Name, m.client.StartNetwork)
		}
		return m, nil
	case "S":
		// Stop is dangerous — kills connectivity for every VM on this network.
		if n, ok := m.currentNetwork(); ok && n.Active {
			m.confirming = true
			m.confirmAction = "stop-net"
			m.confirmName = n.Name
		}
		return m, nil
	case "a":
		if n, ok := m.currentNetwork(); ok {
			return m, networkActionCmd(m.client, "autostart", n.Name, m.client.ToggleNetworkAutostart)
		}
		return m, nil
	case "enter":
		// Drill into DHCP leases for the selected network.
		if n, ok := m.currentNetwork(); ok && n.Active {
			m.mode = viewLeases
			m.leasesFor = n.Name
			m.leases = nil
			m.leasesErr = nil
			return m, loadLeasesCmd(m.client, n.Name)
		}
		return m, nil
	case "R", "F5":
		return m, loadNetworksCmd(m.client)
	}
	return m, nil
}

// handleLeasesKey handles keys while in the DHCP leases view.
func (m Model) handleLeasesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?":
		m.prevMode = m.mode
		m.mode = viewHelp
		return m, nil
	case "esc", "q":
		m.mode = viewNetworks
		return m, nil
	case "R", "F5":
		return m, loadLeasesCmd(m.client, m.leasesFor)
	}
	return m, nil
}

// handlePoolsKey handles keys while in the storage pools view.
func (m Model) handlePoolsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirming {
		return m.handleConfirmKey(msg)
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
	if navSelect(msg.String(), &m.poolsSel, len(m.pools)) {
		return m, nil
	}
	switch msg.String() {
	case "s":
		if p, ok := m.currentPool(); ok && p.State != "running" {
			return m, networkActionCmd(m.client, "start", p.Name, m.client.StartPool)
		}
		return m, nil
	case "S":
		// Stopping a pool while VMs use its volumes is dangerous.
		if p, ok := m.currentPool(); ok && p.State == "running" {
			m.confirming = true
			m.confirmAction = "stop-pool"
			m.confirmName = p.Name
		}
		return m, nil
	case "enter":
		if p, ok := m.currentPool(); ok {
			m.mode = viewVolumes
			m.volumesFor = p.Name
			m.volumesSel = 0
			m.volumes = nil
			return m, loadVolumesCmd(m.client, p.Name)
		}
		return m, nil
	case "R", "F5":
		return m, loadPoolsCmd(m.client)
	}
	return m, nil
}

// handleVolumesKey handles keys while in the volumes drill-down.
func (m Model) handleVolumesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?":
		m.prevMode = m.mode
		m.mode = viewHelp
		return m, nil
	case "esc", "q":
		m.mode = viewPools
		return m, nil
	}
	if navSelect(msg.String(), &m.volumesSel, len(m.volumes)) {
		return m, nil
	}
	switch msg.String() {
	case "R", "F5":
		return m, loadVolumesCmd(m.client, m.volumesFor)
	}
	return m, nil
}

func (m Model) currentNetwork() (lv.Network, bool) {
	if m.networksSel < 0 || m.networksSel >= len(m.networks) {
		return lv.Network{}, false
	}
	return m.networks[m.networksSel], true
}

func (m Model) currentPool() (lv.StoragePool, bool) {
	if m.poolsSel < 0 || m.poolsSel >= len(m.pools) {
		return lv.StoragePool{}, false
	}
	return m.pools[m.poolsSel], true
}

// networkActionCmd is a generic action runner used by network and pool keys.
func networkActionCmd(c *lv.Client, action, name string, fn func(string) error) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		return actionResultMsg{uri: uri, action: action, name: name, err: fn(name)}
	}
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

// maybeFetchGuestUptime returns a Cmd that queries QGA for guest uptime.
// For local URIs we only probe the highlighted VM, because source #2
// (qemu process start time from /proc/<pid>.mtime) already provides an
// accurate uptime for every running VM. For remote URIs source #2 is
// unavailable, so we probe *every* running VM whose cached value is
// stale — otherwise non-focused rows would show "—" forever.
func (m Model) maybeFetchGuestUptime() tea.Cmd {
	if m.snap == nil {
		return nil
	}
	// Remote: probe all running VMs.
	if !strings.HasPrefix(m.client.URI(), "qemu:///") {
		var cmds []tea.Cmd
		for _, d := range m.snap.Domains {
			if d.State != lv.StateRunning {
				continue
			}
			cached, has := m.guestUptime[d.Name]
			if has && time.Since(cached.FetchedAt) < guestUptimeTTL {
				continue
			}
			cmds = append(cmds, guestUptimeCmd(m.client, d.Name))
		}
		if len(cmds) == 0 {
			return nil
		}
		return tea.Batch(cmds...)
	}
	// Local: the per-VM BootedAt already covers the list, so only the
	// highlighted VM needs the QGA refinement (which catches in-guest
	// reboots that source #2 misses).
	d, ok := m.currentDomain()
	if !ok || d.State != lv.StateRunning {
		return nil
	}
	cached, has := m.guestUptime[d.Name]
	if has && time.Since(cached.FetchedAt) < guestUptimeTTL {
		return nil
	}
	return guestUptimeCmd(m.client, d.Name)
}

// isTextInputting reports whether the user is currently typing into a
// text input. Used by Tab mode-cycling so the key does not steal focus
// mid-word.
func (m Model) isTextInputting() bool {
	return m.commanding || m.filtering || m.detailSearching ||
		m.snapshotInput || m.hostInputStage > 0
}

// cycleMode advances to the next top-level view: main → hosts →
// networks → pools → snapshots (if a VM is selected) → main. Detail,
// help, volumes, and the splash are not part of the cycle; from any of
// them Tab lands in main first, then resumes the cycle on the next tab.
func (m Model) cycleMode() (tea.Model, tea.Cmd) {
	next := viewMain
	switch m.mode {
	case viewMain:
		next = viewHosts
	case viewHosts:
		next = viewNetworks
	case viewNetworks:
		next = viewPools
	case viewPools:
		if _, ok := m.currentDomain(); ok {
			next = viewSnapshots
		} else {
			next = viewMain
		}
	case viewSnapshots:
		next = viewMain
	default:
		next = viewMain
	}
	return m.enterView(next)
}

// cycleModeReverse walks the same ring as cycleMode but backwards:
// main → snapshots (if a VM is selected) → pools → networks → hosts →
// main. Invoked by Shift-Tab.
func (m Model) cycleModeReverse() (tea.Model, tea.Cmd) {
	next := viewMain
	switch m.mode {
	case viewMain:
		if _, ok := m.currentDomain(); ok {
			next = viewSnapshots
		} else {
			next = viewPools
		}
	case viewSnapshots:
		next = viewPools
	case viewPools:
		next = viewNetworks
	case viewNetworks:
		next = viewHosts
	case viewHosts:
		next = viewMain
	default:
		next = viewMain
	}
	return m.enterView(next)
}

// enterView is the shared "switch to this top-level view" helper used
// by both cycleMode and cycleModeReverse. It handles the per-view
// bookkeeping (selection reset, initial load command) so the cycle
// functions only need to decide which view comes next.
func (m Model) enterView(next viewMode) (tea.Model, tea.Cmd) {
	switch next {
	case viewMain:
		m.mode = viewMain
		return m, nil
	case viewHosts:
		m.mode = viewHosts
		if m.hostsSel >= len(m.hosts) {
			m.hostsSel = 0
		}
		return m, loadHostsListCmd(m.client.URI())
	case viewNetworks:
		m.mode = viewNetworks
		m.networksSel = 0
		m.networks = nil
		return m, loadNetworksCmd(m.client)
	case viewPools:
		m.mode = viewPools
		m.poolsSel = 0
		m.pools = nil
		return m, loadPoolsCmd(m.client)
	case viewSnapshots:
		d, ok := m.currentDomain()
		if !ok {
			m.mode = viewMain
			return m, nil
		}
		m.mode = viewSnapshots
		m.snapshotsFor = d.Name
		m.snapshotsSel = 0
		m.snapshots = nil
		return m, loadSnapshotsCmd(m.client, d.Name)
	}
	return m, nil
}

// runSSH suspends the Bubble Tea program and execs `ssh <ip>`. The TUI
// resumes when the SSH session ends (exit / Ctrl-D / connection close).
func (m Model) runSSH(ip string) tea.Cmd {
	uri := m.client.URI()
	return tea.ExecProcess(exec.Command("ssh", ip), func(err error) tea.Msg {
		if err != nil {
			return actionResultMsg{uri: uri, action: "ssh", name: ip, err: err}
		}
		return actionResultMsg{uri: uri, action: "ssh", name: ip}
	})
}

// runConsole suspends the Bubble Tea program, execs `virsh console <name>`,
// then resumes the TUI when the user detaches with Ctrl+].
func (m Model) runConsole(name string) tea.Cmd {
	uri := m.client.URI()
	return tea.ExecProcess(exec.Command("virsh", "-c", uri, "console", name), func(err error) tea.Msg {
		if err != nil {
			return actionResultMsg{uri: uri, action: "console", name: name, err: err}
		}
		// Reset terminal stdin echo just in case virsh left it weird.
		_ = os.Stdin.Sync()
		return actionResultMsg{uri: uri, action: "console", name: name}
	})
}

// runEdit suspends the Bubble Tea program and execs `virsh edit <name>`,
// which opens $EDITOR on the live XML. Resumes when the editor exits.
func (m Model) runEdit(name string) tea.Cmd {
	uri := m.client.URI()
	return tea.ExecProcess(exec.Command("virsh", "-c", uri, "edit", name), func(err error) tea.Msg {
		if err != nil {
			return actionResultMsg{uri: uri, action: "edit", name: name, err: err}
		}
		return actionResultMsg{uri: uri, action: "edit", name: name}
	})
}

// runViewer launches virt-viewer as a detached GUI subprocess so the
// master can look at the VM's graphical display without suspending dirt.
// Works for any guest OS (Linux and Windows alike) because it attaches
// to libvirt's SPICE/VNC channel, not the serial port. Flashes an error
// if virt-viewer is not installed or cannot be launched.
func (m Model) runViewer(name string) tea.Cmd {
	uri := m.client.URI()
	return func() tea.Msg {
		cmd := exec.Command("virt-viewer", "--connect", uri, name)
		// Fully detach from dirt's terminal — no stdio and a fresh
		// session so the viewer survives if dirt quits or the tty
		// closes. Setsid is Linux-specific and fine for this project.
		cmd.Stdin = nil
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err != nil {
			return actionResultMsg{uri: uri, action: "viewer", name: name, err: err}
		}
		// Reap the child in the background so Go's runtime does not
		// leave it as a zombie when it eventually exits.
		go func() { _ = cmd.Wait() }()
		return actionResultMsg{uri: uri, action: "viewer", name: name}
	}
}
