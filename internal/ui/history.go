package ui

import (
	"time"

	"github.com/llcoolkm/dirt/internal/lv"
)

// historyWindow is how many samples we keep per metric for sparklines.
const historyWindow = 30

// domHistory tracks rolling per-domain stats for rate and sparkline computation.
// All Last* fields hold the previous cumulative reading; rates come from diffs.
type domHistory struct {
	cpu        []float64 // % of one vCPU
	blockRd    []float64 // bytes/sec
	blockWr    []float64
	blockRdOps []float64 // IOPS
	blockWrOps []float64
	netRx      []float64 // bytes/sec
	netTx      []float64
	netRxPps   []float64 // packets/sec
	netTxPps   []float64
	swapIn     []float64 // pages/sec
	swapOut    []float64
	majorFault []float64 // faults/sec

	// Latest computed average latency in microseconds (read / write).
	rdLatencyUs float64
	wrLatencyUs float64

	// Cumulative previous samples for delta math.
	lastCPUNs       uint64
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

			h.blockRd = appendCap(h.blockRd, float64(d.BlockRdBytes-h.lastBlockRd)/dt, historyWindow)
			h.blockWr = appendCap(h.blockWr, float64(d.BlockWrBytes-h.lastBlockWr)/dt, historyWindow)

			rdOps := float64(d.BlockRdReqs-h.lastBlockRdReqs) / dt
			wrOps := float64(d.BlockWrReqs-h.lastBlockWrReqs) / dt
			h.blockRdOps = appendCap(h.blockRdOps, rdOps, historyWindow)
			h.blockWrOps = appendCap(h.blockWrOps, wrOps, historyWindow)

			// Latency = (Δ time in nanoseconds) / (Δ requests). Convert to µs.
			rdReqDelta := float64(d.BlockRdReqs - h.lastBlockRdReqs)
			if rdReqDelta > 0 {
				h.rdLatencyUs = float64(d.BlockRdTimes-h.lastBlockRdTime) / rdReqDelta / 1000
			}
			wrReqDelta := float64(d.BlockWrReqs - h.lastBlockWrReqs)
			if wrReqDelta > 0 {
				h.wrLatencyUs = float64(d.BlockWrTimes-h.lastBlockWrTime) / wrReqDelta / 1000
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

// uptime returns "≥ duration" since dirt first saw this VM running.
func (h *domHistory) uptime() time.Duration {
	if h.firstRunningSince.IsZero() {
		return 0
	}
	return time.Since(h.firstRunningSince)
}

// reset wipes history (used when a domain stops, so old rates don't linger).
// Note: also clears firstRunningSince so the next start computes uptime fresh.
func (h *domHistory) reset() {
	*h = domHistory{}
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
