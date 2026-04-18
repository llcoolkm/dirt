package ui

import (
	"time"

	"github.com/llcoolkm/dirt/internal/lv"
)



// historyWindow is how many samples we keep per metric. With a 1s
// refresh this covers 5 minutes of data — enough for the sparklines
// in the header *and* the full-width performance graphs view.
const historyWindow = 300

// domHistory tracks rolling per-domain stats for rate and sparkline computation.
// All Last* fields hold the previous cumulative reading; rates come from diffs.
type domHistory struct {
	cpu         []float64   // % of one vCPU (aggregate)
	cpuUser     []float64   // user-space CPU %
	cpuSystem   []float64   // kernel CPU %
	vcpuPct     [][]float64 // per-vCPU CPU % (one slice per vCPU)
	memUsedPct  []float64   // guest memory used% (from balloon stats)
	memCachePct []float64   // page cache % of balloon available
	swapUsedPct []float64   // swap used% (from QGA, updated externally)
	blockRd     []float64   // bytes/sec
	blockWr     []float64
	blockRdOps  []float64   // IOPS
	blockWrOps  []float64
	rdLatencyUs []float64   // µs per read op
	wrLatencyUs []float64   // µs per write op
	netRx       []float64   // bytes/sec
	netTx       []float64
	netRxPps    []float64   // packets/sec
	netTxPps    []float64
	swapIn      []float64   // pages/sec
	swapOut     []float64
	majorFault  []float64   // faults/sec

	// Cumulative previous samples for delta math.
	lastCPUNs       uint64
	lastCPUUserNs   uint64
	lastCPUSystemNs uint64
	lastVCPUTimes   []uint64
	lastBlockRd     uint64
	lastBlockWr     uint64
	lastBlockRdReqs uint64
	lastBlockWrReqs uint64
	lastBlockRdTime uint64
	lastBlockWrTime uint64
	lastNetRx       uint64
	lastNetTx       uint64
	lastNetRxPkts   uint64
	lastNetTxPkts   uint64
	lastSwapIn      uint64
	lastSwapOut     uint64
	lastMajorFault  uint64
	lastT           time.Time
	hasPrev         bool

	// First time this domain was observed in the running state. Used as a
	// dirt-side estimate of uptime — accurate from the moment dirt started.
	firstRunningSince time.Time
}

// update appends one new sample, computing rates from the previous reading.
func (h *domHistory) update(d lv.Domain) {
	// Mark first time we saw this domain running for uptime estimation.
	if h.firstRunningSince.IsZero() {
		h.firstRunningSince = d.SampledAt
	}

	// Memory gauges are instantaneous, not cumulative counters.
	if d.BalloonAvailableKB > 0 {
		usedKB := d.BalloonAvailableKB - d.BalloonUnusedKB - d.BalloonDiskCachesKB
		usedPct := float64(usedKB) / float64(d.BalloonAvailableKB) * 100
		cachePct := float64(d.BalloonDiskCachesKB) / float64(d.BalloonAvailableKB) * 100
		h.memUsedPct = appendCap(h.memUsedPct, usedPct, historyWindow)
		h.memCachePct = appendCap(h.memCachePct, cachePct, historyWindow)
	}

	if h.hasPrev {
		dt := d.SampledAt.Sub(h.lastT).Seconds()
		if dt > 0 {
			// CPU% = (Δ cpu time) / (Δ wall time) * 100
			// CPUTimeNs is aggregate over all vCPUs; divide by NrVCPU for per-core %.
			dCPU := float64(d.CPUTimeNs-h.lastCPUNs) / 1e9
			vcpus := float64(d.NrVCPU)
			if vcpus < 1 {
				vcpus = 1
			}
			cpuPct := dCPU / dt / vcpus * 100
			h.cpu = appendCap(h.cpu, cpuPct, historyWindow)

			// User / system CPU breakdown. Clamp to [0,100] — timing
			// jitter in the delta can briefly produce >100%.
			if d.CPUUserNs > 0 || h.lastCPUUserNs > 0 {
				userPct := float64(d.CPUUserNs-h.lastCPUUserNs) / 1e9 / dt / vcpus * 100
				if userPct > 100 {
					userPct = 100
				}
				if userPct < 0 {
					userPct = 0
				}
				h.cpuUser = appendCap(h.cpuUser, userPct, historyWindow)
			}
			if d.CPUSystemNs > 0 || h.lastCPUSystemNs > 0 {
				sysPct := float64(d.CPUSystemNs-h.lastCPUSystemNs) / 1e9 / dt / vcpus * 100
				if sysPct > 100 {
					sysPct = 100
				}
				if sysPct < 0 {
					sysPct = 0
				}
				h.cpuSystem = appendCap(h.cpuSystem, sysPct, historyWindow)
			}

			// Per-vCPU breakdown.
			if len(d.VCPUTimes) > 0 && len(h.lastVCPUTimes) == len(d.VCPUTimes) {
				for len(h.vcpuPct) < len(d.VCPUTimes) {
					h.vcpuPct = append(h.vcpuPct, nil)
				}
				for i, t := range d.VCPUTimes {
					pct := float64(t-h.lastVCPUTimes[i]) / 1e9 / dt * 100
					h.vcpuPct[i] = appendCap(h.vcpuPct[i], pct, historyWindow)
				}
			}

			h.blockRd = appendCap(h.blockRd, float64(d.BlockRdBytes-h.lastBlockRd)/dt, historyWindow)
			h.blockWr = appendCap(h.blockWr, float64(d.BlockWrBytes-h.lastBlockWr)/dt, historyWindow)

			rdOps := float64(d.BlockRdReqs-h.lastBlockRdReqs) / dt
			wrOps := float64(d.BlockWrReqs-h.lastBlockWrReqs) / dt
			h.blockRdOps = appendCap(h.blockRdOps, rdOps, historyWindow)
			h.blockWrOps = appendCap(h.blockWrOps, wrOps, historyWindow)

			// Latency = (Δ time in nanoseconds) / (Δ requests). Convert to µs.
			rdReqDelta := float64(d.BlockRdReqs - h.lastBlockRdReqs)
			if rdReqDelta > 0 {
				h.rdLatencyUs = appendCap(h.rdLatencyUs,
					float64(d.BlockRdTimes-h.lastBlockRdTime)/rdReqDelta/1000, historyWindow)
			} else {
				h.rdLatencyUs = appendCap(h.rdLatencyUs, 0, historyWindow)
			}
			wrReqDelta := float64(d.BlockWrReqs - h.lastBlockWrReqs)
			if wrReqDelta > 0 {
				h.wrLatencyUs = appendCap(h.wrLatencyUs,
					float64(d.BlockWrTimes-h.lastBlockWrTime)/wrReqDelta/1000, historyWindow)
			} else {
				h.wrLatencyUs = appendCap(h.wrLatencyUs, 0, historyWindow)
			}

			h.netRx = appendCap(h.netRx, float64(d.NetRxBytes-h.lastNetRx)/dt, historyWindow)
			h.netTx = appendCap(h.netTx, float64(d.NetTxBytes-h.lastNetTx)/dt, historyWindow)
			h.netRxPps = appendCap(h.netRxPps, float64(d.NetRxPkts-h.lastNetRxPkts)/dt, historyWindow)
			h.netTxPps = appendCap(h.netTxPps, float64(d.NetTxPkts-h.lastNetTxPkts)/dt, historyWindow)

			h.swapIn = appendCap(h.swapIn, float64(d.BalloonSwapIn-h.lastSwapIn)/dt, historyWindow)
			h.swapOut = appendCap(h.swapOut, float64(d.BalloonSwapOut-h.lastSwapOut)/dt, historyWindow)
			h.majorFault = appendCap(h.majorFault, float64(d.BalloonMajorFault-h.lastMajorFault)/dt, historyWindow)
		}
	}
	h.lastCPUNs = d.CPUTimeNs
	h.lastCPUUserNs = d.CPUUserNs
	h.lastCPUSystemNs = d.CPUSystemNs
	if len(d.VCPUTimes) > 0 {
		h.lastVCPUTimes = make([]uint64, len(d.VCPUTimes))
		copy(h.lastVCPUTimes, d.VCPUTimes)
	}
	h.lastBlockRd = d.BlockRdBytes
	h.lastBlockWr = d.BlockWrBytes
	h.lastBlockRdReqs = d.BlockRdReqs
	h.lastBlockWrReqs = d.BlockWrReqs
	h.lastBlockRdTime = d.BlockRdTimes
	h.lastBlockWrTime = d.BlockWrTimes
	h.lastNetRx = d.NetRxBytes
	h.lastNetTx = d.NetTxBytes
	h.lastNetRxPkts = d.NetRxPkts
	h.lastNetTxPkts = d.NetTxPkts
	h.lastSwapIn = d.BalloonSwapIn
	h.lastSwapOut = d.BalloonSwapOut
	h.lastMajorFault = d.BalloonMajorFault
	h.lastT = d.SampledAt
	h.hasPrev = true
}

// uptime returns the dirt-side uptime estimate (since we first saw the VM
// running). Used as a fallback when no real boot time is available.
func (h *domHistory) uptime() time.Duration {
	if h.firstRunningSince.IsZero() {
		return 0
	}
	return time.Since(h.firstRunningSince)
}

// effectiveUptime returns the most accurate uptime we can compute for a domain.
// Preference order, most accurate first:
//   1. guest /proc/uptime via qemu-guest-agent — survives in-VM reboots
//   2. qemu host process start time from /proc/<pid> — local URIs only
//   3. dirt-side observation window — coarse, under-estimates after a fresh start
//
// The bool is true when the value reflects the *guest's* actual boot time
// (sources 1 and 2). It is false when we're relying on the dirt-side estimate.
func effectiveUptime(d lv.Domain, h *domHistory, qga lv.GuestUptime) (time.Duration, bool) {
	// 1. QGA wins: it knows about in-guest reboots.
	if qga.Available && !qga.BootedAt.IsZero() {
		return time.Since(qga.BootedAt), true
	}
	// 2. qemu process start time — accurate for the qemu process, but does
	//    not reflect guest-internal reboots that keep qemu running.
	if !d.BootedAt.IsZero() {
		return time.Since(d.BootedAt), true
	}
	// 3. Fallback: when we first saw it running.
	if h != nil {
		return h.uptime(), false
	}
	return 0, false
}

// reset wipes history (used when a domain stops, so old rates don't linger).
// Note: also clears firstRunningSince so the next start computes uptime fresh.
func (h *domHistory) reset() {
	*h = domHistory{}
}

// anomalyThreshold is the CPU or memory percentage above which a VM
// is considered hot. anomalyConsecutive is how many consecutive
// samples above the threshold before we raise an alert.
const (
	anomalyThreshold   = 90.0
	anomalyConsecutive = 5
)

// checkAnomaly returns alert strings for each metric that has been
// above the threshold for at least anomalyConsecutive consecutive
// samples. Used by the main refresh path to flash warnings.
func (h *domHistory) checkAnomaly() []string {
	var alerts []string
	if breach("CPU", h.cpu, anomalyThreshold, anomalyConsecutive) {
		alerts = append(alerts, "CPU > 90%")
	}
	if breach("MEM", h.memUsedPct, anomalyThreshold, anomalyConsecutive) {
		alerts = append(alerts, "MEM > 90%")
	}
	return alerts
}

// breach reports whether the last `n` samples of `s` are all above `threshold`.
func breach(_ string, s []float64, threshold float64, n int) bool {
	if len(s) < n {
		return false
	}
	tail := s[len(s)-n:]
	for _, v := range tail {
		if v < threshold {
			return false
		}
	}
	return true
}

// currentCPU returns the most recent CPU% sample, or 0.
func (h *domHistory) currentCPU() float64 {
	if len(h.cpu) == 0 {
		return 0
	}
	return h.cpu[len(h.cpu)-1]
}

// currentRate returns the most recent value of a series, or 0.
func currentRate(s []float64) float64 {
	if len(s) == 0 {
		return 0
	}
	return s[len(s)-1]
}

func appendCap(s []float64, v float64, cap int) []float64 {
	s = append(s, v)
	if len(s) > cap {
		s = s[len(s)-cap:]
	}
	return s
}
