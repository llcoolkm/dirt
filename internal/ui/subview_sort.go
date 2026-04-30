package ui

import (
	"sort"
	"strings"

	"github.com/llcoolkm/dirt/internal/config"
	"github.com/llcoolkm/dirt/internal/lv"
)

// Header arrows shown next to the active sort column in subviews.
// Mirrors the main table's ▲ / ▼ convention.
func sortArrow(desc bool) string {
	if desc {
		return "▼"
	}
	return "▲"
}

// arrowedHeader returns the column label with the appropriate arrow
// when active is true; otherwise the bare label.
func arrowedHeader(label string, active, desc bool) string {
	if !active {
		return label
	}
	return label + sortArrow(desc)
}

// ──────────────────────────── Networks ──────────────────────────────

const (
	netColName int = iota
	netColState
	netColAuto
	netColBridge
	netColForward
	netColLeases
	netColRX
	netColTX
	netColMax
)

func (m Model) sortedNetworks() []lv.Network {
	out := append([]lv.Network(nil), m.networks...)
	flip := func(b bool) bool {
		if m.networksSortDesc {
			return !b
		}
		return b
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch m.networksSortIdx {
		case netColState:
			if a.Active != b.Active {
				return flip(a.Active && !b.Active)
			}
			return a.Name < b.Name
		case netColAuto:
			if a.Autostart != b.Autostart {
				return flip(a.Autostart && !b.Autostart)
			}
			return a.Name < b.Name
		case netColBridge:
			if a.Bridge != b.Bridge {
				return flip(a.Bridge < b.Bridge)
			}
			return a.Name < b.Name
		case netColForward:
			if a.Forward != b.Forward {
				return flip(a.Forward < b.Forward)
			}
			return a.Name < b.Name
		case netColLeases:
			if a.NumLeases != b.NumLeases {
				return flip(a.NumLeases > b.NumLeases)
			}
			return a.Name < b.Name
		case netColRX:
			ra := m.bridgeRates[a.Bridge].rxBps
			rb := m.bridgeRates[b.Bridge].rxBps
			if ra != rb {
				return flip(ra > rb)
			}
			return a.Name < b.Name
		case netColTX:
			ra := m.bridgeRates[a.Bridge].txBps
			rb := m.bridgeRates[b.Bridge].txBps
			if ra != rb {
				return flip(ra > rb)
			}
			return a.Name < b.Name
		}
		return flip(a.Name < b.Name)
	})
	return out
}

// netHeaderClick applies a click on the networks-view header row at
// terminal x-coordinate.
func (m Model) netHeaderClick(x int) Model {
	widths := []int{netNameW, netStateW, netAutoW, netBridgeW, netForwardW, netLeasesW, netRateW, netRateW}
	idx, ok := clickedHeaderColIdx(x, widths)
	if !ok {
		return m
	}
	if m.networksSortIdx == idx {
		m.networksSortDesc = !m.networksSortDesc
	} else {
		m.networksSortIdx = idx
		m.networksSortDesc = false
	}
	return m
}

// ──────────────────────────── Pools ──────────────────────────────

const (
	poolColName int = iota
	poolColState
	poolColType
	poolColCap
	poolColAlloc
	poolColFree
	poolColUsage
	poolColMax
)

func (m Model) sortedPools() []lv.StoragePool {
	out := append([]lv.StoragePool(nil), m.pools...)
	flip := func(b bool) bool {
		if m.poolsSortDesc {
			return !b
		}
		return b
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch m.poolsSortIdx {
		case poolColState:
			if a.State != b.State {
				return flip(a.State < b.State)
			}
			return a.Name < b.Name
		case poolColType:
			if a.Type != b.Type {
				return flip(a.Type < b.Type)
			}
			return a.Name < b.Name
		case poolColCap:
			if a.Capacity != b.Capacity {
				return flip(a.Capacity > b.Capacity)
			}
			return a.Name < b.Name
		case poolColAlloc:
			if a.Allocation != b.Allocation {
				return flip(a.Allocation > b.Allocation)
			}
			return a.Name < b.Name
		case poolColFree:
			if a.Available != b.Available {
				return flip(a.Available > b.Available)
			}
			return a.Name < b.Name
		case poolColUsage:
			pa, pb := poolUsagePct(a), poolUsagePct(b)
			if pa != pb {
				return flip(pa > pb)
			}
			return a.Name < b.Name
		}
		return flip(a.Name < b.Name)
	})
	return out
}

func poolUsagePct(p lv.StoragePool) float64 {
	if p.Capacity == 0 {
		return 0
	}
	return float64(p.Allocation) / float64(p.Capacity) * 100
}

func (m Model) poolHeaderClick(x int) Model {
	widths := []int{poolNameW, poolStateW, poolTypeW, poolCapW, poolAllocW, poolFreeW, poolUsageW}
	idx, ok := clickedHeaderColIdx(x, widths)
	if !ok {
		return m
	}
	if m.poolsSortIdx == idx {
		m.poolsSortDesc = !m.poolsSortDesc
	} else {
		m.poolsSortIdx = idx
		m.poolsSortDesc = false
	}
	return m
}

// ──────────────────────────── Volumes ──────────────────────────────

const (
	volColName int = iota
	volColType
	volColCap
	volColAlloc
	volColPath
	volColMax
)

func (m Model) sortedVolumes() []lv.StorageVolume {
	out := append([]lv.StorageVolume(nil), m.volumes...)
	flip := func(b bool) bool {
		if m.volumesSortDesc {
			return !b
		}
		return b
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch m.volumesSortIdx {
		case volColType:
			if a.Type != b.Type {
				return flip(a.Type < b.Type)
			}
			return a.Name < b.Name
		case volColCap:
			if a.Capacity != b.Capacity {
				return flip(a.Capacity > b.Capacity)
			}
			return a.Name < b.Name
		case volColAlloc:
			if a.Allocation != b.Allocation {
				return flip(a.Allocation > b.Allocation)
			}
			return a.Name < b.Name
		case volColPath:
			if a.Path != b.Path {
				return flip(a.Path < b.Path)
			}
			return a.Name < b.Name
		}
		return flip(a.Name < b.Name)
	})
	return out
}

func (m Model) volHeaderClick(x int) Model {
	widths := []int{volNameW, volTypeW, volCapW, volAllocW, volPathW}
	idx, ok := clickedHeaderColIdx(x, widths)
	if !ok {
		return m
	}
	if m.volumesSortIdx == idx {
		m.volumesSortDesc = !m.volumesSortDesc
	} else {
		m.volumesSortIdx = idx
		m.volumesSortDesc = false
	}
	return m
}

// ──────────────────────────── Hosts ──────────────────────────────

const (
	hostColName int = iota
	hostColURI
	hostColStatus
	hostColDomains
	hostColMax
)

func (m Model) sortedHosts() []config.Host {
	out := append([]config.Host(nil), m.hosts...)
	flip := func(b bool) bool {
		if m.hostsSortDesc {
			return !b
		}
		return b
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch m.hostsSortIdx {
		case hostColURI:
			if a.URI != b.URI {
				return flip(a.URI < b.URI)
			}
			return a.Name < b.Name
		case hostColStatus:
			sa := int(m.hostsProbe[a.Name].state)
			sb := int(m.hostsProbe[b.Name].state)
			if sa != sb {
				return flip(sa < sb)
			}
			return a.Name < b.Name
		case hostColDomains:
			da := m.hostsProbe[a.Name].domains
			db := m.hostsProbe[b.Name].domains
			if da != db {
				return flip(da > db)
			}
			return a.Name < b.Name
		}
		return flip(strings.ToLower(a.Name) < strings.ToLower(b.Name))
	})
	return out
}

func (m Model) hostHeaderClick(x int) Model {
	widths := []int{hostsNameW, hostsURIW, hostsStatusW, hostsDomainsW}
	idx, ok := clickedHeaderColIdx(x, widths)
	if !ok {
		return m
	}
	if m.hostsSortIdx == idx {
		m.hostsSortDesc = !m.hostsSortDesc
	} else {
		m.hostsSortIdx = idx
		m.hostsSortDesc = false
	}
	return m
}

// ──────────────────────────── Helpers ──────────────────────────────

// clickedHeaderColIdx maps an x-coordinate from a click on a subview
// header row to a column index. Subview layouts share a common
// prefix: 1 (left border) + 1 (padding) + 1 (indent) = 3 cells of
// non-column space, then column widths separated by 2-cell gaps.
func clickedHeaderColIdx(x int, widths []int) (int, bool) {
	cur := 3
	for i, w := range widths {
		end := cur + w
		if x >= cur && x < end {
			return i, true
		}
		if i < len(widths)-1 {
			cur = end + 2
		}
	}
	return 0, false
}
