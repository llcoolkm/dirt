package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/llcoolkm/dirt/internal/lv"
)

// sideBySideMinWidth is the terminal width threshold below which we stack
// the VM and host boxes vertically instead of placing them side by side.
const sideBySideMinWidth = 110

// borderWidth accounts for lipgloss's convention that Style.Width() sets the
// *content* width (including padding) and adds the border on top. A bordered
// style with Width(N) renders as N+2 total characters, so callers that need
// the final rendered width to equal W must pass W-borderWidth.
const borderWidth = 2

// headerView renders the top pane. When the terminal is wide enough, it
// places a per-VM box on the left and a host summary on the right; on narrow
// terminals it falls back to a single full-width box.
func (m Model) headerView() string {
	width := m.contentWidth()
	if m.snap == nil {
		return headerBox.Width(width - borderWidth).Render("connecting to libvirt…")
	}

	if width < sideBySideMinWidth {
		// Narrow terminal: just show the VM (or host) box at full width.
		return m.renderVMOrHostBox(width)
	}

	// Side-by-side: domain on the left, host on the right.
	leftW := width / 2
	rightW := width - leftW

	leftBox := m.renderVMBox(leftW)
	rightBox := m.renderHostBox(rightW)
	return lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)
}

// renderVMOrHostBox is the narrow-terminal fallback. We stack: host on top,
// VM beneath, both at full width.
func (m Model) renderVMOrHostBox(width int) string {
	host := m.renderHostBox(width)
	vm := m.renderVMBox(width)
	return lipgloss.JoinVertical(lipgloss.Left, host, vm)
}

// renderVMBox renders the per-VM stats pane (or a placeholder if nothing is
// selected).
func (m Model) renderVMBox(boxWidth int) string {
	d, ok := m.currentDomain()
	if !ok {
		return m.emptyVMBox(boxWidth)
	}
	if d.State == lv.StateRunning {
		return m.runningVMBox(d, boxWidth)
	}
	return m.idleVMBox(d, boxWidth)
}

// emptyVMBox is shown on the left when no domain is selected.
func (m Model) emptyVMBox(boxWidth int) string {
	title := headerTitle.Render("no VM selected")
	hint := headerLabel.Render("press ") + keyHint.Render("j") + headerLabel.Render(" / ") +
		keyHint.Render("k") + headerLabel.Render(" to navigate, ") +
		keyHint.Render("/") + headerLabel.Render(" to filter")
	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		hint,
		"",
		"",
		"",
	)
	return headerBox.Width(boxWidth - borderWidth).Render(body)
}

// runningVMBox draws CPU / MEM / SWAP / DISK / NET for a running domain.
// Decluttered: no inline memory breakdown, no major-fault inline, no IOPS or
// latency suffixes — those metrics still exist in history and may be exposed
// elsewhere later.
func (m Model) runningVMBox(d lv.Domain, boxWidth int) string {
	h := m.history[d.UUID]
	if h == nil {
		h = &domHistory{}
	}

	inner := boxWidth - 4
	if inner < 30 {
		inner = 30
	}

	// Title: name (bold) + state + uptime + OS (subtle).
	title := headerTitle.Render(d.Name) +
		headerLabel.Render("  ") + stateRunning.Render("running") +
		headerLabel.Render("  ") + headerValue.Render(fmt.Sprintf("%d vCPU", d.NrVCPU))
	if up, accurate := effectiveUptime(d, h, m.guestUptime[d.Name]); up > 0 {
		label := formatDuration(up)
		if !accurate {
			label = "≥" + label
		}
		title += headerLabel.Render("  uptime ") + headerValue.Render(label)
	}
	if d.OS != "" {
		title += headerLabel.Render("  ") + headerValue.Render(d.OS)
	}

	cpuLine := buildCPULine("CPU  ", h.currentCPU(), inner)
	memLine := buildVMMemLine(d, inner)
	swapLine := buildVMSwapLine(d, h, m.swap[d.Name], inner)
	diskLine := buildVMDiskLine(h, inner)
	netLine := buildVMNetLine(h, inner)
	storeLine := buildVMStorageLine(d, h)

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		cpuLine,
		memLine,
		swapLine,
		diskLine,
		netLine,
		storeLine,
	)
	return headerBox.Width(boxWidth - borderWidth).Render(body)
}

// idleVMBox is shown for non-running VMs — config snapshot, no live stats.
// Padded to 7 lines so it matches the height of the host box and the running
// VM box for clean side-by-side rendering.
func (m Model) idleVMBox(d lv.Domain, boxWidth int) string {
	stateStr := stateStyleFor(d.State).Render(d.State.String())

	title := headerTitle.Render(d.Name) +
		headerLabel.Render("  ") + stateStr
	if d.OS != "" {
		title += headerLabel.Render("  ") + headerValue.Render(d.OS)
	}

	autostart := "no"
	if d.Autostart {
		autostart = "yes"
	}
	persistent := "no"
	if d.Persistent {
		persistent = "yes"
	}

	specsLine := headerLabel.Render("vCPUs:    ") + headerValue.Render(fmt.Sprintf("%d", d.NrVCPU))
	memLine := headerLabel.Render("max mem:  ") + headerValue.Render(formatKB(d.MaxMemKB))
	autoLine := headerLabel.Render("autostart:") + headerValue.Render("  "+autostart) +
		headerLabel.Render("    persistent:") + headerValue.Render("  "+persistent)

	// Storage info — same line shape as the running box, but without IOPS.
	disksLabel := "1 disk"
	if d.NumDisks != 1 {
		disksLabel = fmt.Sprintf("%d disks", d.NumDisks)
	}
	usageLabel := "—"
	if d.TotalDiskCapacityBytes > 0 {
		usageLabel = fmt.Sprintf("%s/%s",
			formatBytes(float64(d.TotalDiskAllocationBytes)),
			formatBytes(float64(d.TotalDiskCapacityBytes)))
	}
	storeLine := headerLabel.Render("STORE     ") +
		headerValue.Render(usageLabel) +
		headerLabel.Render("  ·  ") +
		headerValue.Render(disksLabel)

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		specsLine,
		memLine,
		autoLine,
		"",
		storeLine,
	)
	return headerBox.Width(boxWidth - borderWidth).Render(body)
}

// renderHostBox draws the host pane: hostname, host CPU/MEM/SWAP, load, doms.
func (m Model) renderHostBox(boxWidth int) string {
	running := 0
	var allocVCPU uint
	var allocMemKB uint64
	for _, d := range m.snap.Domains {
		if d.State == lv.StateRunning {
			running++
			allocVCPU += d.NrVCPU
			allocMemKB += d.MaxMemKB
		}
	}

	inner := boxWidth - 4
	if inner < 30 {
		inner = 30
	}

	// Title: hostname, then logical CPU count *before* uptime to match the
	// VM box's "vCPU before uptime" order. OS pretty name comes last, also
	// matching the VM box. A subtle "(remote)" tag signals when the sample
	// came from libvirt's node APIs instead of /proc.
	title := headerTitle.Render("host: " + m.snap.Hostname)
	if m.hostStats.Remote {
		title += headerLabel.Render("  (remote)")
	}
	if m.host.CPUs > 0 {
		title += headerLabel.Render("  ") + headerValue.Render(fmt.Sprintf("%d cores", m.host.CPUs))
	}
	if m.hostStats.UptimeSeconds > 0 {
		uptime := time.Duration(m.hostStats.UptimeSeconds * float64(time.Second))
		title += headerLabel.Render("  uptime ") + headerValue.Render(formatDuration(uptime))
	}
	if m.host.OSPretty != "" {
		title += headerLabel.Render("  ") + headerValue.Render(m.host.OSPretty)
	}

	// CPU info subtitle: model name + topology, on its own line.
	cpuInfo := buildHostCPUInfoLine(m.host, inner)

	cpuLine := buildHostCPULine(m.hostStats, m.hostCPUPct, inner)
	memLine := buildHostMemLine(m.hostStats, inner)
	swapLine := buildHostSwapLine(m.hostStats, inner)
	loadLine := buildHostLoadLine(m.hostStats)
	domsLine := buildHostDomsLine(m, len(m.snap.Domains), running, allocVCPU, allocMemKB)

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		cpuInfo,
		cpuLine,
		memLine,
		swapLine,
		loadLine,
		domsLine,
	)
	return headerBox.Width(boxWidth - borderWidth).Render(body)
}

// buildHostCPUInfoLine renders the CPU model + topology subtitle, e.g.:
//   "AMD Ryzen AI 9 HX 370 — 1 socket · 12 cores · 24 threads"
// Truncates the model name if necessary so the whole line fits in `inner`.
func buildHostCPUInfoLine(h lv.HostInfo, inner int) string {
	if h.CPUModel == "" {
		return ""
	}
	model := h.CPUModel
	// Topology breakdown — only when libvirt actually filled the fields.
	topo := ""
	if h.Sockets > 0 && h.CoresPerSocket > 0 && h.Threads > 0 {
		topo = fmt.Sprintf("%d socket · %d cores · %d threads",
			h.Sockets, h.Sockets*h.CoresPerSocket, h.Sockets*h.CoresPerSocket*h.Threads)
	}
	// Truncate the model if the combined line is too wide for the inner area.
	combined := model
	if topo != "" {
		combined = model + "  " + topo
	}
	if lipgloss.Width(combined) > inner {
		// Shrink the model name first, keeping the topology readable.
		room := inner - lipgloss.Width(topo) - 2
		if room < 8 {
			room = 8
		}
		model = truncate(model, room)
		combined = model + "  " + topo
	}
	if topo == "" {
		return headerLabel.Render(model)
	}
	return headerValue.Render(model) + headerLabel.Render("  "+topo)
}

// ──────────────────────────── Line builders ──────────────────────────────────

// buildCPULine renders a generic CPU bar line: "label [bar] pct%".
func buildCPULine(label string, pct float64, inner int) string {
	value := fmt.Sprintf(" %5.1f%%", pct)
	barW := inner - lipgloss.Width(label) - lipgloss.Width(value) - 2
	if barW < 10 {
		barW = 10
	}
	return headerLabel.Render(label) +
		headerLabel.Render("[") + colorBar(pct, barW) + headerLabel.Render("]") +
		headerValue.Render(value)
}

// buildVMMemLine renders the per-VM memory bar (multi-segment when balloon
// stats are present). Decluttered — no inline used/cache/free breakdown.
func buildVMMemLine(d lv.Domain, inner int) string {
	totalKB := d.BalloonAvailableKB
	if totalKB == 0 {
		totalKB = d.MaxMemKB
	}
	if totalKB == 0 {
		return headerLabel.Render("MEM  ") + headerValue.Render("(no stats)")
	}

	var usedKB, cacheKB uint64
	hasBalloon := d.BalloonAvailableKB > 0 && d.BalloonUnusedKB > 0
	if hasBalloon {
		cacheKB = d.BalloonDiskCachesKB
		freeKB := d.BalloonUnusedKB
		if totalKB > freeKB+cacheKB {
			usedKB = totalKB - freeKB - cacheKB
		}
	} else {
		usedKB = d.MemoryKB
		if usedKB > totalKB {
			usedKB = totalKB
		}
	}

	usedPct := float64(usedKB) / float64(totalKB) * 100
	cachePct := float64(cacheKB) / float64(totalKB) * 100

	label := "MEM  "
	detail := fmt.Sprintf(" %s/%s", formatKB(usedKB+cacheKB), formatKB(totalKB))
	barW := inner - lipgloss.Width(label) - lipgloss.Width(detail) - 2
	if barW < 10 {
		barW = 10
	}

	memBar := multiBar([]barSegment{
		{pct: usedPct, color: colMemUsed},
		{pct: cachePct, color: colMemCache},
	}, barW)

	return headerLabel.Render(label) +
		headerLabel.Render("[") + memBar + headerLabel.Render("]") +
		headerValue.Render(detail)
}

// buildVMSwapLine renders the per-VM swap line. With QGA: usage bar.
// Without QGA: a compact "—" placeholder.
func buildVMSwapLine(d lv.Domain, h *domHistory, info lv.SwapInfo, inner int) string {
	if info.Available {
		if !info.HasSwap {
			return headerLabel.Render("SWAP ") + headerValue.Render("disabled in guest")
		}
		usedPct := float64(info.UsedKB) / float64(info.TotalKB) * 100
		label := "SWAP "
		detail := fmt.Sprintf(" %s/%s", formatKB(info.UsedKB), formatKB(info.TotalKB))
		barW := inner - lipgloss.Width(label) - lipgloss.Width(detail) - 2
		if barW < 10 {
			barW = 10
		}
		swapBar := multiBar([]barSegment{
			{pct: usedPct, color: colSwap},
		}, barW)
		return headerLabel.Render(label) +
			headerLabel.Render("[") + swapBar + headerLabel.Render("]") +
			headerValue.Render(detail)
	}
	return headerLabel.Render("SWAP ") + headerValue.Render("(install qemu-guest-agent for usage)")
}

// buildVMDiskLine is the compact disk read/write line. The sparkline width
// is computed from the available inner box width so the line never wraps:
// the rolling history is sliced to whatever fits beside the labels and rates.
func buildVMDiskLine(h *domHistory, inner int) string {
	rdRate := formatRate(currentRate(h.blockRd))
	wrRate := formatRate(currentRate(h.blockWr))
	// Layout: "DISK r [spark] " + rdRate + "    w [spark] " + wrRate
	// Fixed cells (everything except the two sparklines):
	fixed := lipgloss.Width("DISK r  "+rdRate+"    w  "+wrRate) + 1
	sparkW := (inner - fixed) / 2
	if sparkW < 4 {
		sparkW = 4
	}
	if sparkW > historyWindow {
		sparkW = historyWindow
	}
	rdSpark := sparkline(tail(h.blockRd, sparkW))
	wrSpark := sparkline(tail(h.blockWr, sparkW))
	return headerLabel.Render("DISK ") +
		headerLabel.Render("r ") + headerValue.Render(rdSpark) + headerValue.Render(" "+rdRate) +
		headerLabel.Render("    w ") + headerValue.Render(wrSpark) + headerValue.Render(" "+wrRate)
}

// buildVMNetLine is the compact network rx/tx line. Width-aware like DISK.
func buildVMNetLine(h *domHistory, inner int) string {
	rxRate := formatRate(currentRate(h.netRx))
	txRate := formatRate(currentRate(h.netTx))
	fixed := lipgloss.Width("NET  ↓  "+rxRate+"    ↑  "+txRate) + 1
	sparkW := (inner - fixed) / 2
	if sparkW < 4 {
		sparkW = 4
	}
	if sparkW > historyWindow {
		sparkW = historyWindow
	}
	rxSpark := sparkline(tail(h.netRx, sparkW))
	txSpark := sparkline(tail(h.netTx, sparkW))
	return headerLabel.Render("NET  ") +
		headerLabel.Render("↓ ") + headerValue.Render(rxSpark) + headerValue.Render(" "+rxRate) +
		headerLabel.Render("    ↑ ") + headerValue.Render(txSpark) + headerValue.Render(" "+txRate)
}

// buildVMStorageLine is the at-a-glance storage summary: allocated/total
// disk usage, disk count, and current read/write IOPS. One line, narrow-friendly.
func buildVMStorageLine(d lv.Domain, h *domHistory) string {
	disksLabel := "1 disk"
	if d.NumDisks != 1 {
		disksLabel = fmt.Sprintf("%d disks", d.NumDisks)
	}
	usageLabel := "—"
	if d.TotalDiskCapacityBytes > 0 {
		usageLabel = fmt.Sprintf("%s/%s",
			formatBytes(float64(d.TotalDiskAllocationBytes)),
			formatBytes(float64(d.TotalDiskCapacityBytes)))
	}
	rdIops := currentRate(h.blockRdOps)
	wrIops := currentRate(h.blockWrOps)
	return headerLabel.Render("STORE ") +
		headerValue.Render(usageLabel) +
		headerLabel.Render("  ·  ") +
		headerValue.Render(disksLabel) +
		headerLabel.Render("  ·  iops r ") +
		headerValue.Render(fmt.Sprintf("%.0f", rdIops)) +
		headerLabel.Render(" / w ") +
		headerValue.Render(fmt.Sprintf("%.0f", wrIops))
}

// buildHostCPULine is the host CPU bar — only the bar and percent on this line.
func buildHostCPULine(s lv.HostStats, pct float64, inner int) string {
	return buildCPULine("CPU  ", pct, inner)
}

// buildHostMemLine renders the host memory bar (multi-segment) without
// the inline used/cache/free breakdown.
func buildHostMemLine(s lv.HostStats, inner int) string {
	if s.MemTotalKB == 0 {
		return headerLabel.Render("MEM  ") + headerValue.Render("(no stats)")
	}
	used := s.MemUsedKB()
	cache := s.MemCacheKB()
	usedPct := float64(used) / float64(s.MemTotalKB) * 100
	cachePct := float64(cache) / float64(s.MemTotalKB) * 100

	label := "MEM  "
	detail := fmt.Sprintf(" %s/%s", formatKB(used+cache), formatKB(s.MemTotalKB))
	barW := inner - lipgloss.Width(label) - lipgloss.Width(detail) - 2
	if barW < 10 {
		barW = 10
	}
	memBar := multiBar([]barSegment{
		{pct: usedPct, color: colMemUsed},
		{pct: cachePct, color: colMemCache},
	}, barW)
	return headerLabel.Render(label) +
		headerLabel.Render("[") + memBar + headerLabel.Render("]") +
		headerValue.Render(detail)
}

// buildHostSwapLine renders the host swap bar from /proc/meminfo data.
// Libvirt's node APIs do not expose swap, so for remote connections we
// show a placeholder instead of a bar.
func buildHostSwapLine(s lv.HostStats, inner int) string {
	if s.Remote {
		return headerLabel.Render("SWAP ") + headerValue.Render("—")
	}
	if s.SwapTotalKB == 0 {
		return headerLabel.Render("SWAP ") + headerValue.Render("disabled")
	}
	used := s.SwapUsedKB()
	usedPct := float64(used) / float64(s.SwapTotalKB) * 100

	label := "SWAP "
	detail := fmt.Sprintf(" %s/%s", formatKB(used), formatKB(s.SwapTotalKB))
	barW := inner - lipgloss.Width(label) - lipgloss.Width(detail) - 2
	if barW < 10 {
		barW = 10
	}
	swapBar := multiBar([]barSegment{
		{pct: usedPct, color: colSwap},
	}, barW)
	return headerLabel.Render(label) +
		headerLabel.Render("[") + swapBar + headerLabel.Render("]") +
		headerValue.Render(detail)
}

// buildHostLoadLine is a one-line load average display. Load average is
// not exposed by libvirt's node APIs, so remote connections show a
// placeholder instead.
func buildHostLoadLine(s lv.HostStats) string {
	if s.Remote {
		return headerLabel.Render("LOAD ") + headerValue.Render("—")
	}
	return headerLabel.Render("LOAD ") +
		headerValue.Render(fmt.Sprintf("%.2f  %.2f  %.2f", s.Load1, s.Load5, s.Load15)) +
		headerLabel.Render("    (1m  5m  15m)")
}

// buildHostDomsLine renders the domain count + overcommit ratios.
func buildHostDomsLine(m Model, total, running int, allocVCPU uint, allocMemKB uint64) string {
	line := headerLabel.Render("DOMS ") +
		headerValue.Render(fmt.Sprintf("%d", total)) +
		headerLabel.Render("  running ") +
		headerValue.Render(fmt.Sprintf("%d", running))

	if m.host.CPUs > 0 {
		ratio := float64(allocVCPU) / float64(m.host.CPUs)
		valStyle := headerValue
		if ratio > 1.0 {
			valStyle = lipgloss.NewStyle().Foreground(colPaused)
		}
		if ratio > 2.0 {
			valStyle = lipgloss.NewStyle().Foreground(colCrashed).Bold(true)
		}
		line += headerLabel.Render("  vCPU ") +
			valStyle.Render(fmt.Sprintf("%d/%d", allocVCPU, m.host.CPUs))
	}
	if m.host.MemoryKB > 0 {
		ratio := float64(allocMemKB) / float64(m.host.MemoryKB) * 100
		valStyle := headerValue
		if ratio >= 80 {
			valStyle = lipgloss.NewStyle().Foreground(colPaused)
		}
		if ratio >= 100 {
			valStyle = lipgloss.NewStyle().Foreground(colCrashed).Bold(true)
		}
		line += headerLabel.Render("  mem ") +
			valStyle.Render(fmt.Sprintf("%.0f%%", ratio))
	}
	return line
}

// ──────────────────────────── Helpers ────────────────────────────────────────

// formatPagesPerSec turns a pages/sec value into a short readable label.
// (Kept for any caller that still wants balloon swap activity rendering.)
func formatPagesPerSec(pps float64) string {
	if pps <= 0 {
		return "idle"
	}
	bps := pps * 4096
	return formatRate(bps)
}

// colorBar renders a horizontal bar coloured by fill percentage:
// green ≤ 60, yellow ≤ 85, red above.
func colorBar(pct float64, width int) string {
	b := bar(pct, width)
	style := lipgloss.NewStyle()
	switch {
	case pct >= 85:
		style = style.Foreground(colCrashed)
	case pct >= 60:
		style = style.Foreground(colPaused)
	default:
		style = style.Foreground(colRunning)
	}
	return style.Render(b)
}

// stateStyleFor returns the lipgloss style appropriate for a domain state.
func stateStyleFor(s lv.State) lipgloss.Style {
	switch s {
	case lv.StateRunning:
		return stateRunning
	case lv.StatePaused, lv.StatePMSuspended:
		return statePaused
	case lv.StateCrashed:
		return stateCrashed
	default:
		return stateShutoff
	}
}
