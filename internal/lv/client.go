// Package lv is a thin Go-friendly wrapper over libvirt.org/go/libvirt.
// It hides C-backed handle lifetimes by copying state into plain Go structs.
package lv

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"strconv"
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
	NrVCPU      uint
	MaxMemKB    uint64
	MemoryKB    uint64
	CPUTimeNs   uint64   // cumulative
	CPUUserNs   uint64   // cumulative user-space CPU time
	CPUSystemNs uint64   // cumulative kernel CPU time
	VCPUTimes   []uint64 // cumulative CPU time per vCPU (ns)
	BootedAt   time.Time // qemu process start time (zero for remote URIs)
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
	BalloonMajorFault   uint64 // cumulative major page faults (memory pressure)
	BalloonMinorFault   uint64

	// I/O counters, summed across all disks / interfaces. Cumulative.
	BlockRdBytes uint64
	BlockWrBytes uint64
	BlockRdReqs  uint64 // cumulative read requests (for IOPS)
	BlockWrReqs  uint64
	BlockRdTimes uint64 // cumulative read time (ns) — for latency
	BlockWrTimes uint64

	// Disk inventory — populated via virDomainGetBlockInfo for every disk.
	NumDisks                 int
	TotalDiskCapacityBytes   uint64 // sum of virtual sizes (what the guest sees)
	TotalDiskAllocationBytes uint64 // sum of actual on-host disk usage (sparse-aware)
	NetRxBytes   uint64
	NetTxBytes   uint64
	NetRxPkts    uint64
	NetTxPkts    uint64
	NetRxErrs    uint64
	NetRxDrop    uint64
	NetTxErrs    uint64
	NetTxDrop    uint64
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
		if s.Cpu != nil {
			if s.Cpu.TimeSet {
				dom.CPUTimeNs = s.Cpu.Time
			}
			if s.Cpu.UserSet {
				dom.CPUUserNs = s.Cpu.User
			}
			if s.Cpu.SystemSet {
				dom.CPUSystemNs = s.Cpu.System
			}
		}

		// Per-vCPU CPU times — one cumulative nanosecond counter per vCPU.
		if len(s.Vcpu) > 0 {
			dom.VCPUTimes = make([]uint64, len(s.Vcpu))
			for i, v := range s.Vcpu {
				if v.TimeSet {
					dom.VCPUTimes[i] = v.Time
				}
			}
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
			if s.Balloon.MajorFaultSet {
				dom.BalloonMajorFault = s.Balloon.MajorFault
			}
			if s.Balloon.MinorFaultSet {
				dom.BalloonMinorFault = s.Balloon.MinorFault
			}
		}

		// OS label — cached on first sight, since it doesn't change at runtime.
		dom.OS = c.osFor(d, dom.UUID)

		// Make sure the guest balloon driver pushes memory stats on a regular
		// cadence. By default the QEMU balloon stat period is 0 ("on demand")
		// which makes the numbers stale. We set it once per session.
		// Also probe for the guest's primary IPv4 address and qemu process
		// start time (the latter is local-only).
		if dom.State == StateRunning {
			c.ensureStatsPeriod(d, dom.UUID)
			dom.IP = primaryIPv4(d)
			dom.BootedAt = c.bootedAt(name)
		}

		// Disk inventory is available for both running and stopped domains —
		// libvirt reads the qcow2 header directly when no qemu has the file.
		dom.NumDisks, dom.TotalDiskCapacityBytes, dom.TotalDiskAllocationBytes = diskInventory(d)

		// Sum block stats across all disks.
		for _, bs := range s.Block {
			if bs.RdBytesSet {
				dom.BlockRdBytes += bs.RdBytes
			}
			if bs.WrBytesSet {
				dom.BlockWrBytes += bs.WrBytes
			}
			if bs.RdReqsSet {
				dom.BlockRdReqs += bs.RdReqs
			}
			if bs.WrReqsSet {
				dom.BlockWrReqs += bs.WrReqs
			}
			if bs.RdTimesSet {
				dom.BlockRdTimes += bs.RdTimes
			}
			if bs.WrTimesSet {
				dom.BlockWrTimes += bs.WrTimes
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
			if ns.RxPktsSet {
				dom.NetRxPkts += ns.RxPkts
			}
			if ns.TxPktsSet {
				dom.NetTxPkts += ns.TxPkts
			}
			if ns.RxErrsSet {
				dom.NetRxErrs += ns.RxErrs
			}
			if ns.RxDropSet {
				dom.NetRxDrop += ns.RxDrop
			}
			if ns.TxErrsSet {
				dom.NetTxErrs += ns.TxErrs
			}
			if ns.TxDropSet {
				dom.NetTxDrop += ns.TxDrop
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

// Undefine removes a defined (stopped) domain from libvirt. Snapshots and
// managed-save state are removed too. Will fail if the domain is running.
func (c *Client) Undefine(name string) error {
	return c.withDomain(name, func(d *libvirt.Domain) error {
		flags := libvirt.DOMAIN_UNDEFINE_SNAPSHOTS_METADATA |
			libvirt.DOMAIN_UNDEFINE_MANAGED_SAVE |
			libvirt.DOMAIN_UNDEFINE_NVRAM
		return d.UndefineFlags(flags)
	})
}

// HostInfo holds a small subset of libvirt's NodeInfo plus a name.
type HostInfo struct {
	Hostname       string
	CPUModel       string // CPU brand string, e.g. "AMD Ryzen AI 9 HX 370"
	CPUs           uint
	CoresPerSocket uint32
	Sockets        uint32
	Threads        uint32
	MemoryKB       uint64
	OSPretty       string // PRETTY_NAME from /etc/os-release, e.g. "Ubuntu 25.04"
}

// Host returns basic host node information from libvirt, supplemented by
// /proc/cpuinfo (real CPU brand string) and /etc/os-release (OS pretty name)
// when libvirt is local.
func (c *Client) Host() (HostInfo, error) {
	if c == nil || c.conn == nil {
		return HostInfo{}, fmt.Errorf("nil client")
	}
	ni, err := c.conn.GetNodeInfo()
	if err != nil {
		return HostInfo{}, err
	}
	h := HostInfo{
		Hostname:       c.Hostname(),
		CPUModel:       ni.Model, // libvirt usually returns the arch ("x86_64")
		CPUs:           ni.Cpus,
		CoresPerSocket: ni.Cores,
		Sockets:        ni.Sockets,
		Threads:        ni.Threads,
		MemoryKB:       ni.Memory,
	}
	// Override the CPU brand and add the OS label from /proc and /etc when
	// libvirt is on the same host as dirt. For remote URIs we leave the
	// libvirt-provided values in place.
	if strings.HasPrefix(c.uri, "qemu:///") {
		if model := readCPUModelName(); model != "" {
			h.CPUModel = model
		}
		h.OSPretty = readOSPrettyName()
	}
	return h, nil
}

// GuestUptime is a guest-side uptime sample, fetched via qemu-guest-agent.
// Available is true only if the QGA call succeeded; otherwise Err is set.
type GuestUptime struct {
	Available bool
	BootedAt  time.Time // computed: FetchedAt - reported uptime
	Uptime    time.Duration
	FetchedAt time.Time
	Err       error
}

// QueryGuestUptime asks the qemu-guest-agent in the named domain for its
// guest-side uptime by running `cat /proc/uptime` via guest-exec. Returns
// Available=false (and a non-nil Err) when QGA is not installed or fails.
//
// This is the only way to get *guest* uptime (which resets on a guest reboot
// even when the qemu process keeps running). The qemu process start time
// from /proc on the host does not capture in-VM reboots.
func (c *Client) QueryGuestUptime(name string) GuestUptime {
	info := GuestUptime{FetchedAt: time.Now()}
	err := c.withDomain(name, func(d *libvirt.Domain) error {
		startCmd := `{"execute":"guest-exec","arguments":{"path":"/usr/bin/cat","arg":["/proc/uptime"],"capture-output":true}}`
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
				} `json:"return"`
			}
			if err := json.Unmarshal([]byte(sresp), &st); err != nil {
				return fmt.Errorf("decode status: %w", err)
			}
			if !st.Return.Exited {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			if st.Return.ExitCode != 0 {
				return fmt.Errorf("cat /proc/uptime exit %d", st.Return.ExitCode)
			}
			data, err := base64.StdEncoding.DecodeString(st.Return.OutData)
			if err != nil {
				return fmt.Errorf("decode out-data: %w", err)
			}
			// /proc/uptime is "<seconds_since_boot> <idle_seconds>".
			fields := strings.Fields(string(data))
			if len(fields) < 1 {
				return fmt.Errorf("unexpected /proc/uptime")
			}
			sec, err := strconv.ParseFloat(fields[0], 64)
			if err != nil {
				return fmt.Errorf("parse uptime: %w", err)
			}
			info.Uptime = time.Duration(sec * float64(time.Second))
			info.BootedAt = info.FetchedAt.Add(-info.Uptime)
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

// DomainSnapshot represents one libvirt domain snapshot, copied out of the
// C handle. (Distinct from Snapshot, which is a sample of the whole host.)
type DomainSnapshot struct {
	Name      string
	Parent    string
	State     string // running, shutoff, paused, …
	CreatedAt time.Time
	IsCurrent bool
	Desc      string

	// SizeBytes is the saved VM-state size summed across all disks. For
	// snapshots taken while running this is dominated by the saved RAM; for
	// snapshots of stopped domains it is typically zero. Returned by
	// `qemu-img info` and may be 0 if qemu-img wasn't reachable.
	SizeBytes int64
}

// snapshotXML is the minimal XML schema we parse out of GetXMLDesc.
type snapshotXML struct {
	Name         string `xml:"name"`
	Description  string `xml:"description"`
	State        string `xml:"state"`
	CreationTime int64  `xml:"creationTime"`
	Parent       struct {
		Name string `xml:"name"`
	} `xml:"parent"`
}

// ListSnapshots returns all snapshots of the named domain. Sizes are
// populated via qemu-img on the host (best effort — failures leave
// SizeBytes at 0).
func (c *Client) ListSnapshots(name string) ([]DomainSnapshot, error) {
	var out []DomainSnapshot
	var diskPaths []string
	err := c.withDomain(name, func(d *libvirt.Domain) error {
		// Pull disk file paths from the domain XML so we can ask qemu-img
		// for snapshot sizes after we list them.
		if x, err := d.GetXMLDesc(0); err == nil {
			diskPaths = parseDiskPaths(x)
		}

		snaps, err := d.ListAllSnapshots(0)
		if err != nil {
			return err
		}
		// Determine which snapshot is current.
		curName := ""
		if cur, err := d.SnapshotCurrent(0); err == nil {
			if n, e := cur.GetName(); e == nil {
				curName = n
			}
			_ = cur.Free()
		}
		for i := range snaps {
			s := &snaps[i]
			x, err := s.GetXMLDesc(0)
			if err != nil {
				_ = s.Free()
				continue
			}
			var sx snapshotXML
			_ = xml.Unmarshal([]byte(x), &sx)
			out = append(out, DomainSnapshot{
				Name:      sx.Name,
				Parent:    sx.Parent.Name,
				State:     sx.State,
				CreatedAt: time.Unix(sx.CreationTime, 0),
				IsCurrent: sx.Name == curName,
				Desc:      sx.Description,
			})
			_ = s.Free()
		}
		return nil
	})
	if err != nil {
		return out, err
	}

	// Populate sizes via qemu-img info — runs once per disk, host-side.
	if len(out) > 0 && len(diskPaths) > 0 {
		sizes := snapshotSizesFromDisks(diskPaths)
		for i := range out {
			out[i].SizeBytes = sizes[out[i].Name]
		}
	}
	return out, nil
}

// parseDiskPaths returns the on-host file paths of every actual disk
// (device='disk') in the given domain XML. CD-ROM and floppy devices are
// deliberately skipped — cloud-init ISOs and similar bootstrap media should
// not count as data disks.
func parseDiskPaths(x string) []string {
	type source struct {
		File string `xml:"file,attr"`
	}
	type disk struct {
		Type   string `xml:"type,attr"`
		Device string `xml:"device,attr"`
		Source source `xml:"source"`
	}
	type devices struct {
		Disks []disk `xml:"disk"`
	}
	type domain struct {
		Devices devices `xml:"devices"`
	}
	var d domain
	if err := xml.Unmarshal([]byte(x), &d); err != nil {
		return nil
	}
	var paths []string
	for _, dk := range d.Devices.Disks {
		// device defaults to "disk" when omitted; treat empty as disk too.
		if dk.Device != "" && dk.Device != "disk" {
			continue
		}
		if dk.Type == "file" && dk.Source.File != "" {
			paths = append(paths, dk.Source.File)
		}
	}
	return paths
}

// snapshotSizesFromDisks runs `qemu-img info -U --output=json` on each disk
// path and returns a map of snapshot name → total VM-state-size summed across
// all disks. The -U flag (force-share) is required because libvirt holds a
// write lock on the qcow2 file while the VM is running.
//
// Returns an empty map if qemu-img isn't reachable or no snapshots are found.
func snapshotSizesFromDisks(paths []string) map[string]int64 {
	sizes := make(map[string]int64)
	for _, p := range paths {
		out, err := exec.Command("qemu-img", "info", "-U", "--output=json", p).Output()
		if err != nil {
			continue
		}
		var info struct {
			Snapshots []struct {
				Name        string `json:"name"`
				VMStateSize int64  `json:"vm-state-size"`
			} `json:"snapshots"`
		}
		if err := json.Unmarshal(out, &info); err != nil {
			continue
		}
		for _, s := range info.Snapshots {
			sizes[s.Name] += s.VMStateSize
		}
	}
	return sizes
}

// CreateSnapshot creates a new snapshot on the named domain. If snapName is
// empty, libvirt assigns a timestamp-based default name.
func (c *Client) CreateSnapshot(domain, snapName, description string) error {
	return c.withDomain(domain, func(d *libvirt.Domain) error {
		x := "<domainsnapshot>"
		if snapName != "" {
			x += "<name>" + xmlEscape(snapName) + "</name>"
		}
		if description != "" {
			x += "<description>" + xmlEscape(description) + "</description>"
		}
		x += "</domainsnapshot>"
		s, err := d.CreateSnapshotXML(x, 0)
		if err != nil {
			return err
		}
		_ = s.Free()
		return nil
	})
}

// RevertSnapshot reverts the named domain to the named snapshot.
func (c *Client) RevertSnapshot(domain, snapName string) error {
	return c.withDomain(domain, func(d *libvirt.Domain) error {
		s, err := d.SnapshotLookupByName(snapName, 0)
		if err != nil {
			return err
		}
		defer func() { _ = s.Free() }()
		return s.RevertToSnapshot(0)
	})
}

// DeleteSnapshot removes the named snapshot.
func (c *Client) DeleteSnapshot(domain, snapName string) error {
	return c.withDomain(domain, func(d *libvirt.Domain) error {
		s, err := d.SnapshotLookupByName(snapName, 0)
		if err != nil {
			return err
		}
		defer func() { _ = s.Free() }()
		return s.Delete(0)
	})
}

// xmlEscape is a tiny escape helper for the snapshot XML payload.
func xmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}

// ──────────────────────────── Networks ───────────────────────────────────────

// Network is a snapshot of a libvirt network at a moment in time.
type Network struct {
	Name       string
	UUID       string
	Active     bool
	Persistent bool
	Autostart  bool
	Bridge     string
	Forward    string // nat / route / bridge / open / none
	NumLeases  int    // count of active DHCP leases (0 if not queryable)
}

// networkXML is the minimal XML schema we parse out of GetXMLDesc.
type networkXML struct {
	Forward struct {
		Mode string `xml:"mode,attr"`
	} `xml:"forward"`
}

// ListNetworks returns all defined networks (active + inactive).
func (c *Client) ListNetworks() ([]Network, error) {
	nets, err := c.conn.ListAllNetworks(0)
	if err != nil {
		return nil, fmt.Errorf("list networks: %w", err)
	}
	out := make([]Network, 0, len(nets))
	for i := range nets {
		n := &nets[i]
		name, _ := n.GetName()
		uuid, _ := n.GetUUIDString()
		active, _ := n.IsActive()
		persistent, _ := n.IsPersistent()
		autostart, _ := n.GetAutostart()
		bridge, _ := n.GetBridgeName()

		var nx networkXML
		if x, err := n.GetXMLDesc(0); err == nil {
			_ = xml.Unmarshal([]byte(x), &nx)
		}

		// DHCP lease count — only meaningful for active networks with DHCP.
		leaseCount := 0
		if active {
			if leases, err := n.GetDHCPLeases(); err == nil {
				leaseCount = len(leases)
			}
		}

		out = append(out, Network{
			Name:       name,
			UUID:       uuid,
			Active:     active,
			Persistent: persistent,
			Autostart:  autostart,
			Bridge:     bridge,
			Forward:    nx.Forward.Mode,
			NumLeases:  leaseCount,
		})
		_ = n.Free()
	}
	return out, nil
}

func (c *Client) withNetwork(name string, fn func(*libvirt.Network) error) error {
	n, err := c.conn.LookupNetworkByName(name)
	if err != nil {
		return err
	}
	defer func() { _ = n.Free() }()
	return fn(n)
}

// StartNetwork starts (activates) a network.
func (c *Client) StartNetwork(name string) error {
	return c.withNetwork(name, func(n *libvirt.Network) error { return n.Create() })
}

// StopNetwork deactivates a network.
func (c *Client) StopNetwork(name string) error {
	return c.withNetwork(name, func(n *libvirt.Network) error { return n.Destroy() })
}

// ToggleNetworkAutostart flips the autostart flag.
func (c *Client) ToggleNetworkAutostart(name string) error {
	return c.withNetwork(name, func(n *libvirt.Network) error {
		cur, err := n.GetAutostart()
		if err != nil {
			return err
		}
		return n.SetAutostart(!cur)
	})
}

// ──────────────────────────── Storage pools ──────────────────────────────────

// StoragePool represents one libvirt storage pool, copied out of C handles.
type StoragePool struct {
	Name       string
	UUID       string
	State      string // running / inactive / building / degraded / inaccessible
	Type       string // dir, lvm, nfs, …
	Persistent bool
	Autostart  bool
	Capacity   uint64 // bytes
	Allocation uint64
	Available  uint64
}

// poolXML extracts the pool type from its XML.
type poolXML struct {
	Type string `xml:"type,attr"`
}

// ListStoragePools returns all defined storage pools.
func (c *Client) ListStoragePools() ([]StoragePool, error) {
	pools, err := c.conn.ListAllStoragePools(0)
	if err != nil {
		return nil, fmt.Errorf("list pools: %w", err)
	}
	out := make([]StoragePool, 0, len(pools))
	for i := range pools {
		p := &pools[i]
		name, _ := p.GetName()
		uuid, _ := p.GetUUIDString()
		persistent, _ := p.IsPersistent()
		autostart, _ := p.GetAutostart()

		var info *libvirt.StoragePoolInfo
		info, _ = p.GetInfo()

		var px poolXML
		if x, err := p.GetXMLDesc(0); err == nil {
			_ = xml.Unmarshal([]byte(x), &px)
		}

		pool := StoragePool{
			Name:       name,
			UUID:       uuid,
			Persistent: persistent,
			Autostart:  autostart,
			Type:       px.Type,
		}
		if info != nil {
			pool.State = poolStateString(info.State)
			pool.Capacity = info.Capacity
			pool.Allocation = info.Allocation
			pool.Available = info.Available
		}
		out = append(out, pool)
		_ = p.Free()
	}
	return out, nil
}

func poolStateString(s libvirt.StoragePoolState) string {
	switch s {
	case libvirt.STORAGE_POOL_INACTIVE:
		return "inactive"
	case libvirt.STORAGE_POOL_BUILDING:
		return "building"
	case libvirt.STORAGE_POOL_RUNNING:
		return "running"
	case libvirt.STORAGE_POOL_DEGRADED:
		return "degraded"
	case libvirt.STORAGE_POOL_INACCESSIBLE:
		return "inaccessible"
	}
	return "unknown"
}

func (c *Client) withPool(name string, fn func(*libvirt.StoragePool) error) error {
	p, err := c.conn.LookupStoragePoolByName(name)
	if err != nil {
		return err
	}
	defer func() { _ = p.Free() }()
	return fn(p)
}

// StartPool starts (activates) a storage pool.
func (c *Client) StartPool(name string) error {
	return c.withPool(name, func(p *libvirt.StoragePool) error { return p.Create(0) })
}

// StopPool deactivates a storage pool.
func (c *Client) StopPool(name string) error {
	return c.withPool(name, func(p *libvirt.StoragePool) error { return p.Destroy() })
}

// ──────────────────────────── Storage volumes ────────────────────────────────

// StorageVolume is one volume inside a pool.
type StorageVolume struct {
	Name       string
	Path       string
	Type       string
	Capacity   uint64
	Allocation uint64
}

// ListVolumes returns all volumes inside the named pool.
func (c *Client) ListVolumes(poolName string) ([]StorageVolume, error) {
	var out []StorageVolume
	err := c.withPool(poolName, func(p *libvirt.StoragePool) error {
		vols, err := p.ListAllStorageVolumes(0)
		if err != nil {
			return err
		}
		for i := range vols {
			v := &vols[i]
			name, _ := v.GetName()
			path, _ := v.GetPath()
			info, _ := v.GetInfo()
			vol := StorageVolume{
				Name: name,
				Path: path,
			}
			if info != nil {
				vol.Type = volTypeString(info.Type)
				vol.Capacity = info.Capacity
				vol.Allocation = info.Allocation
			}
			out = append(out, vol)
			_ = v.Free()
		}
		return nil
	})
	return out, err
}

func volTypeString(t libvirt.StorageVolType) string {
	switch t {
	case libvirt.STORAGE_VOL_FILE:
		return "file"
	case libvirt.STORAGE_VOL_BLOCK:
		return "block"
	case libvirt.STORAGE_VOL_DIR:
		return "dir"
	case libvirt.STORAGE_VOL_NETWORK:
		return "network"
	case libvirt.STORAGE_VOL_NETDIR:
		return "netdir"
	case libvirt.STORAGE_VOL_PLOOP:
		return "ploop"
	}
	return "unknown"
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

// diskInventory enumerates the running domain's file-backed disks via the
// libvirt domain XML and queries each one for capacity + allocation. Returns
// (numDisks, totalCapacityBytes, totalAllocationBytes). Any field may be 0
// if libvirt declines (e.g. on non-running domains for some backends).
func diskInventory(d *libvirt.Domain) (int, uint64, uint64) {
	x, err := d.GetXMLDesc(0)
	if err != nil {
		return 0, 0, 0
	}
	paths := parseDiskPaths(x)
	var totalCap, totalAlloc uint64
	count := 0
	for _, p := range paths {
		info, err := d.GetBlockInfo(p, 0)
		if err != nil {
			continue
		}
		totalCap += info.Capacity
		totalAlloc += info.Allocation
		count++
	}
	return count, totalCap, totalAlloc
}

// bootedAt returns the qemu process start time for a running domain by
// reading the libvirt PID file at /run/libvirt/qemu/<name>.pid and using
// the kernel-provided ModTime of /proc/<pid> (which equals the process
// creation time on Linux). Returns zero time for remote URIs or any failure.
func (c *Client) bootedAt(name string) time.Time {
	// Only meaningful when libvirt is on the same host as dirt.
	if !strings.HasPrefix(c.uri, "qemu:///") {
		return time.Time{}
	}

	// libvirt writes the qemu PID to /run/libvirt/qemu/<domain>.pid by default.
	pidPath := "/run/libvirt/qemu/" + name + ".pid"
	pidBytes, err := os.ReadFile(pidPath)
	if err != nil {
		return time.Time{}
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil || pid <= 0 {
		return time.Time{}
	}

	// On Linux, the mtime of /proc/<pid> is set to the process creation time.
	info, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
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
