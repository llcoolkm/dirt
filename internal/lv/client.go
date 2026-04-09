// Package lv is a thin Go-friendly wrapper over libvirt.org/go/libvirt.
// It hides C-backed handle lifetimes by copying state into plain Go structs.
package lv

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
	"sync"
	"time"

	"libvirt.org/go/libvirt"
)

// Client is a libvirt connection.
type Client struct {
	conn *libvirt.Connect
	uri  string

	// osCache maps UUID -> short OS name. Populated lazily on first sight of a
	// domain since the OS doesn't change at runtime.
	osMu    sync.Mutex
	osCache map[string]string

	// statsPeriodSet records which domains have already had their balloon
	// stats push period configured. We do it once per dirt session.
	statsMu        sync.Mutex
	statsPeriodSet map[string]bool
}

// New opens a connection to the given libvirt URI. Empty defaults to qemu:///system.
func New(uri string) (*Client, error) {
	if uri == "" {
		uri = "qemu:///system"
	}
	conn, err := libvirt.NewConnect(uri)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", uri, err)
	}
	return &Client{
		conn:           conn,
		uri:            uri,
		osCache:        make(map[string]string),
		statsPeriodSet: make(map[string]bool),
	}, nil
}

// URI returns the libvirt URI in use.
func (c *Client) URI() string { return c.uri }

// Hostname returns the hypervisor hostname.
func (c *Client) Hostname() string {
	if c == nil || c.conn == nil {
		return ""
	}
	h, err := c.conn.GetHostname()
	if err != nil {
		return ""
	}
	return h
}

// Close releases the underlying libvirt connection.
func (c *Client) Close() {
	if c != nil && c.conn != nil {
		_, _ = c.conn.Close()
	}
}

// State is a small enum mirroring libvirt.DomainState.
type State int

const (
	StateNoState State = iota
	StateRunning
	StateBlocked
	StatePaused
	StateShutdown
	StateShutoff
	StateCrashed
	StatePMSuspended
)

// String returns a short human label.
func (s State) String() string {
	switch s {
	case StateRunning:
		return "running"
	case StateBlocked:
		return "blocked"
	case StatePaused:
		return "paused"
	case StateShutdown:
		return "shutdown"
	case StateShutoff:
		return "shut off"
	case StateCrashed:
		return "crashed"
	case StatePMSuspended:
		return "suspended"
	default:
		return "—"
	}
}

// Domain is a snapshot of a libvirt domain at a moment in time.
// All counter fields are cumulative; rates are computed in the UI by diffing.
type Domain struct {
	Name       string
	UUID       string
	ID         uint
	State      State
	OS         string // friendly OS label parsed from libosinfo metadata, e.g. "Ubuntu 24.04"
	IP         string // primary IPv4 (DHCP lease, ARP fallback, QGA if available)
	NrVCPU     uint
	MaxMemKB   uint64
	MemoryKB   uint64
	CPUTimeNs  uint64 // cumulative
	Persistent bool
	Autostart  bool
	SampledAt  time.Time

	// Balloon stats — only populated when the guest has a working balloon driver.
	BalloonCurrentKB    uint64 // current balloon size (≈ allocated to guest)
	BalloonAvailableKB  uint64 // total memory the guest can see
	BalloonUnusedKB     uint64 // free memory inside the guest
	BalloonDiskCachesKB uint64 // page cache + buffers inside the guest
	BalloonRssKB        uint64 // resident set size on the host
	BalloonSwapIn       uint64 // cumulative pages swapped in (guest)
	BalloonSwapOut      uint64 // cumulative pages swapped out (guest)

	// I/O counters, summed across all disks / interfaces. Cumulative bytes.
	BlockRdBytes uint64
	BlockWrBytes uint64
	NetRxBytes   uint64
	NetTxBytes   uint64
}

// Snapshot is one full sample of the host plus all its domains.
type Snapshot struct {
	Hostname  string
	URI       string
	SampledAt time.Time
	Domains   []Domain
}

// Snapshot returns a fresh sample of every defined domain (running and inactive)
// with state, CPU, balloon, block, and net stats populated in one batched call.
func (c *Client) Snapshot() (*Snapshot, error) {
	flags := libvirt.CONNECT_GET_ALL_DOMAINS_STATS_ACTIVE |
		libvirt.CONNECT_GET_ALL_DOMAINS_STATS_INACTIVE
	types := libvirt.DOMAIN_STATS_STATE |
		libvirt.DOMAIN_STATS_CPU_TOTAL |
		libvirt.DOMAIN_STATS_BALLOON |
		libvirt.DOMAIN_STATS_VCPU |
		libvirt.DOMAIN_STATS_BLOCK |
		libvirt.DOMAIN_STATS_INTERFACE

	stats, err := c.conn.GetAllDomainStats(nil, types, flags)
	if err != nil {
		return nil, fmt.Errorf("get domain stats: %w", err)
	}
	// Free the C-backed domain handles inside each DomainStats once we're done.
	defer func() {
		for i := range stats {
			if stats[i].Domain != nil {
				_ = stats[i].Domain.Free()
			}
		}
	}()

	now := time.Now()
	snap := &Snapshot{
		Hostname:  c.Hostname(),
		URI:       c.uri,
		SampledAt: now,
	}

	for i := range stats {
		s := &stats[i]
		if s.Domain == nil {
			continue
		}
		d := s.Domain

		name, _ := d.GetName()
		uuid, _ := d.GetUUIDString()
		id, _ := d.GetID()
		persistent, _ := d.IsPersistent()
		autostart, _ := d.GetAutostart()

		dom := Domain{
			Name:       name,
			UUID:       uuid,
			ID:         id,
			Persistent: persistent,
			Autostart:  autostart,
			SampledAt:  now,
		}

		// State and basic info — DOMAIN_STATS_STATE returns it, but GetInfo
		// is also reliable and gives us NrVirtCpu / Memory in one go.
		if info, err := d.GetInfo(); err == nil {
			dom.State = State(info.State)
			dom.NrVCPU = info.NrVirtCpu
			dom.MaxMemKB = info.MaxMem
			dom.MemoryKB = info.Memory
			dom.CPUTimeNs = info.CpuTime
		}

		// Override CPU time from stats if present (more accurate timing).
		if s.Cpu != nil && s.Cpu.TimeSet {
			dom.CPUTimeNs = s.Cpu.Time
		}

		if s.Balloon != nil {
			if s.Balloon.CurrentSet {
				dom.BalloonCurrentKB = s.Balloon.Current
			}
			if s.Balloon.AvailableSet {
				dom.BalloonAvailableKB = s.Balloon.Available
			}
			if s.Balloon.UnusedSet {
				dom.BalloonUnusedKB = s.Balloon.Unused
			}
			if s.Balloon.DiskCachesSet {
				dom.BalloonDiskCachesKB = s.Balloon.DiskCaches
			}
			if s.Balloon.RssSet {
				dom.BalloonRssKB = s.Balloon.Rss
			}
			if s.Balloon.SwapInSet {
				dom.BalloonSwapIn = s.Balloon.SwapIn
			}
			if s.Balloon.SwapOutSet {
				dom.BalloonSwapOut = s.Balloon.SwapOut
			}
		}

		// OS label — cached on first sight, since it doesn't change at runtime.
		dom.OS = c.osFor(d, dom.UUID)

		// Make sure the guest balloon driver pushes memory stats on a regular
		// cadence. By default the QEMU balloon stat period is 0 ("on demand")
		// which makes the numbers stale. We set it once per session.
		// Also probe for the guest's primary IPv4 address.
		if dom.State == StateRunning {
			c.ensureStatsPeriod(d, dom.UUID)
			dom.IP = primaryIPv4(d)
		}

		// Sum block stats across all disks.
		for _, bs := range s.Block {
			if bs.RdBytesSet {
				dom.BlockRdBytes += bs.RdBytes
			}
			if bs.WrBytesSet {
				dom.BlockWrBytes += bs.WrBytes
			}
		}
		// Sum net stats across all interfaces.
		for _, ns := range s.Net {
			if ns.RxBytesSet {
				dom.NetRxBytes += ns.RxBytes
			}
			if ns.TxBytesSet {
				dom.NetTxBytes += ns.TxBytes
			}
		}

		snap.Domains = append(snap.Domains, dom)
	}
	return snap, nil
}

// withDomain looks a domain up by name, runs fn, and frees the handle.
func (c *Client) withDomain(name string, fn func(*libvirt.Domain) error) error {
	d, err := c.conn.LookupDomainByName(name)
	if err != nil {
		return err
	}
	defer func() { _ = d.Free() }()
	return fn(d)
}

// Lifecycle actions.
func (c *Client) Start(name string) error {
	return c.withDomain(name, func(d *libvirt.Domain) error { return d.Create() })
}
func (c *Client) Shutdown(name string) error {
	return c.withDomain(name, func(d *libvirt.Domain) error { return d.Shutdown() })
}
func (c *Client) Destroy(name string) error {
	return c.withDomain(name, func(d *libvirt.Domain) error { return d.Destroy() })
}
func (c *Client) Reboot(name string) error {
	return c.withDomain(name, func(d *libvirt.Domain) error { return d.Reboot(0) })
}
func (c *Client) Suspend(name string) error {
	return c.withDomain(name, func(d *libvirt.Domain) error { return d.Suspend() })
}
func (c *Client) Resume(name string) error {
	return c.withDomain(name, func(d *libvirt.Domain) error { return d.Resume() })
}

// SwapInfo describes the swap state of a guest, queried via qemu-guest-agent.
type SwapInfo struct {
	Available  bool      // true only if QGA was reachable and parsing succeeded
	HasSwap    bool      // true if the guest actually has swap configured (TotalKB > 0)
	TotalKB    uint64
	FreeKB     uint64
	UsedKB     uint64
	FetchedAt  time.Time
	Err        error
}

// Swap queries the qemu-guest-agent inside the named domain for /proc/meminfo,
// parses SwapTotal and SwapFree, and returns the result. Returns Available=false
// (with Err set) if QGA is not installed, not connected, or times out.
func (c *Client) Swap(name string) SwapInfo {
	info := SwapInfo{FetchedAt: time.Now()}
	err := c.withDomain(name, func(d *libvirt.Domain) error {
		// guest-exec /usr/bin/cat /proc/meminfo
		startCmd := `{"execute":"guest-exec","arguments":{"path":"/usr/bin/cat","arg":["/proc/meminfo"],"capture-output":true}}`
		resp, err := d.QemuAgentCommand(startCmd, 2, 0)
		if err != nil {
			return fmt.Errorf("guest-exec: %w", err)
		}
		var startResp struct {
			Return struct {
				PID int `json:"pid"`
			} `json:"return"`
		}
		if err := json.Unmarshal([]byte(resp), &startResp); err != nil {
			return fmt.Errorf("decode guest-exec: %w", err)
		}
		pid := startResp.Return.PID

		// Poll guest-exec-status briefly until the cat finishes.
		statusCmd := fmt.Sprintf(`{"execute":"guest-exec-status","arguments":{"pid":%d}}`, pid)
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			sresp, err := d.QemuAgentCommand(statusCmd, 2, 0)
			if err != nil {
				return fmt.Errorf("guest-exec-status: %w", err)
			}
			var st struct {
				Return struct {
					Exited   bool   `json:"exited"`
					ExitCode int    `json:"exitcode"`
					OutData  string `json:"out-data"`
					ErrData  string `json:"err-data"`
				} `json:"return"`
			}
			if err := json.Unmarshal([]byte(sresp), &st); err != nil {
				return fmt.Errorf("decode guest-exec-status: %w", err)
			}
			if !st.Return.Exited {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			if st.Return.ExitCode != 0 {
				return fmt.Errorf("cat /proc/meminfo exit %d", st.Return.ExitCode)
			}
			data, err := base64.StdEncoding.DecodeString(st.Return.OutData)
			if err != nil {
				return fmt.Errorf("decode out-data: %w", err)
			}
			total, free := parseMeminfoSwap(string(data))
			info.TotalKB = total
			info.FreeKB = free
			if total >= free {
				info.UsedKB = total - free
			}
			info.HasSwap = total > 0
			info.Available = true
			return nil
		}
		return fmt.Errorf("guest-exec timed out")
	})
	if err != nil {
		info.Err = err
	}
	return info
}

// parseMeminfoSwap pulls SwapTotal and SwapFree out of /proc/meminfo content.
func parseMeminfoSwap(s string) (totalKB, freeKB uint64) {
	for _, line := range strings.Split(s, "\n") {
		switch {
		case strings.HasPrefix(line, "SwapTotal:"):
			fmt.Sscanf(line, "SwapTotal: %d", &totalKB)
		case strings.HasPrefix(line, "SwapFree:"):
			fmt.Sscanf(line, "SwapFree: %d", &freeKB)
		}
	}
	return
}

// XMLDesc returns the live XML description of the domain.
func (c *Client) XMLDesc(name string) (string, error) {
	var out string
	err := c.withDomain(name, func(d *libvirt.Domain) error {
		s, err := d.GetXMLDesc(0)
		if err != nil {
			return err
		}
		out = s
		return nil
	})
	return out, err
}

// primaryIPv4 returns the first non-loopback IPv4 address of the domain.
// Tries the DHCP-lease source first (cheap, libvirt's own dnsmasq), then ARP
// (host's neighbour table), and finally the qemu-guest-agent if reachable.
// Returns "" if no source yields anything.
func primaryIPv4(d *libvirt.Domain) string {
	sources := []libvirt.DomainInterfaceAddressesSource{
		libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_LEASE,
		libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_ARP,
		libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_AGENT,
	}
	for _, src := range sources {
		ifaces, err := d.ListAllInterfaceAddresses(src)
		if err != nil {
			continue
		}
		for _, ifc := range ifaces {
			for _, a := range ifc.Addrs {
				if libvirt.IPAddrType(a.Type) != libvirt.IP_ADDR_TYPE_IPV4 {
					continue
				}
				if a.Addr == "" || strings.HasPrefix(a.Addr, "127.") {
					continue
				}
				return a.Addr
			}
		}
	}
	return ""
}

// ensureStatsPeriod sets the balloon-driver stats push interval to 2 seconds
// for the given running domain, once per dirt session. Without this, MemoryStats
// values can be stale or zero on guests that don't poll on their own.
func (c *Client) ensureStatsPeriod(d *libvirt.Domain, uuid string) {
	c.statsMu.Lock()
	if c.statsPeriodSet[uuid] {
		c.statsMu.Unlock()
		return
	}
	c.statsPeriodSet[uuid] = true
	c.statsMu.Unlock()
	// Best-effort — older qemu / non-balloon guests will return ENOSUPP.
	_ = d.SetMemoryStatsPeriod(2, libvirt.DOMAIN_MEM_LIVE)
}

// osFor returns a friendly OS label for a domain, caching the result.
// Looks for libosinfo metadata in the live XML.
func (c *Client) osFor(d *libvirt.Domain, uuid string) string {
	c.osMu.Lock()
	if v, ok := c.osCache[uuid]; ok {
		c.osMu.Unlock()
		return v
	}
	c.osMu.Unlock()

	x, err := d.GetXMLDesc(0)
	if err != nil {
		return ""
	}
	label := parseOSFromXML(x)

	c.osMu.Lock()
	c.osCache[uuid] = label
	c.osMu.Unlock()
	return label
}

// parseOSFromXML extracts a short OS label from a domain's libosinfo metadata.
// Returns "" if the metadata is missing or unparseable.
func parseOSFromXML(x string) string {
	type osTag struct {
		ID string `xml:"id,attr"`
	}
	type libosinfoTag struct {
		OS osTag `xml:"os"`
	}
	type metadataTag struct {
		Libosinfo libosinfoTag `xml:"libosinfo"`
	}
	type domainTag struct {
		Metadata metadataTag `xml:"metadata"`
	}
	var d domainTag
	if err := xml.Unmarshal([]byte(x), &d); err != nil {
		return ""
	}
	return prettyOSFromURL(d.Metadata.Libosinfo.OS.ID)
}

// prettyOSFromURL turns "http://ubuntu.com/ubuntu/24.04" → "Ubuntu 24.04",
// "http://archlinux.org/archlinux/rolling" → "Arch Linux", etc.
func prettyOSFromURL(id string) string {
	if id == "" {
		return ""
	}
	id = strings.TrimPrefix(id, "http://")
	id = strings.TrimPrefix(id, "https://")
	parts := strings.Split(id, "/")
	if len(parts) < 3 {
		return id
	}
	distro := parts[1]
	ver := parts[2]

	names := map[string]string{
		"ubuntu":      "Ubuntu",
		"debian":      "Debian",
		"fedora":      "Fedora",
		"rhel":        "RHEL",
		"centos":      "CentOS",
		"rocky":       "Rocky",
		"alma":        "Alma",
		"archlinux":   "Arch",
		"alpinelinux": "Alpine",
		"opensuse":    "openSUSE",
		"win":         "Windows",
		"freebsd":     "FreeBSD",
		"openbsd":     "OpenBSD",
		"netbsd":      "NetBSD",
		"macosx":      "macOS",
	}
	name := names[distro]
	if name == "" {
		// Capitalize first letter as a sane default.
		if len(distro) > 0 {
			name = strings.ToUpper(distro[:1]) + distro[1:]
		}
	}
	if ver == "rolling" {
		return name
	}
	return name + " " + ver
}
