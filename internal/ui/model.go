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
	"github.com/llcoolkm/dirt/internal/backend"
	"github.com/llcoolkm/dirt/internal/config"
	"github.com/llcoolkm/dirt/internal/lv"
)

// defaultRefreshInterval is the fall-back tick rate when none is configured.
const defaultRefreshInterval = 1 * time.Second

// bulkUndefineCeiling is the threshold at which the normal y/d confirm
// prompt is replaced by a typed-phrase confirmation. Above this, a
// stray keystroke cannot annihilate a large set.
// TODO: expose via config once :config lands.
const bulkUndefineCeiling = 20

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
	viewJobs                      // active + recent background jobs
	viewMigrate                   // destination picker for live migration
	viewColumns                   // picker for which VM-list columns are shown
	viewAll                       // aggregated multi-host VM list (`:all`)
)

// swapTTL is how long a fetched SwapInfo is considered fresh enough to skip
// re-querying QGA. We refresh on tick if older than this.
const swapTTL = 5 * time.Second

// guestUptimeTTL controls how often we re-query QGA for guest uptime. Uptime
// changes monotonically (or resets on reboot), so a slower refresh is fine.
const guestUptimeTTL = 10 * time.Second

// Model is the root Bubble Tea model.
type Model struct {
	client backend.Backend

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

	// Marks for bulk operations. Keyed by domain UUID so marks
	// survive sort/filter/refresh until the domain itself disappears.
	marks map[string]bool

	// Grouping: when set, the list is rendered with group headers
	// (and domains in folded groups are hidden from visibleDomains).
	// Empty string means no grouping.
	groupBy      string
	foldedGroups map[string]bool

	// Vim-style numeric prefix: digits accumulate here and the next
	// motion key consumes the count (returning 1 if unset). Reset by
	// Esc or by the motion that consumes it.
	pendingCount int

	// Last cursor direction: +1 for downward motions (j/pgdown/…),
	// -1 for upward (k/pgup/…). SPACE uses this to decide which way
	// to advance after marking. Zero defaults to +1.
	lastDir int

	// markAdvance controls SPACE's advance direction:
	//   - "directional" / "" — follow lastDir (default).
	//   - "down"             — always move down.
	//   - "none"             — don't move; SPACE is a pure toggle.
	markAdvance string

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
	networks         []lv.Network
	bridgeRates      map[string]bridgeRate // keyed by bridge name
	networksSel      int
	networksErr      error
	networksSortIdx  int  // index into the networks header columns
	networksSortDesc bool

	// DHCP leases view state (drill-down from networks).
	leases    []lv.DHCPLease
	leasesFor string // network name
	leasesErr error

	// Storage pools view state.
	pools         []lv.StoragePool
	poolsSel      int
	poolsErr      error
	poolsSortIdx  int
	poolsSortDesc bool

	// Storage volumes view state (drill-down from pools).
	volumes         []lv.StorageVolume
	volumesFor      string // pool name
	volumesSel      int
	volumesErr      error
	volumesSortIdx  int
	volumesSortDesc bool

	// Hosts view state — the multi-host list and async probe results.
	hosts         []config.Host
	hostsSel      int
	hostsErr      error
	hostsProbe    map[string]hostProbeStatus
	hostsSortIdx  int
	hostsSortDesc bool

	// Multi-host aggregated view (`:all`). Backends opened lazily on
	// first entry and reused thereafter. Snapshots refresh on every
	// tick while viewAll is active. Read-only in this iteration.
	allBackends   map[string]backend.Backend
	allSnapshots  map[string]*lv.Snapshot
	allErrs       map[string]error
	allSel        int

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
	confirmAction string   // e.g. "destroy"
	confirmName   string   // single-target mode
	confirmBulk   bool     // true when confirmTargets holds the set
	confirmTargets []string // bulk-mode victim list

	// Typed-confirmation state — engaged for irreversible bulk ops
	// above bulkUndefineCeiling. The master must type the expected
	// phrase exactly, with an optional " delete" suffix to also
	// remove storage. Anything else cancels.
	confirmTyping       bool
	confirmTypingAction string
	confirmTypingExpect string
	confirmTypingInput  string

	// Transient flash message in the status bar.
	flash      string
	flashUntil time.Time

	// Column sort. sortColumn indexes into the list's sortable columns.
	sortColumn sortColumn
	sortDesc   bool

	// columnsSel is the cursor in the columns picker view.
	columnsSel int

	// activeColumns is the VM-list column slice honouring the user's
	// config.yaml column-visibility preferences. Always a subset of
	// vmColumns; required columns (NAME, STATE, IP) are always present.
	activeColumns []column

	// Background jobs (live migration, slow snapshot ops, …). Keyed by
	// job ID. Active jobs appear in the bottom status bar; :jobs shows
	// active + recent history.
	jobs map[string]*Job

	// Migration destination picker state — the hosts list is reused,
	// but we need our own selection index for the modal overlay.
	migrateFrom string // source VM name (running domain)
	migrateSel  int    // index into m.hosts (destination)

	// Clone prompt state: user pressed `C` on a stopped VM and is
	// typing the new name in the status bar.
	cloneFrom bool   // true while prompting
	cloneSrc  string // source VM name
	cloneName string // typed new name

	// Volume create prompt state (inside the volumes drill-down).
	// Stage 0 = idle, 1 = typing name, 2 = typing size.
	volInputStage int
	volInputName  string
	volInputSize  string

	// Hot-plug attach prompt. Stage 0 = idle, 1 = pick device type
	// (d=disk, n=nic), 2 = typing device-specific param (path/target
	// for disk, network name for NIC).
	attachDomain string
	attachStage  int    // 0=idle, 1=pick type, 2=param1, 3=param2
	attachVerb   string // "attach" or "detach" — same prompt machinery
	attachType   string // "disk" or "nic"
	attachParam1 string // attach disk: source path, attach nic: network name
	//                     detach disk: target dev (vdb), detach nic: MAC
	attachParam2 string // attach disk only: target device (vdb/vdc)
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
	sortByTag                         // 10
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
	case sortByTag:
		return "tag"
	}
	return "?"
}

// New constructs a fresh Model bound to the given libvirt client.
func New(c backend.Backend) Model {
	return Model{
		client:          c,
		refreshInterval: defaultRefreshInterval,
		history:         make(map[string]*domHistory),
		swap:            make(map[string]lv.SwapInfo),
		guestUptime:     make(map[string]lv.GuestUptime),
		hostsProbe:      make(map[string]hostProbeStatus),
		jobs:            make(map[string]*Job),
		marks:           make(map[string]bool),
		sortColumn:      sortByState, // running first by default
		activeColumns:   vmColumns,
		allBackends:     make(map[string]backend.Backend),
		allSnapshots:    make(map[string]*lv.Snapshot),
		allErrs:         make(map[string]error),
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
	m.markAdvance = cfg.List.MarkAdvance
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

// bulkActionResultMsg is delivered once after a bulk run completes.
// count is the number of targets attempted; failed lists human-
// readable per-target errors (at most a few shown in the flash).
type bulkActionResultMsg struct {
	uri    string
	action string
	count  int
	failed []string
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

type bridgeStatsMsg struct {
	uri   string
	stats []lv.BridgeStats
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

func loadCmd(c backend.Backend) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		snap, err := c.Snapshot()
		return snapshotMsg{uri: uri, snap: snap, err: err}
	}
}

func actionCmd(c backend.Backend, action, name string, fn func(string) error) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		return actionResultMsg{uri: uri, action: action, name: name, err: fn(name)}
	}
}

// bulkActionCmd runs fn serially against each name. Libvirt serialises
// state transitions on its own, and serial execution keeps the flash
// message and error list deterministic. Returns a single summary msg.
func bulkActionCmd(c backend.Backend, action string, names []string, fn func(string) error) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		var failed []string
		for _, n := range names {
			if err := fn(n); err != nil {
				failed = append(failed, fmt.Sprintf("%s (%v)", n, err))
			}
		}
		return bulkActionResultMsg{uri: uri, action: action, count: len(names), failed: failed}
	}
}

func loadDetailCmd(c backend.Backend, name string) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		x, err := c.XMLDesc(name)
		return detailLoadedMsg{uri: uri, name: name, xml: x, err: err}
	}
}

func loadHostCmd(c backend.Backend) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		h, err := c.Host()
		return hostLoadedMsg{uri: uri, host: h, err: err}
	}
}

func loadHostStatsCmd(c backend.Backend) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		s, err := c.HostStats()
		return hostStatsLoadedMsg{uri: uri, stats: s, err: err}
	}
}

func loadSnapshotsCmd(c backend.Backend, domain string) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		list, err := c.ListSnapshots(domain)
		return snapshotsLoadedMsg{uri: uri, domain: domain, list: list, err: err}
	}
}

func loadNetworksCmd(c backend.Backend) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		list, err := c.ListNetworks()
		return networksLoadedMsg{uri: uri, list: list, err: err}
	}
}

// loadBridgeStatsCmd reads /sys/class/net/<bridge>/statistics for
// each named bridge. Remote libvirt URIs simply produce OK=false
// entries, since /sys is the local host's view.
func loadBridgeStatsCmd(uri string, names []string) tea.Cmd {
	return func() tea.Msg {
		return bridgeStatsMsg{uri: uri, stats: lv.ReadBridgeStats(names)}
	}
}

func loadLeasesCmd(c backend.Backend, netName string) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		list, err := c.ListDHCPLeases(netName)
		return leasesLoadedMsg{uri: uri, netName: netName, list: list, err: err}
	}
}

func loadPoolsCmd(c backend.Backend) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		list, err := c.ListStoragePools()
		return poolsLoadedMsg{uri: uri, list: list, err: err}
	}
}

func loadVolumesCmd(c backend.Backend, pool string) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		list, err := c.ListVolumes(pool)
		return volumesLoadedMsg{uri: uri, pool: pool, list: list, err: err}
	}
}

func swapCmd(c backend.Backend, name string) tea.Cmd {
	uri := c.URI()
	return func() tea.Msg {
		return swapMsg{uri: uri, name: name, info: c.Swap(name)}
	}
}

func guestUptimeCmd(c backend.Backend, name string) tea.Cmd {
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
		return m, tea.ClearScreen

	case tickMsg:
		cmds := []tea.Cmd{
			loadCmd(m.client),
			loadHostStatsCmd(m.client),
			tickCmd(m.refreshInterval),
		}
		// While in the aggregated view, fan out a snapshot to every
		// open backend so the list stays live.
		if m.mode == viewAll && len(m.allBackends) > 0 {
			cmds = append(cmds, allRefreshCmd(m.allBackends))
		}
		return m, tea.Batch(cmds...)

	case allBackendOpenedMsg:
		if msg.err != nil {
			m.allErrs[msg.nick] = msg.err
			return m, nil
		}
		// Drop any prior error for this nick on success.
		delete(m.allErrs, msg.nick)
		m.allBackends[msg.nick] = msg.client
		// Trigger an immediate snapshot for the freshly-opened host so
		// the user sees rows arrive without waiting a full tick.
		c := msg.client
		nick := msg.nick
		return m, func() tea.Msg {
			snap, err := c.Snapshot()
			return allSnapshotMsg{nick: nick, uri: c.URI(), snap: snap, err: err}
		}

	case allSnapshotMsg:
		if msg.err != nil {
			m.allErrs[msg.nick] = msg.err
			// Keep any prior snapshot so the row doesn't blank out
			// on a single transient error.
			return m, nil
		}
		delete(m.allErrs, msg.nick)
		m.allSnapshots[msg.nick] = msg.snap
		// Clamp the cursor in case rows shrank.
		rows := len(m.allRows())
		if m.allSel >= rows {
			m.allSel = rows - 1
		}
		if m.allSel < 0 {
			m.allSel = 0
		}
		return m, nil

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
		m.pruneMarks()
		m.boundSelection()
		if m.mode == viewGraphs {
			m.rebuildGraphsCache()
		}
		// Check for anomalies — flash a warning if any VM is hot.
		m.checkAnomalies()
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
		// Kick off a host-side bridge-stats read for every active
		// network. Remote libvirt URIs will get OK=false back and
		// render "—" — no special-casing needed.
		bridges := make([]string, 0, len(m.networks))
		for _, n := range m.networks {
			if n.Active && n.Bridge != "" {
				bridges = append(bridges, n.Bridge)
			}
		}
		return m, loadBridgeStatsCmd(m.client.URI(), bridges)

	case configEditedMsg:
		if msg.err != nil {
			m.flashf("✗ config edit: %v", msg.err)
			return m, nil
		}
		// Re-read and re-apply. Failures here keep the running session
		// alive — the master can fix the file and try again.
		cfg, err := config.LoadConfig()
		if err != nil {
			m.flashf("✗ config reload: %v", err)
			return m, nil
		}
		m.sortColumn = sortColumnFromID(cfg.List.SortBy)
		m.sortDesc = cfg.List.SortReverse
		m.activeColumns = filterActiveColumns(vmColumns, cfg.List.Columns)
		m.markAdvance = cfg.List.MarkAdvance
		ApplyTheme(cfg.Theme)
		m.flashf("✓ config reloaded (%s)", msg.path)
		return m, nil

	case bridgeStatsMsg:
		if m.stale(msg.uri) {
			return m, nil
		}
		m.updateBridgeStats(msg.stats)
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

	case bulkActionResultMsg:
		if m.stale(msg.uri) {
			return m, nil
		}
		ok := msg.count - len(msg.failed)
		switch {
		case len(msg.failed) == 0:
			m.flashf("✓ %s %d VMs", msg.action, msg.count)
		case ok == 0:
			m.flashf("✗ %s: %d/%d failed — %s",
				msg.action, len(msg.failed), msg.count, bulkFailSummary(msg.failed))
		default:
			m.flashf("%s: %d ok, %d failed — %s",
				msg.action, ok, len(msg.failed), bulkFailSummary(msg.failed))
		}
		// Refresh the snapshot so the table reflects the new states.
		return m, loadCmd(m.client)

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

	case jobStartedMsg:
		if m.jobs == nil {
			m.jobs = make(map[string]*Job)
		}
		m.jobs[msg.job.ID] = msg.job
		return m, nil

	case jobProgressMsg:
		if j, ok := m.jobs[msg.id]; ok && j.Running() {
			if msg.phase != "" {
				j.Phase = msg.phase
			}
			if msg.progress >= 0 {
				j.Progress = msg.progress
			}
			if msg.dataTotal > 0 {
				j.DataTotal = msg.dataTotal
			}
			if msg.dataDone > 0 {
				j.DataDone = msg.dataDone
			}
		}
		return m, nil

	case jobDoneMsg:
		if j, ok := m.jobs[msg.id]; ok {
			j.FinishedAt = time.Now()
			j.Err = msg.err
			j.Cancel = nil
			if msg.err != nil {
				m.flashf("✗ %s %s: %v", j.Kind, j.Target, msg.err)
			} else {
				m.flashf("✓ %s %s", j.Kind, j.Target)
			}
		}
		// Prune jobs older than 10 minutes so the map stays bounded.
		m.pruneOldJobs(10 * time.Minute)
		// Refresh state that the completed job may have changed.
		cmds := []tea.Cmd{loadCmd(m.client)}
		switch m.mode {
		case viewSnapshots:
			if m.snapshotsFor != "" {
				cmds = append(cmds, loadSnapshotsCmd(m.client, m.snapshotsFor))
			}
		case viewInfo:
			if m.infoFor != "" {
				cmds = append(cmds, loadInfoCmd(m.client, m.infoFor))
			}
		}
		return m, tea.Batch(cmds...)

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
	case viewJobs:
		return m.handleJobsKey(msg)
	case viewMigrate:
		return m.handleMigrateKey(msg)
	case viewHosts:
		return m.handleHostsKey(msg)
	case viewColumns:
		return m.handleColumnsKey(msg)
	case viewAll:
		return m.handleAllKey(msg)
	}
	switch {
	case m.confirmTyping:
		return m.handleTypedConfirmKey(msg)
	case m.confirming:
		return m.handleConfirmKey(msg)
	case m.cloneFrom:
		return m.handleCloneKey(msg)
	case m.attachStage > 0:
		return m.handleAttachKey(msg)
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

	// Mark management for bulk operations. shift+space clears all
	// marks — unreliable on most terminals, so Esc is the dependable
	// path and :mark none the palette fallback.
	case " ":
		return m.doSpace(doms)

	case "shift+space":
		m.clearMarks()
		return m, nil

	case "*":
		m.invertMarksVisible()
		return m, nil

	case "z":
		// Toggle the fold of the group the cursor is on. No-op when
		// grouping is off — the master gets a flash.
		if m.groupBy == "" {
			m.flashf("not grouped — :group os|state first")
			return m, nil
		}
		if d, ok := m.currentDomain(); ok {
			gk := groupKeyFor(d, m.groupBy)
			if m.foldedGroups == nil {
				m.foldedGroups = make(map[string]bool)
			}
			m.foldedGroups[gk] = !m.foldedGroups[gk]
		}
		return m, nil

	// Numeric prefix: digits accumulate a count consumed by the next
	// motion key. "0" is a digit only when a count is already pending
	// — otherwise it has no binding, so leading zeros never start a
	// phantom count.
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.accumulateCount(msg.String()[0] - '0')
		return m, nil
	case "0":
		if m.pendingCount > 0 {
			m.accumulateCount(0)
		}
		return m, nil

	case "j", "down":
		n := m.consumeCount()
		m.lastDir = +1
		m.selected += n
		if m.selected >= len(doms) {
			m.selected = len(doms) - 1
		}
		return m, tea.Batch(m.maybeFetchSwap(), m.maybeFetchGuestUptime())

	case "k", "up":
		n := m.consumeCount()
		m.lastDir = -1
		m.selected -= n
		if m.selected < 0 {
			m.selected = 0
		}
		return m, tea.Batch(m.maybeFetchSwap(), m.maybeFetchGuestUptime())

	case "g", "home":
		m.pendingCount = 0
		m.lastDir = +1 // from the top, only down makes sense
		m.selected = 0
		return m, nil

	case "G", "end":
		// With a count, jump to row N (1-indexed), vim-style.
		if n := m.consumeCount(); n > 1 {
			m.selected = n - 1
			if m.selected >= len(doms) {
				m.selected = len(doms) - 1
			}
			if m.selected < 0 {
				m.selected = 0
			}
			m.lastDir = +1
			return m, nil
		}
		m.lastDir = -1 // from the bottom, only up makes sense
		if len(doms) > 0 {
			m.selected = len(doms) - 1
		}
		return m, nil

	case "ctrl+d", "pgdown":
		step := m.consumeCount()
		if step == 1 {
			step = 10
		}
		m.lastDir = +1
		m.selected += step
		if m.selected >= len(doms) {
			m.selected = len(doms) - 1
		}
		return m, nil

	case "ctrl+u", "pgup":
		step := m.consumeCount()
		if step == 1 {
			step = 10
		}
		m.lastDir = -1
		m.selected -= step
		if m.selected < 0 {
			m.selected = 0
		}
		return m, nil

	case "/":
		m.filtering = true
		m.filter = ""
		return m, nil

	case "esc":
		// Return to a quiet state, one layer at a time: a pending
		// count first, marks next, then any active filter. Vim-like
		// "back to normal" semantics.
		if m.pendingCount > 0 {
			m.pendingCount = 0
			return m, nil
		}
		if m.markCount() > 0 {
			m.clearMarks()
			return m, nil
		}
		m.filter = ""
		return m, nil

	case "enter":
		// Structured info pane. Raw XML is one keypress away via "x".
		m.warnSingleTargetWithMarks("enter")
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
	// When marks are set, these dispatch to every eligible marked
	// domain; otherwise they act on the cursor row as before.
	case "s":
		if m.markCount() > 0 {
			names := m.markedDomainsInStates(lv.StateShutoff, lv.StateCrashed)
			if len(names) == 0 {
				m.flashf("no marked VMs in a startable state")
				return m, nil
			}
			return m, bulkActionCmd(m.client, "start", names, m.client.Start)
		}
		if d, ok := m.currentDomain(); ok && d.State != lv.StateRunning {
			return m, actionCmd(m.client, "start", d.Name, m.client.Start)
		}
		return m, nil

	case "S":
		// Graceful shutdown is destructive enough to warrant a confirmation —
		// a busy guest losing power can corrupt state.
		if m.markCount() > 0 {
			names := m.markedDomainsInStates(lv.StateRunning)
			if len(names) == 0 {
				m.flashf("no marked VMs are running")
				return m, nil
			}
			m.confirming = true
			m.confirmAction = "shutdown"
			m.confirmBulk = true
			m.confirmTargets = names
			return m, nil
		}
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning {
			m.confirming = true
			m.confirmAction = "shutdown"
			m.confirmName = d.Name
		}
		return m, nil

	case "D":
		if m.markCount() > 0 {
			names := m.markedDomainsInStates(lv.StateRunning)
			if len(names) == 0 {
				m.flashf("no marked VMs are running")
				return m, nil
			}
			m.confirming = true
			m.confirmAction = "destroy"
			m.confirmBulk = true
			m.confirmTargets = names
			return m, nil
		}
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning {
			m.confirming = true
			m.confirmAction = "destroy"
			m.confirmName = d.Name
		}
		return m, nil

	case "R":
		// Reboot is destructive — always confirmed.
		if m.markCount() > 0 {
			names := m.markedDomainsInStates(lv.StateRunning)
			if len(names) == 0 {
				m.flashf("no marked VMs are running")
				return m, nil
			}
			m.confirming = true
			m.confirmAction = "reboot"
			m.confirmBulk = true
			m.confirmTargets = names
			return m, nil
		}
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning {
			m.confirming = true
			m.confirmAction = "reboot"
			m.confirmName = d.Name
		}
		return m, nil

	case "p":
		// Bulk: pause every marked running VM. Resume is intentionally
		// single-target for now — reviving a set requires no confirm
		// and "p" as a true toggle on mixed states is confusing.
		if m.markCount() > 0 {
			names := m.markedDomainsInStates(lv.StateRunning)
			if len(names) == 0 {
				m.flashf("no marked VMs are running (resume is single-target via p)")
				return m, nil
			}
			m.confirming = true
			m.confirmAction = "pause"
			m.confirmBulk = true
			m.confirmTargets = names
			return m, nil
		}
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
		m.warnSingleTargetWithMarks("ssh")
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning && d.IP != "" {
			if net.ParseIP(d.IP) == nil {
				m.flashf("✗ ssh %s: refusing invalid IP %q", d.Name, d.IP)
				return m, nil
			}
			return m, m.runSSH(d.IP)
		}
		return m, nil

	case "M":
		// Live migrate the selected running VM to another host. Opens
		// the destination picker.
		m.warnSingleTargetWithMarks("migrate")
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning {
			if len(m.migrateCandidates()) == 0 {
				m.flashf("no other hosts configured — add one in :host first")
				return m, nil
			}
			m.mode = viewMigrate
			m.migrateFrom = d.Name
			m.migrateSel = 0
		}
		return m, nil

	case "C":
		// Clone the selected stopped VM. Opens an inline name prompt
		// in the status bar; Enter kicks off the background job.
		m.warnSingleTargetWithMarks("clone")
		if d, ok := m.currentDomain(); ok && d.State != lv.StateRunning {
			m.cloneFrom = true
			m.cloneSrc = d.Name
			m.cloneName = d.Name + "-clone"
		} else {
			m.flashf("clone only works on stopped VMs")
		}
		return m, nil

	case "A":
		// Hot-plug a device to the selected running VM.
		m.warnSingleTargetWithMarks("attach")
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning {
			m.attachDomain = d.Name
			m.attachVerb = "attach"
			m.attachStage = 1
		} else {
			m.flashf("attach only works on running VMs")
		}
		return m, nil

	case "X":
		// Hot-remove a device from the selected running VM. Reuses
		// the attach prompt machinery — single key collects disk
		// target (vdb, vdc, …) or NIC MAC, then libvirt does the rest.
		m.warnSingleTargetWithMarks("detach")
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning {
			m.attachDomain = d.Name
			m.attachVerb = "detach"
			m.attachStage = 1
		} else {
			m.flashf("detach only works on running VMs")
		}
		return m, nil

	case "c":
		m.warnSingleTargetWithMarks("console")
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning {
			return m, m.runConsole(d.Name)
		}
		return m, nil

	case "v":
		// Graphical console via virt-viewer — Linux AND Windows friendly.
		// Launched detached so dirt remains usable while the window is open.
		m.warnSingleTargetWithMarks("viewer")
		if d, ok := m.currentDomain(); ok && d.State == lv.StateRunning {
			return m, m.runViewer(d.Name)
		}
		return m, nil

	case "e":
		m.warnSingleTargetWithMarks("edit")
		if d, ok := m.currentDomain(); ok {
			return m, m.runEdit(d.Name)
		}
		return m, nil

	case "x":
		// Open the raw XML detail view.
		m.warnSingleTargetWithMarks("xml")
		if d, ok := m.currentDomain(); ok {
			m.mode = viewDetail
			m.detailFor = d.Name
			m.detailXML = "(loading…)"
			m.detailLines = []string{m.detailXML}
			return m, loadDetailCmd(m.client, d.Name)
		}
		return m, nil

	case "U":
		// Undefine. Destructive → always confirmed. In bulk, above the
		// ceiling the y/d/any-key prompt is replaced with a typed
		// "undefine N" confirmation — a single slip cannot annihilate
		// a large set.
		if m.markCount() > 0 {
			names := m.markedDomainsInStates(lv.StateShutoff, lv.StateCrashed)
			if len(names) == 0 {
				m.flashf("no marked VMs in a stopped state — cannot undefine")
				return m, nil
			}
			if len(names) > bulkUndefineCeiling {
				m.confirmTyping = true
				m.confirmTypingAction = "undefine"
				m.confirmTypingExpect = fmt.Sprintf("undefine %d", len(names))
				m.confirmTypingInput = ""
				m.confirmTargets = names
				return m, nil
			}
			m.confirming = true
			m.confirmAction = "undefine"
			m.confirmBulk = true
			m.confirmTargets = names
			return m, nil
		}
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
	// Undefine has a two-choice prompt: y = keep disks, d = delete storage.
	if m.confirmAction == "undefine" && msg.String() == "d" {
		if m.confirmBulk {
			names := m.confirmTargets
			m.resetConfirm()
			client := m.client
			uri := client.URI()
			return m, func() tea.Msg {
				var failed []string
				for _, n := range names {
					warnings, err := client.UndefineAndDelete(n)
					if err == nil && len(warnings) > 0 {
						err = fmt.Errorf("%s", strings.Join(warnings, ", "))
					}
					if err != nil {
						failed = append(failed, fmt.Sprintf("%s (%v)", n, err))
					}
				}
				return bulkActionResultMsg{uri: uri, action: "undefine+delete", count: len(names), failed: failed}
			}
		}
		name := m.confirmName
		m.resetConfirm()
		client := m.client
		uri := client.URI()
		return m, func() tea.Msg {
			warnings, err := client.UndefineAndDelete(name)
			if err == nil && len(warnings) > 0 {
				err = fmt.Errorf("undefined, but could not delete: %s",
					strings.Join(warnings, ", "))
			}
			return actionResultMsg{uri: uri, action: "undefine+delete", name: name, err: err}
		}
	}
	switch msg.String() {
	case "y", "Y":
		// Bulk dispatch when a target set was stashed.
		if m.confirmBulk {
			names := m.confirmTargets
			action := m.confirmAction
			m.resetConfirm()
			switch action {
			case "destroy":
				return m, bulkActionCmd(m.client, "destroy", names, m.client.Destroy)
			case "undefine":
				return m, bulkActionCmd(m.client, "undefine", names, m.client.Undefine)
			case "reboot":
				return m, bulkActionCmd(m.client, "reboot", names, m.client.Reboot)
			case "shutdown":
				return m, bulkActionCmd(m.client, "shutdown", names, m.client.Shutdown)
			case "pause":
				return m, bulkActionCmd(m.client, "pause", names, m.client.Suspend)
			}
			return m, nil
		}
		name := m.confirmName
		action := m.confirmAction
		m.resetConfirm()
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
		case "delete-vol":
			pool := m.volumesFor
			client := m.client
			return m, tea.Batch(
				func() tea.Msg {
					err := client.DeleteVolume(pool, name)
					return actionResultMsg{uri: client.URI(), action: "delete-vol", name: name, err: err}
				},
				loadVolumesCmd(client, pool),
			)
		}
		return m, nil
	default:
		m.resetConfirm()
		m.flash = "cancelled"
		m.flashUntil = time.Now().Add(2 * time.Second)
		return m, nil
	}
}

// resetConfirm clears all confirm-dialog state — single-target and
// bulk alike. Called from every terminal branch of handleConfirmKey
// so no stale confirmName or confirmTargets can bleed into the next
// action.
func (m *Model) resetConfirm() {
	m.confirming = false
	m.confirmAction = ""
	m.confirmName = ""
	m.confirmBulk = false
	m.confirmTargets = nil
}

func (m *Model) resetTypedConfirm() {
	m.confirmTyping = false
	m.confirmTypingAction = ""
	m.confirmTypingExpect = ""
	m.confirmTypingInput = ""
	m.confirmTargets = nil
}

// handleTypedConfirmKey runs the typed-phrase confirmation for bulk
// operations above the safety ceiling. The master must type the
// expected phrase (e.g. "undefine 47") and press Enter to proceed,
// or append " delete" before Enter to also remove storage. Esc or
// any non-matching phrase cancels.
func (m Model) handleTypedConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case "esc", "ctrl+c":
		m.resetTypedConfirm()
		m.flashf("cancelled")
		return m, nil
	case "backspace":
		m.confirmTypingInput = runeBackspace(m.confirmTypingInput)
		return m, nil
	case "enter":
		typed := m.confirmTypingInput
		expect := m.confirmTypingExpect
		withDelete := typed == expect+" delete"
		if typed != expect && !withDelete {
			m.resetTypedConfirm()
			m.flashf("phrase mismatch — cancelled")
			return m, nil
		}
		names := m.confirmTargets
		action := m.confirmTypingAction
		m.resetTypedConfirm()
		if action == "undefine" {
			client := m.client
			uri := client.URI()
			if withDelete {
				return m, func() tea.Msg {
					var failed []string
					for _, n := range names {
						warnings, err := client.UndefineAndDelete(n)
						if err == nil && len(warnings) > 0 {
							err = fmt.Errorf("%s", strings.Join(warnings, ", "))
						}
						if err != nil {
							failed = append(failed, fmt.Sprintf("%s (%v)", n, err))
						}
					}
					return bulkActionResultMsg{uri: uri, action: "undefine+delete", count: len(names), failed: failed}
				}
			}
			return m, bulkActionCmd(m.client, "undefine", names, m.client.Undefine)
		}
		return m, nil
	default:
		if len(s) == 1 {
			m.confirmTypingInput += s
		}
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

// visibleDomains returns the filtered + sorted slice currently shown
// in the table. When grouping is active, domains in folded groups
// are excluded so the master never selects something they cannot
// see — bulk actions on a folded group simply skip those rows.
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
		if m.groupBy != "" && m.foldedGroups[groupKeyFor(d, m.groupBy)] {
			continue
		}
		out = append(out, d)
	}
	sort.SliceStable(out, func(i, j int) bool {
		// When grouping, sort by group first so all members of one
		// group land contiguously, then by the active sort column.
		if m.groupBy != "" {
			ki := groupKeyFor(out[i], m.groupBy)
			kj := groupKeyFor(out[j], m.groupBy)
			if ki != kj {
				return ki < kj
			}
		}
		return m.lessDomain(out[i], out[j])
	})
	return out
}

// groupKeyFor returns the group bucket for a domain under the given
// group field. Unknown fields fall through to the empty key, which
// puts everything in one group (a no-op).
func groupKeyFor(d lv.Domain, field string) string {
	switch field {
	case "os":
		if d.OS == "" {
			return "(unknown)"
		}
		return d.OS
	case "state":
		return d.State.String()
	case "arch":
		if d.Arch == "" {
			return "(unknown)"
		}
		return d.Arch
	case "tag":
		if len(d.Tags) == 0 {
			return "(untagged)"
		}
		// Group by the first tag — VMs with multiple tags appear
		// under whichever sorts first.
		return d.Tags[0]
	case "host":
		return "" // single-host views; reserved for multi-host expansion
	}
	return ""
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
	case sortByTag:
		ta := strings.Join(a.Tags, ",")
		tb := strings.Join(b.Tags, ",")
		if ta != tb {
			return flip(ta < tb)
		}
		return a.Name < b.Name
	}
	return a.Name < b.Name
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

// warnSingleTargetWithMarks flashes a notice when the master fires a
// single-target action while marks exist. The action still proceeds
// on the cursor row — informational only, never blocking.
func (m *Model) warnSingleTargetWithMarks(action string) {
	if m.markCount() > 0 {
		m.flashf("%s is single-target — acting on cursor row", action)
	}
}

// ──────────────────────────── Marks ──────────────────────────────────────────

func (m Model) isMarked(uuid string) bool {
	return m.marks[uuid]
}

func (m Model) markCount() int { return len(m.marks) }

func (m *Model) toggleMark(uuid string) {
	if uuid == "" {
		return
	}
	if m.marks == nil {
		m.marks = make(map[string]bool)
	}
	if m.marks[uuid] {
		delete(m.marks, uuid)
	} else {
		m.marks[uuid] = true
	}
}

func (m *Model) clearMarks() {
	m.marks = make(map[string]bool)
}

// markAllVisible marks every domain currently shown by the active
// filter. Hidden-by-filter domains are left untouched.
func (m *Model) markAllVisible() {
	if m.marks == nil {
		m.marks = make(map[string]bool)
	}
	for _, d := range m.visibleDomains() {
		m.marks[d.UUID] = true
	}
}

// invertMarksVisible flips the mark on every currently visible domain.
// Marks on hidden-by-filter domains are preserved as-is.
func (m *Model) invertMarksVisible() {
	if m.marks == nil {
		m.marks = make(map[string]bool)
	}
	for _, d := range m.visibleDomains() {
		if m.marks[d.UUID] {
			delete(m.marks, d.UUID)
		} else {
			m.marks[d.UUID] = true
		}
	}
}

// pruneMarks drops marks whose UUID is no longer present in the
// current snapshot — domains that were undefined elsewhere, or
// vanished on a host switch.
func (m *Model) pruneMarks() {
	if len(m.marks) == 0 || m.snap == nil {
		return
	}
	alive := make(map[string]bool, len(m.snap.Domains))
	for _, d := range m.snap.Domains {
		alive[d.UUID] = true
	}
	for uuid := range m.marks {
		if !alive[uuid] {
			delete(m.marks, uuid)
		}
	}
}

// marksHiddenByFilter counts marks whose domain is not visible under
// the current filter. Used in the confirmation text for bulk actions
// so the master is never blindsided by hidden victims.
func (m Model) marksHiddenByFilter() int {
	if len(m.marks) == 0 {
		return 0
	}
	visible := make(map[string]bool, len(m.marks))
	for _, d := range m.visibleDomains() {
		if m.marks[d.UUID] {
			visible[d.UUID] = true
		}
	}
	return len(m.marks) - len(visible)
}

// markedDomainsInStates returns the names of marked domains whose
// state is one of the given states. Pulled straight from the current
// snapshot so a stale mark set cannot resurrect a vanished VM.
func (m Model) markedDomainsInStates(states ...lv.State) []string {
	if len(m.marks) == 0 || m.snap == nil {
		return nil
	}
	wanted := make(map[lv.State]bool, len(states))
	for _, s := range states {
		wanted[s] = true
	}
	out := make([]string, 0, len(m.marks))
	for _, d := range m.snap.Domains {
		if !m.marks[d.UUID] {
			continue
		}
		if len(wanted) > 0 && !wanted[d.State] {
			continue
		}
		out = append(out, d.Name)
	}
	return out
}

// accumulateCount appends a digit to the pending numeric prefix.
// Clamped at 9999 so a wedged keyboard cannot overflow selection.
func (m *Model) accumulateCount(d uint8) {
	next := m.pendingCount*10 + int(d)
	if next > 9999 {
		next = 9999
	}
	m.pendingCount = next
}

// consumeCount returns the pending count (≥1) and clears it. Use
// from every motion / operator handler that should respect a count.
func (m *Model) consumeCount() int {
	n := m.pendingCount
	m.pendingCount = 0
	if n < 1 {
		return 1
	}
	return n
}

// dirOrDown returns lastDir if set, otherwise +1 (down). SPACE uses
// this so the advance direction mirrors the master's last cursor
// motion.
func (m Model) dirOrDown() int {
	if m.lastDir == -1 {
		return -1
	}
	return +1
}

// doSpace handles SPACE in the main list: toggle the mark on the
// cursor row and advance `n` times (from the pending count, default
// 1). Direction follows m.markAdvance:
//   - "directional" / "" — last cursor motion (default).
//   - "down"             — always advance down.
//   - "none"             — pure toggle, cursor stays put.
// Leaves lastDir untouched so a run of SPACEs keeps travelling.
func (m Model) doSpace(doms []lv.Domain) (tea.Model, tea.Cmd) {
	n := m.consumeCount()
	stay := m.markAdvance == "none"
	dir := +1
	if !stay && m.markAdvance != "down" {
		dir = m.dirOrDown()
	}
	for i := 0; i < n; i++ {
		if d, ok := m.currentDomain(); ok {
			m.toggleMark(d.UUID)
		}
		if stay {
			continue
		}
		if dir == +1 {
			if m.selected < len(doms)-1 {
				m.selected++
			}
		} else {
			if m.selected > 0 {
				m.selected--
			}
		}
	}
	return m, nil
}

// bridgeRate caches the previous BridgeStats reading for a single
// bridge along with the wall-clock time it was taken — successive
// readings divide the byte / packet delta by the elapsed seconds to
// produce a rate. Negative deltas (counter wrap, interface reset)
// reset the cache rather than emitting bogus values.
type bridgeRate struct {
	prev      lv.BridgeStats
	prevAt    time.Time
	rxBps     float64
	txBps     float64
	rxPps     float64
	txPps     float64
	available bool
}

// updateBridgeStats folds a fresh BridgeStats batch into the rate
// cache. Time deltas under 100 ms are treated as no-progress so we
// don't divide by tiny numbers.
func (m *Model) updateBridgeStats(batch []lv.BridgeStats) {
	if m.bridgeRates == nil {
		m.bridgeRates = make(map[string]bridgeRate)
	}
	now := time.Now()
	for _, s := range batch {
		if !s.OK {
			delete(m.bridgeRates, s.Name)
			continue
		}
		prev, has := m.bridgeRates[s.Name]
		next := bridgeRate{prev: s, prevAt: now}
		if has && !prev.prevAt.IsZero() {
			dt := now.Sub(prev.prevAt).Seconds()
			if dt >= 0.1 && s.RxBytes >= prev.prev.RxBytes && s.TxBytes >= prev.prev.TxBytes {
				next.rxBps = float64(s.RxBytes-prev.prev.RxBytes) / dt
				next.txBps = float64(s.TxBytes-prev.prev.TxBytes) / dt
				next.rxPps = float64(s.RxPkts-prev.prev.RxPkts) / dt
				next.txPps = float64(s.TxPkts-prev.prev.TxPkts) / dt
				next.available = true
			}
		}
		m.bridgeRates[s.Name] = next
	}
}

// bulkFailSummary renders up to three failure entries from a bulk
// action for the status-bar flash, with an ellipsis when more remain.
func bulkFailSummary(failed []string) string {
	const max = 3
	if len(failed) <= max {
		return strings.Join(failed, "; ")
	}
	return strings.Join(failed[:max], "; ") + fmt.Sprintf("; +%d more", len(failed)-max)
}

// execMarkCommand routes :mark and its subcommands.
func (m Model) execMarkCommand(cmd string) Model {
	switch cmd {
	case "mark all":
		m.markAllVisible()
		m.flashf("marked %d visible", m.markCount())
	case "mark invert":
		m.invertMarksVisible()
		m.flashf("inverted — %d marked", m.markCount())
	case "mark none", "unmark":
		n := m.markCount()
		m.clearMarks()
		if n > 0 {
			m.flashf("cleared %d mark(s)", n)
		}
	case "mark":
		if m.markCount() == 0 {
			m.flashf("no marks — :mark [all|invert|none]")
		} else {
			m.flashf("%d marked", m.markCount())
		}
	}
	return m
}

// execSortCommand routes :sort [col] [desc]. Empty col flashes the
// current sort state; "desc" reverses direction.
func (m Model) execSortCommand(args string) Model {
	fields := strings.Fields(args)
	if len(fields) == 0 {
		m.flashf("current sort: %s — :sort <col> [desc]", sortColumnID(m.sortColumn))
		return m
	}
	col := fields[0]
	sc := sortColumnFromID(col)
	// sortColumnFromID falls through to sortByState on unknown ids;
	// guard against that so a typo doesn't silently resort.
	if !isSortableID(col) {
		m.flashf("unknown sort column: %s", col)
		return m
	}
	desc := false
	if len(fields) > 1 {
		switch fields[1] {
		case "desc", "reverse", "rev":
			desc = true
		case "asc":
			desc = false
		default:
			m.flashf("unknown direction: %s (use desc or asc)", fields[1])
			return m
		}
	}
	m.sortColumn = sc
	m.sortDesc = desc
	return m
}

// isSortableID reports whether id names a sortable column in vmColumns.
func isSortableID(id string) bool {
	for _, c := range vmColumns {
		if c.id == id && c.sort != 0 {
			return true
		}
	}
	return false
}

// sortColumnID is the reverse of sortColumnFromID — recovers the
// canonical id for a sortColumn enum value.
func sortColumnID(sc sortColumn) string {
	for _, c := range vmColumns {
		if c.sort == sc {
			return c.id
		}
	}
	return "?"
}

// execThemeCommand hot-swaps the colour palette. Empty or unknown
// names flash the list of available themes.
func (m Model) execThemeCommand(name string) Model {
	if name == "" {
		m.flashf(":theme %s", strings.Join(themeNames(), ", "))
		return m
	}
	if _, ok := themes[name]; !ok {
		m.flashf("unknown theme: %s — %s", name, strings.Join(themeNames(), ", "))
		return m
	}
	ApplyTheme(name)
	m.flashf("theme: %s", name)
	return m
}

// execSaveCommand serialises the master's current runtime preferences
// to ~/.local/state/dirt/state.yaml (XDG_STATE_HOME convention) so
// config.yaml — which the master may hand-edit — is left untouched
// by routine TUI churn. Theme, sort, column visibility, and
// mark-advance behaviour all round-trip through this file.
func (m Model) execSaveCommand() (Model, tea.Cmd) {
	visibility := m.currentColumnVisibility()
	rev := m.sortDesc
	state := config.State{
		Theme:       currentTheme,
		SortBy:      sortColumnID(m.sortColumn),
		SortReverse: &rev,
		MarkAdvance: m.markAdvance,
		Columns:     visibility,
	}
	if err := config.SaveState(state); err != nil {
		m.flashf("✗ save: %v", err)
		return m, nil
	}
	m.flashf("✓ saved → %s", config.StatePath())
	return m, nil
}

// execGroupCommand routes :group <field>. Empty arg flashes the
// current grouping; "none" turns it off and clears the fold map.
func (m Model) execGroupCommand(arg string) Model {
	if arg == "" {
		if m.groupBy == "" {
			m.flashf(":group os|state|none — currently ungrouped")
		} else {
			m.flashf("grouped by %s — :group none to clear", m.groupBy)
		}
		return m
	}
	switch arg {
	case "none", "off":
		m.groupBy = ""
		m.foldedGroups = nil
	case "os", "state", "arch", "tag":
		m.groupBy = arg
		if m.foldedGroups == nil {
			m.foldedGroups = make(map[string]bool)
		}
	default:
		m.flashf("unknown group field: %s — use os, state, arch, tag, or none", arg)
	}
	return m
}

// execExportCommand routes :export csv|json [path].
func (m Model) execExportCommand(args string) Model {
	fields := strings.Fields(args)
	if len(fields) == 0 {
		m.flashf(":export csv|json [path]")
		return m
	}
	format := fields[0]
	if format != "csv" && format != "json" {
		m.flashf("unknown format: %s — use csv or json", format)
		return m
	}
	dest := ""
	if len(fields) > 1 {
		dest = strings.Join(fields[1:], " ")
	}
	path, err := m.exportTable(format, dest)
	if err != nil {
		m.flashf("✗ export: %v", err)
		return m
	}
	m.flashf("✓ exported %d row(s) → %s", len(m.visibleDomains()), path)
	return m
}

// themeNames returns the registered theme names in a deterministic
// order so the flash text is stable between runs.
func themeNames() []string {
	names := make([]string, 0, len(themes))
	for n := range themes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// checkAnomalies scans every running VM's history for sustained high
// CPU or memory and flashes a warning in the status bar. The flash is
// set to 2× the refresh interval so it stays visible until the next
// tick replaces or clears it.
func (m *Model) checkAnomalies() {
	if m.snap == nil {
		return
	}
	var hot []string
	for _, d := range m.snap.Domains {
		if d.State != lv.StateRunning {
			continue
		}
		h := m.history[d.UUID]
		if h == nil {
			continue
		}
		alerts := h.checkAnomaly()
		for _, a := range alerts {
			hot = append(hot, d.Name+": "+a)
		}
	}
	if len(hot) > 0 {
		// Don't overwrite a user-initiated flash (action result).
		if m.flash == "" || time.Now().After(m.flashUntil) {
			msg := "⚠ " + strings.Join(hot, "  ·  ")
			m.flash = msg
			m.flashUntil = time.Now().Add(2 * m.refreshInterval)
		}
	}
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
	case "tab":
		m.command = completePaletteInput(m.command)
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

// uniquePrefixMatch returns the canonical command name if `prefix`
// uniquely matches exactly one entry in paletteCommands. Returns
// ("", false) if zero or multiple commands match.
func uniquePrefixMatch(prefix string) (string, bool) {
	if prefix == "" {
		return "", false
	}
	var match string
	count := 0
	for _, c := range paletteCommands {
		if strings.HasPrefix(c.name, prefix) {
			match = c.name
			count++
		}
	}
	if count == 1 {
		return match, true
	}
	return "", false
}

// completePaletteInput is the TAB handler for the `:` prompt. It
// extends the typed text to the longest unambiguous prefix — either
// the command name at the top level, or a sub-arg once the master has
// added a space. Unknown input is returned unchanged.
func completePaletteInput(input string) string {
	if idx := strings.Index(input, " "); idx >= 0 {
		base := input[:idx]
		// Preserve whatever spacing the master typed between the
		// command and the sub-arg prefix.
		rest := input[idx:]
		lead := rest[:len(rest)-len(strings.TrimLeft(rest, " "))]
		sub := strings.TrimLeft(rest, " ")
		for _, c := range paletteCommands {
			if c.name == base && len(c.args) > 0 {
				names := make([]string, len(c.args))
				for i, a := range c.args {
					names[i] = a.name
				}
				if extended, ok := extendPrefix(sub, names); ok {
					return base + lead + extended
				}
				return input
			}
		}
		return input
	}
	names := make([]string, len(paletteCommands))
	for i, c := range paletteCommands {
		names[i] = c.name
	}
	if extended, ok := extendPrefix(input, names); ok {
		return extended
	}
	return input
}

// extendPrefix returns the longest common prefix of every candidate
// starting with `prefix`. Returns ok=false when nothing matches;
// returns the single candidate verbatim when exactly one matches.
func extendPrefix(prefix string, candidates []string) (string, bool) {
	matches := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if strings.HasPrefix(c, prefix) {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		return "", false
	}
	if len(matches) == 1 {
		return matches[0], true
	}
	common := matches[0]
	for _, m := range matches[1:] {
		common = commonPrefix(common, m)
		if common == prefix {
			return prefix, true // no forward progress possible
		}
	}
	if len(common) > len(prefix) {
		return common, true
	}
	return prefix, true
}

// commonPrefix returns the longest string that is a prefix of both a
// and b, working on runes so multi-byte characters stay intact.
func commonPrefix(a, b string) string {
	ar := []rune(a)
	br := []rune(b)
	n := len(ar)
	if len(br) < n {
		n = len(br)
	}
	i := 0
	for i < n && ar[i] == br[i] {
		i++
	}
	return string(ar[:i])
}

// handleCloneKey runs the inline rename prompt triggered by C on a
// stopped VM. Enter kicks off the clone job; esc cancels.
func (m Model) handleCloneKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.cloneFrom = false
		m.cloneSrc = ""
		m.cloneName = ""
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.cloneName)
		src := m.cloneSrc
		m.cloneFrom = false
		m.cloneSrc = ""
		m.cloneName = ""
		if name == "" || name == src {
			m.flashf("✗ clone: new name must differ from source")
			return m, nil
		}
		client := m.client
		job := &Job{
			ID:       fmt.Sprintf("clone-%s-%s-%d", src, name, time.Now().UnixNano()),
			Kind:     "clone",
			Target:   src,
			Detail:   "→ " + name,
			Phase:    "copying disks",
			Progress: -1,
		}
		return m, runDomainJob(job,
			func() error { return client.Clone(src, name) },
			nil) // virt-clone doesn't expose progress via the libvirt job API
	case "backspace":
		m.cloneName = runeBackspace(m.cloneName)
		return m, nil
	default:
		// Accept the same character set as snapshot names — safe for
		// libvirt domain names. Plus underscore/hyphen/dot.
		s := msg.String()
		if len(s) == 1 && isValidDomainNameChar(s[0]) {
			m.cloneName += s
		}
		return m, nil
	}
}

// isValidDomainNameChar reports whether b is allowed in a libvirt
// domain name. Matches the same safe grammar as snapshot names.
func isValidDomainNameChar(b byte) bool {
	return isValidSnapshotChar(b)
}

// handleAttachKey runs the multi-stage attach / detach prompt: pick
// device type → provide device-specific params → execute. The verb
// ("attach" or "detach") was set by the trigger key (A or X).
func (m Model) handleAttachKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.attachStage {
	case 1:
		// Pick device type: d for disk, n for NIC.
		switch msg.String() {
		case "esc":
			m.attachStage = 0
			return m, nil
		case "d":
			m.attachType = "disk"
			m.attachStage = 2
			if m.attachVerb == "detach" {
				m.attachParam1 = "vdb" // target dev
			} else {
				m.attachParam1 = "/var/lib/libvirt/images/"
			}
			return m, nil
		case "n":
			m.attachType = "nic"
			m.attachStage = 2
			if m.attachVerb == "detach" {
				m.attachParam1 = "" // MAC — no sensible default
			} else {
				m.attachParam1 = "default"
			}
			return m, nil
		}
		return m, nil
	case 2:
		switch msg.String() {
		case "esc":
			m.attachStage = 0
			return m, nil
		case "enter":
			if m.attachVerb == "detach" {
				return m.executeDetach()
			}
			if m.attachType == "disk" {
				// Move to stage 3 to pick the target device name.
				m.attachStage = 3
				m.attachParam2 = "vdb"
				return m, nil
			}
			// NIC: execute now — only needs the network name.
			network := strings.TrimSpace(m.attachParam1)
			if network == "" {
				m.flashf("✗ network name required")
				return m, nil
			}
			domain := m.attachDomain
			client := m.client
			m.attachStage = 0
			return m, func() tea.Msg {
				err := client.AttachNIC(domain, network)
				return actionResultMsg{uri: client.URI(), action: "attach nic", name: domain, err: err}
			}
		case "backspace":
			m.attachParam1 = runeBackspace(m.attachParam1)
			return m, nil
		default:
			if len(msg.String()) == 1 {
				m.attachParam1 += msg.String()
			}
			return m, nil
		}
	case 3:
		// Disk: target device (vdb, vdc, …)
		switch msg.String() {
		case "esc":
			m.attachStage = 0
			return m, nil
		case "enter":
			path := strings.TrimSpace(m.attachParam1)
			target := strings.TrimSpace(m.attachParam2)
			if path == "" || target == "" {
				m.flashf("✗ both path and target required")
				return m, nil
			}
			domain := m.attachDomain
			client := m.client
			m.attachStage = 0
			return m, func() tea.Msg {
				err := client.AttachDisk(domain, path, target)
				return actionResultMsg{uri: client.URI(), action: "attach disk", name: domain, err: err}
			}
		case "backspace":
			m.attachParam2 = runeBackspace(m.attachParam2)
			return m, nil
		default:
			if len(msg.String()) == 1 {
				m.attachParam2 += msg.String()
			}
			return m, nil
		}
	}
	return m, nil
}

// executeDetach fires the libvirt detach call — disk by target
// device (vdb), NIC by MAC. Single-stage prompt is enough since
// either identifier is a single token.
func (m Model) executeDetach() (tea.Model, tea.Cmd) {
	param := strings.TrimSpace(m.attachParam1)
	if param == "" {
		if m.attachType == "disk" {
			m.flashf("✗ target device required (e.g. vdb)")
		} else {
			m.flashf("✗ MAC address required")
		}
		return m, nil
	}
	domain := m.attachDomain
	client := m.client
	devType := m.attachType
	m.attachStage = 0
	return m, func() tea.Msg {
		var err error
		if devType == "disk" {
			err = client.DetachDisk(domain, param)
		} else {
			err = client.DetachNIC(domain, param)
		}
		return actionResultMsg{uri: client.URI(), action: "detach " + devType, name: domain, err: err}
	}
}

// execCommand interprets a `:` command and switches view mode.
func (m Model) execCommand(cmd string) (Model, tea.Cmd) {
	switch cmd {
	case "":
		return m, nil
	case "q", "quit":
		return m, tea.Quit
	case "help":
		m.mode = viewHelp
		return m, nil
	case "vm":
		m.mode = viewMain
		return m, nil
	case "all", "fleet":
		return m.enterAllView()
	case "snap":
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
	case "net":
		m.mode = viewNetworks
		m.networksSel = 0
		m.networks = nil
		return m, loadNetworksCmd(m.client)
	case "pool":
		m.mode = viewPools
		m.poolsSel = 0
		m.pools = nil
		return m, loadPoolsCmd(m.client)
	case "host":
		m.mode = viewHosts
		m.hostsSel = 0
		return m, loadHostsListCmd(m.client.URI())
	case "perf":
		if _, ok := m.currentDomain(); !ok {
			m.flashf("no domain selected")
			return m, nil
		}
		m.mode = viewGraphs
		m.graphsDirty = true
		return m, nil
	case "jobs":
		m.mode = viewJobs
		return m, nil
	case "columns":
		m.mode = viewColumns
		m.columnsSel = 0
		return m, nil
	case "columns reset":
		// Restore default column visibility without touching the
		// config file. Master can :save afterwards if they want it
		// to stick.
		def := config.DefaultConfig().List.Columns
		m.activeColumns = filterActiveColumns(vmColumns, def)
		m.flashf("✓ columns reset to defaults")
		return m, nil
	case "config":
		// Open the config file in $EDITOR, suspending dirt for the
		// duration. On exit the file is re-read and runtime state
		// (theme, column visibility, sort) re-applied.
		return m, m.runConfigEdit()
	case "save", "w", "write":
		// Persist the runtime preferences (theme, sort, columns,
		// mark advance) to config.yaml so they survive a restart.
		// Hand-written comments WILL be lost — `:config` then
		// editing remains the way to keep them.
		return m.execSaveCommand()
	case "wq", "x":
		// Vim-style save-and-quit.
		newM, _ := m.execSaveCommand()
		return newM, tea.Quit
	case "mark", "mark all", "mark invert", "mark none", "unmark":
		return m.execMarkCommand(cmd), nil
	case "resume":
		// Bulk-resume every marked paused VM (or the cursor row when
		// no marks are set, via the regular `p` toggle path).
		if m.markCount() > 0 {
			names := m.markedDomainsInStates(lv.StatePaused)
			if len(names) == 0 {
				m.flashf("no marked VMs are paused")
				return m, nil
			}
			return m, bulkActionCmd(m.client, "resume", names, m.client.Resume)
		}
		if d, ok := m.currentDomain(); ok && d.State == lv.StatePaused {
			return m, actionCmd(m.client, "resume", d.Name, m.client.Resume)
		}
		m.flashf("no paused VM to resume")
		return m, nil
	}
	// Sort commands take the column id (and optional direction) as args.
	if strings.HasPrefix(cmd, "sort") {
		return m.execSortCommand(strings.TrimSpace(strings.TrimPrefix(cmd, "sort"))), nil
	}
	// Theme hot-swap: :theme <name>. Empty name flashes the list.
	if strings.HasPrefix(cmd, "theme") {
		return m.execThemeCommand(strings.TrimSpace(strings.TrimPrefix(cmd, "theme"))), nil
	}
	// Grouping: :group os|state|none.
	if strings.HasPrefix(cmd, "group") {
		return m.execGroupCommand(strings.TrimSpace(strings.TrimPrefix(cmd, "group"))), nil
	}
	// Export the current VM-list view: :export csv|json [path]
	if strings.HasPrefix(cmd, "export") {
		return m.execExportCommand(strings.TrimSpace(strings.TrimPrefix(cmd, "export"))), nil
	}
	// No exact match — try unique prefix before giving up.
	if match, ok := uniquePrefixMatch(cmd); ok {
		return m.execCommand(match)
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
		client := m.client
		job := &Job{
			ID:       fmt.Sprintf("snap-create-%s-%s-%d", domain, name, time.Now().UnixNano()),
			Kind:     "snap-create",
			Target:   domain,
			Detail:   "@ " + name,
			Phase:    "creating",
			Progress: -1,
		}
		return m, runDomainJob(job,
			func() error { return client.CreateSnapshot(domain, name, "") },
			snapshotProgressPoller(client, domain))
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
		client := m.client
		switch action {
		case "revert":
			job := &Job{
				ID:       fmt.Sprintf("snap-revert-%s-%s-%d", domain, name, time.Now().UnixNano()),
				Kind:     "snap-revert",
				Target:   domain,
				Detail:   "@ " + name,
				Phase:    "reverting",
				Progress: -1,
			}
			return m, runDomainJob(job,
				func() error { return client.RevertSnapshot(domain, name) },
				snapshotProgressPoller(client, domain))
		case "delete-snap":
			job := &Job{
				ID:       fmt.Sprintf("snap-delete-%s-%s-%d", domain, name, time.Now().UnixNano()),
				Kind:     "snap-delete",
				Target:   domain,
				Detail:   "@ " + name,
				Phase:    "deleting",
				Progress: -1,
			}
			return m, runDomainJob(job,
				func() error { return client.DeleteSnapshot(domain, name) },
				snapshotProgressPoller(client, domain))
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
	// Sub-states first.
	if m.volInputStage > 0 {
		return m.handleVolInputKey(msg)
	}
	if m.confirming {
		return m.handleConfirmKey(msg)
	}
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
	case "c":
		// Create a new volume in this pool.
		m.volInputStage = 1
		m.volInputName = ""
		m.volInputSize = "10G"
		return m, nil
	case "D":
		// Delete the selected volume.
		if v, ok := m.currentVolume(); ok {
			m.confirming = true
			m.confirmAction = "delete-vol"
			m.confirmName = v.Name
		}
		return m, nil
	case "R", "F5":
		return m, loadVolumesCmd(m.client, m.volumesFor)
	}
	return m, nil
}

// handleVolInputKey runs the two-stage name+size prompt for creating a
// new volume in the current pool.
func (m Model) handleVolInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.volInputStage = 0
		m.volInputName = ""
		m.volInputSize = ""
		return m, nil
	case "enter":
		switch m.volInputStage {
		case 1:
			name := strings.TrimSpace(m.volInputName)
			if name == "" {
				m.flashf("✗ name cannot be empty")
				return m, nil
			}
			m.volInputStage = 2
			return m, nil
		case 2:
			size, err := parseHumanSize(m.volInputSize)
			if err != nil {
				m.flashf("✗ %v", err)
				return m, nil
			}
			name := strings.TrimSpace(m.volInputName)
			pool := m.volumesFor
			client := m.client
			m.volInputStage = 0
			m.volInputName = ""
			m.volInputSize = ""
			job := &Job{
				ID:       fmt.Sprintf("vol-create-%s-%s-%d", pool, name, time.Now().UnixNano()),
				Kind:     "vol-create",
				Target:   name,
				Detail:   fmt.Sprintf("@ %s · %s", pool, formatBytes(float64(size))),
				Phase:    "allocating",
				Progress: -1,
			}
			return m, tea.Batch(
				runDomainJob(job,
					func() error { return client.CreateVolume(pool, name, size) },
					nil),
				loadVolumesCmd(client, pool),
			)
		}
	case "backspace":
		switch m.volInputStage {
		case 1:
			m.volInputName = runeBackspace(m.volInputName)
		case 2:
			m.volInputSize = runeBackspace(m.volInputSize)
		}
		return m, nil
	default:
		s := msg.String()
		if len(s) == 1 {
			switch m.volInputStage {
			case 1:
				m.volInputName += s
			case 2:
				m.volInputSize += s
			}
		}
		return m, nil
	}
	return m, nil
}

// parseHumanSize accepts "10G", "500M", "1.5T", "4096" (bytes) and
// returns the value in bytes.
func parseHumanSize(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("size cannot be empty")
	}
	mult := uint64(1)
	last := s[len(s)-1]
	switch last {
	case 'K', 'k':
		mult = 1024
		s = s[:len(s)-1]
	case 'M', 'm':
		mult = 1024 * 1024
		s = s[:len(s)-1]
	case 'G', 'g':
		mult = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	case 'T', 't':
		mult = 1024 * 1024 * 1024 * 1024
		s = s[:len(s)-1]
	}
	var num float64
	if _, err := fmt.Sscanf(s, "%f", &num); err != nil {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	if num <= 0 {
		return 0, fmt.Errorf("size must be positive")
	}
	return uint64(num * float64(mult)), nil
}

// currentNetwork returns the network at the visual cursor position,
// honouring the active sort. The cursor index is into the sorted
// view, not the raw m.networks slice.
func (m Model) currentNetwork() (lv.Network, bool) {
	sorted := m.sortedNetworks()
	if m.networksSel < 0 || m.networksSel >= len(sorted) {
		return lv.Network{}, false
	}
	return sorted[m.networksSel], true
}

func (m Model) currentPool() (lv.StoragePool, bool) {
	sorted := m.sortedPools()
	if m.poolsSel < 0 || m.poolsSel >= len(sorted) {
		return lv.StoragePool{}, false
	}
	return sorted[m.poolsSel], true
}

func (m Model) currentVolume() (lv.StorageVolume, bool) {
	sorted := m.sortedVolumes()
	if m.volumesSel < 0 || m.volumesSel >= len(sorted) {
		return lv.StorageVolume{}, false
	}
	return sorted[m.volumesSel], true
}

// networkActionCmd is a generic action runner used by network and pool keys.
func networkActionCmd(c backend.Backend, action, name string, fn func(string) error) tea.Cmd {
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
	if m.client == nil {
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
	if m.snap == nil || m.client == nil {
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
		m.snapshotInput || m.hostInputStage > 0 || m.cloneFrom ||
		m.volInputStage > 0 || m.attachStage > 0
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

// runConfigEdit suspends Bubble Tea and opens the config file in
// $EDITOR. On exit, fires a configReloadedMsg so the new file is
// re-read and the theme / column visibility / sort defaults
// re-apply without restarting dirt.
func (m Model) runConfigEdit() tea.Cmd {
	path := config.ConfigPath()
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return configEditedMsg{path: path, err: err}
	})
}

// configEditedMsg lands when the editor exits. Triggers a re-read of
// the config file and re-application of theme / sort / columns.
type configEditedMsg struct {
	path string
	err  error
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
