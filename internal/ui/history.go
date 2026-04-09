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
	cpu     []float64 // % of one vCPU
	blockRd []float64 // bytes/sec
	blockWr []float64
	netRx   []float64
	netTx   []float64
	swapIn  []float64 // pages/sec
	swapOut []float64

	lastCPUNs    uint64
	lastBlockRd  uint64
	lastBlockWr  uint64
	lastNetRx    uint64
	lastNetTx    uint64
	lastSwapIn   uint64
	lastSwapOut  uint64
	lastT        time.Time
	hasPrev      bool
}

// update appends one new sample, computing rates from the previous reading.
func (h *domHistory) update(d lv.Domain) {
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
			h.netRx = appendCap(h.netRx, float64(d.NetRxBytes-h.lastNetRx)/dt, historyWindow)
			h.netTx = appendCap(h.netTx, float64(d.NetTxBytes-h.lastNetTx)/dt, historyWindow)
			h.swapIn = appendCap(h.swapIn, float64(d.BalloonSwapIn-h.lastSwapIn)/dt, historyWindow)
			h.swapOut = appendCap(h.swapOut, float64(d.BalloonSwapOut-h.lastSwapOut)/dt, historyWindow)
		}
	}
	h.lastCPUNs = d.CPUTimeNs
	h.lastBlockRd = d.BlockRdBytes
	h.lastBlockWr = d.BlockWrBytes
	h.lastNetRx = d.NetRxBytes
	h.lastNetTx = d.NetTxBytes
	h.lastSwapIn = d.BalloonSwapIn
	h.lastSwapOut = d.BalloonSwapOut
	h.lastT = d.SampledAt
	h.hasPrev = true
}

// reset wipes history (used when a domain stops, so old rates don't linger).
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
