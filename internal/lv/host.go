package lv

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"libvirt.org/go/libvirt"
)

// HostStats is a sample of dynamic host metrics — CPU, memory, swap, load,
// uptime. For a local libvirt connection we read /proc/* directly. For a
// remote connection we fall back to libvirt's virNodeGetCPUStats and
// virNodeGetMemoryStats, which give us CPU times and memory totals but no
// swap, load average, or host uptime. In that case Remote is true and the
// swap / load / uptime fields stay zero, letting the renderer hide them.
//
// Cumulative CPU times need to be diffed against a previous sample to compute
// a percentage; the differencing happens in the UI. Units differ between the
// two sources (jiffies locally, nanoseconds via libvirt) but the ratio used
// for the percentage is dimensionless, so consumers need not care.
type HostStats struct {
	SampledAt time.Time

	// Remote indicates the sample came from libvirt node APIs rather than
	// /proc, and therefore lacks swap, load average, uptime, and the
	// reclaimable-cache breakdown.
	Remote bool

	// CPU times (cumulative since boot). Units are jiffies when Remote is
	// false, nanoseconds when Remote is true.
	CPUUser    uint64
	CPUNice    uint64
	CPUSystem  uint64
	CPUIdle    uint64
	CPUIOWait  uint64
	CPUIRQ     uint64
	CPUSoftIRQ uint64
	CPUSteal   uint64

	// Memory in KB.
	MemTotalKB     uint64
	MemFreeKB      uint64
	MemAvailableKB uint64
	BuffersKB      uint64
	CachedKB       uint64
	SReclaimableKB uint64
	SwapTotalKB    uint64
	SwapFreeKB     uint64

	// Load average.
	Load1, Load5, Load15 float64

	// Host uptime in seconds.
	UptimeSeconds float64
}

// CPUTotal returns the sum of all CPU time fields.
func (s HostStats) CPUTotal() uint64 {
	return s.CPUUser + s.CPUNice + s.CPUSystem + s.CPUIdle +
		s.CPUIOWait + s.CPUIRQ + s.CPUSoftIRQ + s.CPUSteal
}

// CPUActive returns CPU time spent doing useful work (excluding idle/iowait).
func (s HostStats) CPUActive() uint64 {
	return s.CPUTotal() - s.CPUIdle - s.CPUIOWait
}

// MemUsedKB returns the htop definition of "used" — total minus the
// reclaimable parts (free, buffers, cached, sreclaimable).
func (s HostStats) MemUsedKB() uint64 {
	if s.MemTotalKB == 0 {
		return 0
	}
	used := s.MemTotalKB - s.MemFreeKB - s.BuffersKB - s.CachedKB - s.SReclaimableKB
	if used > s.MemTotalKB {
		return 0 // underflow
	}
	return used
}

// MemCacheKB returns buffers + cache + sreclaimable (the yellow segment in htop).
func (s HostStats) MemCacheKB() uint64 {
	return s.BuffersKB + s.CachedKB + s.SReclaimableKB
}

// SwapUsedKB returns swap total - swap free.
func (s HostStats) SwapUsedKB() uint64 {
	if s.SwapTotalKB < s.SwapFreeKB {
		return 0
	}
	return s.SwapTotalKB - s.SwapFreeKB
}

// HostStats populates a HostStats sample for the connected hypervisor.
// For local libvirt (qemu:///…) it reads /proc/* directly and returns the
// full set of fields. For any remote URI it falls back to libvirt's own
// node-stats APIs, which yield CPU and memory but not swap, load, or
// uptime; those stay zero and Remote is set to true.
func (c *Client) HostStats() (HostStats, error) {
	var s HostStats
	s.SampledAt = time.Now()

	if strings.HasPrefix(c.uri, "qemu:///") {
		if err := readProcStat(&s); err != nil {
			return s, fmt.Errorf("read /proc/stat: %w", err)
		}
		if err := readProcMeminfo(&s); err != nil {
			return s, fmt.Errorf("read /proc/meminfo: %w", err)
		}
		_ = readProcLoadavg(&s)
		_ = readProcUptime(&s)
		return s, nil
	}

	s.Remote = true
	if err := c.readNodeCPUStats(&s); err != nil {
		return s, fmt.Errorf("virNodeGetCPUStats: %w", err)
	}
	if err := c.readNodeMemoryStats(&s); err != nil {
		return s, fmt.Errorf("virNodeGetMemoryStats: %w", err)
	}
	return s, nil
}

// readNodeCPUStats populates the CPU time fields from libvirt's
// virNodeGetCPUStats API — usable over any connection. Values are reported
// in nanoseconds, not jiffies; percentages in the UI are ratios so the unit
// change is invisible to consumers.
func (c *Client) readNodeCPUStats(s *HostStats) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("nil connection")
	}
	stats, err := c.conn.GetCPUStats(int(libvirt.NODE_CPU_STATS_ALL_CPUS), 0)
	if err != nil {
		return err
	}
	if stats.UserSet {
		s.CPUUser = stats.User
	}
	if stats.KernelSet {
		s.CPUSystem = stats.Kernel
	}
	if stats.IdleSet {
		s.CPUIdle = stats.Idle
	}
	if stats.IowaitSet {
		s.CPUIOWait = stats.Iowait
	}
	if stats.IntrSet {
		s.CPUIRQ = stats.Intr
	}
	return nil
}

// readNodeMemoryStats populates the memory fields from libvirt's
// virNodeGetMemoryStats API. Values are reported in KB, matching /proc/meminfo.
// Libvirt exposes Total, Free, Buffers, Cached — no SReclaimable, no swap.
func (c *Client) readNodeMemoryStats(s *HostStats) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("nil connection")
	}
	stats, err := c.conn.GetMemoryStats(libvirt.NODE_MEMORY_STATS_ALL_CELLS, 0)
	if err != nil {
		return err
	}
	if stats.TotalSet {
		s.MemTotalKB = stats.Total
	}
	if stats.FreeSet {
		s.MemFreeKB = stats.Free
		s.MemAvailableKB = stats.Free + stats.Buffers + stats.Cached
	}
	if stats.BuffersSet {
		s.BuffersKB = stats.Buffers
	}
	if stats.CachedSet {
		s.CachedKB = stats.Cached
	}
	return nil
}

func readProcStat(s *HostStats) error {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		// "cpu  user nice system idle iowait irq softirq steal guest guest_nice"
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return fmt.Errorf("unexpected /proc/stat line: %q", line)
		}
		vals := make([]uint64, 0, 8)
		for _, fld := range fields[1:] {
			v, _ := strconv.ParseUint(fld, 10, 64)
			vals = append(vals, v)
		}
		// Assign in order, defaulting to 0 if absent.
		get := func(i int) uint64 {
			if i < len(vals) {
				return vals[i]
			}
			return 0
		}
		s.CPUUser = get(0)
		s.CPUNice = get(1)
		s.CPUSystem = get(2)
		s.CPUIdle = get(3)
		s.CPUIOWait = get(4)
		s.CPUIRQ = get(5)
		s.CPUSoftIRQ = get(6)
		s.CPUSteal = get(7)
		return nil
	}
	return fmt.Errorf("no aggregated cpu line in /proc/stat")
}

func readProcMeminfo(s *HostStats) error {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		key, val, ok := parseMeminfoLine(line)
		if !ok {
			continue
		}
		switch key {
		case "MemTotal":
			s.MemTotalKB = val
		case "MemFree":
			s.MemFreeKB = val
		case "MemAvailable":
			s.MemAvailableKB = val
		case "Buffers":
			s.BuffersKB = val
		case "Cached":
			s.CachedKB = val
		case "SReclaimable":
			s.SReclaimableKB = val
		case "SwapTotal":
			s.SwapTotalKB = val
		case "SwapFree":
			s.SwapFreeKB = val
		}
	}
	return nil
}

// parseMeminfoLine parses one /proc/meminfo line: "Key: 12345 kB".
func parseMeminfoLine(line string) (string, uint64, bool) {
	colon := strings.IndexByte(line, ':')
	if colon < 0 {
		return "", 0, false
	}
	key := line[:colon]
	rest := strings.TrimSpace(line[colon+1:])
	rest = strings.TrimSuffix(rest, " kB")
	val, err := strconv.ParseUint(strings.TrimSpace(rest), 10, 64)
	if err != nil {
		return "", 0, false
	}
	return key, val, true
}

func readProcLoadavg(s *HostStats) error {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return err
	}
	fields := strings.Fields(string(b))
	if len(fields) < 3 {
		return fmt.Errorf("unexpected /proc/loadavg")
	}
	s.Load1, _ = strconv.ParseFloat(fields[0], 64)
	s.Load5, _ = strconv.ParseFloat(fields[1], 64)
	s.Load15, _ = strconv.ParseFloat(fields[2], 64)
	return nil
}

func readProcUptime(s *HostStats) error {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return err
	}
	fields := strings.Fields(string(b))
	if len(fields) < 1 {
		return fmt.Errorf("unexpected /proc/uptime")
	}
	s.UptimeSeconds, _ = strconv.ParseFloat(fields[0], 64)
	return nil
}

// readCPUModelName returns the CPU brand string from the first "model name"
// line in /proc/cpuinfo, e.g. "AMD Ryzen AI 9 HX 370 w/ Radeon 890M". On
// non-x86 architectures the field may be absent or differently named ("Hardware",
// "cpu") — returns "" in that case so the caller can fall back.
func readCPUModelName() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "model name") {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		return strings.TrimSpace(line[colon+1:])
	}
	return ""
}

// readOSPrettyName returns the PRETTY_NAME field from /etc/os-release,
// e.g. "Ubuntu 25.04" or "Debian GNU/Linux 12 (bookworm)". Returns "" if
// the file is missing or unparseable.
func readOSPrettyName() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "PRETTY_NAME=") {
			continue
		}
		v := strings.TrimPrefix(line, "PRETTY_NAME=")
		v = strings.Trim(v, `"'`)
		return v
	}
	return ""
}
