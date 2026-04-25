package lv

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// BridgeStats is one snapshot of host-side counters for a bridge
// interface, read from /sys/class/net/<name>/statistics/. Counters
// are monotonic byte/packet totals; rates are computed by the caller
// from successive samples.
type BridgeStats struct {
	Name     string
	RxBytes  uint64
	TxBytes  uint64
	RxPkts   uint64
	TxPkts   uint64
	OK       bool // false when /sys was unreadable (remote libvirt, missing iface)
}

// ReadBridgeStats returns a one-shot reading for each named bridge.
// Missing or unreadable bridges come back with OK=false rather than
// failing the whole batch — a remote libvirt URI simply yields all
// false entries, which the caller can render as "—".
func ReadBridgeStats(names []string) []BridgeStats {
	out := make([]BridgeStats, len(names))
	for i, n := range names {
		out[i] = BridgeStats{Name: n}
		if n == "" {
			continue
		}
		base := "/sys/class/net/" + n + "/statistics/"
		rxB, rOK := readUint(base + "rx_bytes")
		txB, tOK := readUint(base + "tx_bytes")
		if !rOK || !tOK {
			continue
		}
		rxP, _ := readUint(base + "rx_packets")
		txP, _ := readUint(base + "tx_packets")
		out[i] = BridgeStats{
			Name: n, RxBytes: rxB, TxBytes: txB, RxPkts: rxP, TxPkts: txP, OK: true,
		}
	}
	return out
}

func readUint(path string) (uint64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	s := strings.TrimSpace(string(b))
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// String renders a BridgeStats for debugging.
func (b BridgeStats) String() string {
	if !b.OK {
		return fmt.Sprintf("%s: (unavailable)", b.Name)
	}
	return fmt.Sprintf("%s: rx=%d tx=%d", b.Name, b.RxBytes, b.TxBytes)
}
