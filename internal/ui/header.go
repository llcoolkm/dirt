package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/llcoolkm/dirt/internal/lv"
)

// headerView renders the htop-style stats pane for the currently selected VM.
// When no VM is selected, or no snapshot has loaded yet, it shows a host summary.
func (m Model) headerView() string {
	width := m.contentWidth()
	if m.snap == nil {
		return headerBox.Width(width).Render("connecting to libvirt…")
	}

	d, ok := m.currentDomain()
	if !ok {
		return m.hostHeaderView(width)
	}

	switch d.State {
	case lv.StateRunning:
		return m.runningHeaderView(d, width)
	default:
		return m.idleHeaderView(d, width)
	}
}

// runningHeaderView is the full htop-style pane: CPU, MEM, SWAP, DISK, NET.
func (m Model) runningHeaderView(d lv.Domain, boxWidth int) string {
	h := m.history[d.UUID]
	if h == nil {
		h = &domHistory{}
	}

	// Compose title bar — name, state, vCPU, OS, uptime.
	title := headerTitle.Render(d.Name) +
		headerLabel.Render("  state ") + stateRunning.Render("running") +
		headerLabel.Render("  vCPU ") + headerValue.Render(fmt.Sprintf("%d", d.NrVCPU))
	if d.OS != "" {
		title += headerLabel.Render("  os ") + headerValue.Render(d.OS)
	}
	if up, accurate := effectiveUptime(d, h); up > 0 {
		label := formatDuration(up)
		if !accurate {
			label = "≥" + label
		}
		title += headerLabel.Render("  uptime ") + headerValue.Render(label)
	}

	// Inner content width — box has 1 char border + 1 char padding on each side.
	inner := boxWidth - 4
	if inner < 30 {
		inner = 30
	}

	// CPU bar — width is inner minus the label and value text.
	cpuPct := h.currentCPU()
	cpuLabel := "CPU  "
	cpuValue := fmt.Sprintf(" %5.1f%%", cpuPct)
	cpuBarW := inner - len(cpuLabel) - len(cpuValue) - 2 // [ ]
	if cpuBarW < 10 {
		cpuBarW = 10
	}
	cpuLine := headerLabel.Render(cpuLabel) +
		headerLabel.Render("[") +
		colorBar(cpuPct, cpuBarW) +
		headerLabel.Render("]") +
		headerValue.Render(cpuValue)

	// Memory bar — htop-style multi-segment when balloon stats are present.
	memLine := renderMemLine(d, inner)

	// Append major-fault sparkline if we have any history.
	if len(h.majorFault) > 0 {
		fr := currentRate(h.majorFault)
		var faultSuffix string
		if fr > 0 {
			faultSuffix = "  " + headerLabel.Render("fault ") +
				lipgloss.NewStyle().Foreground(colCrashed).Render(sparkline(h.majorFault)) +
				headerValue.Render(fmt.Sprintf(" %.0f/s", fr))
		} else {
			faultSuffix = "  " + headerLabel.Render("fault ") +
				lipgloss.NewStyle().Foreground(colDimmed).Render(sparkline(h.majorFault)) +
				headerLabel.Render(" idle")
		}
		memLine += faultSuffix
	}

	// Swap line — when QGA is installed in the guest, show usage bar; otherwise
	// fall back to activity sparklines from the balloon counters.
	swapInfo, hasSwap := m.swap[d.Name]
	swapLine := renderSwapLine(h, swapInfo, hasSwap, inner)

	// Disk line — sparkline + bytes/sec + IOPS + average latency.
	rdSpark := sparkline(h.blockRd)
	wrSpark := sparkline(h.blockWr)
	rdRate := formatRate(currentRate(h.blockRd))
	wrRate := formatRate(currentRate(h.blockWr))
	rdIops := currentRate(h.blockRdOps)
	wrIops := currentRate(h.blockWrOps)
	rdLat := formatLatencyUs(h.rdLatencyUs)
	wrLat := formatLatencyUs(h.wrLatencyUs)

	diskLine := headerLabel.Render("DISK ") +
		headerLabel.Render("r ") + headerValue.Render(rdSpark) +
		headerValue.Render(fmt.Sprintf(" %s %.0f iops %s", rdRate, rdIops, rdLat)) +
		headerLabel.Render("    w ") + headerValue.Render(wrSpark) +
		headerValue.Render(fmt.Sprintf(" %s %.0f iops %s", wrRate, wrIops, wrLat))

	// Net line — sparkline + bytes/sec + packets/sec + error counters.
	rxSpark := sparkline(h.netRx)
	txSpark := sparkline(h.netTx)
	rxRate := formatRate(currentRate(h.netRx))
	txRate := formatRate(currentRate(h.netTx))
	rxPps := currentRate(h.netRxPps)
	txPps := currentRate(h.netTxPps)

	netLine := headerLabel.Render("NET  ") +
		headerLabel.Render("↓ ") + headerValue.Render(rxSpark) +
		headerValue.Render(fmt.Sprintf(" %s %.0f pps", rxRate, rxPps)) +
		headerLabel.Render("    ↑ ") + headerValue.Render(txSpark) +
		headerValue.Render(fmt.Sprintf(" %s %.0f pps", txRate, txPps))

	// Append a red error count if any rx/tx errors or drops were observed.
	if errs := d.NetRxErrs + d.NetTxErrs + d.NetRxDrop + d.NetTxDrop; errs > 0 {
		netLine += "    " + errorStyle.Render(fmt.Sprintf("errs/drops %d", errs))
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		cpuLine,
		memLine,
		swapLine,
		diskLine,
		netLine,
	)
	return headerBox.Width(boxWidth).Render(body)
}

// renderMemLine builds the htop-style memory bar with used (green) and cache
// (yellow) segments. Falls back to a simple bar if balloon stats are absent.
func renderMemLine(d lv.Domain, inner int) string {
	totalKB := d.BalloonAvailableKB
	if totalKB == 0 {
		totalKB = d.MaxMemKB
	}
	if totalKB == 0 {
		return headerLabel.Render("MEM  ") + headerValue.Render("(no stats)")
	}

	var usedKB, cacheKB, freeKB uint64
	hasBalloon := d.BalloonAvailableKB > 0 && d.BalloonUnusedKB > 0
	if hasBalloon {
		freeKB = d.BalloonUnusedKB
		cacheKB = d.BalloonDiskCachesKB
		// used = total - free - cache, with underflow protection.
		if totalKB > freeKB+cacheKB {
			usedKB = totalKB - freeKB - cacheKB
		}
	} else {
		// No balloon — show allocated memory as "used".
		usedKB = d.MemoryKB
		if usedKB > totalKB {
			usedKB = totalKB
		}
	}

	usedPct := float64(usedKB) / float64(totalKB) * 100
	cachePct := float64(cacheKB) / float64(totalKB) * 100

	label := "MEM  "
	detail := fmt.Sprintf(" %s/%s", formatKB(usedKB+cacheKB), formatKB(totalKB))
	barW := inner - len(label) - len(detail) - 2 // [ ]
	if barW < 10 {
		barW = 10
	}

	memBar := multiBar([]barSegment{
		{pct: usedPct, color: colMemUsed},
		{pct: cachePct, color: colMemCache},
	}, barW)

	line := headerLabel.Render(label) +
		headerLabel.Render("[") + memBar + headerLabel.Render("]") +
		headerValue.Render(detail)

	// Append a small breakdown if balloon stats are available.
	if hasBalloon {
		breakdown := "  " +
			lipgloss.NewStyle().Foreground(colMemUsed).Render("■") + headerLabel.Render(" used ") + headerValue.Render(formatKB(usedKB)) +
			"  " +
			lipgloss.NewStyle().Foreground(colMemCache).Render("■") + headerLabel.Render(" cache ") + headerValue.Render(formatKB(cacheKB)) +
			"  " +
			lipgloss.NewStyle().Foreground(colDimmed).Render("□") + headerLabel.Render(" free ") + headerValue.Render(formatKB(freeKB))
		line += breakdown
	}
	return line
}

// renderSwapLine shows guest swap. When QGA reports usage, draw an htop-style
// bar (used / total). Otherwise fall back to activity sparklines from the
// balloon page-in/page-out counters, with a hint about installing QGA.
func renderSwapLine(h *domHistory, info lv.SwapInfo, have bool, inner int) string {
	if have && info.Available {
		if !info.HasSwap {
			return headerLabel.Render("SWAP ") + headerValue.Render("disabled in guest")
		}
		usedPct := float64(info.UsedKB) / float64(info.TotalKB) * 100
		label := "SWAP "
		detail := fmt.Sprintf(" %s/%s", formatKB(info.UsedKB), formatKB(info.TotalKB))
		barW := inner - len(label) - len(detail) - 2 // [ ]
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

	// QGA unavailable — show activity from balloon counters.
	inSpark := sparkline(h.swapIn)
	outSpark := sparkline(h.swapOut)
	inRate := formatPagesPerSec(currentRate(h.swapIn))
	outRate := formatPagesPerSec(currentRate(h.swapOut))
	hint := headerLabel.Render("  (install qemu-guest-agent for usage)")
	return headerLabel.Render("SWAP ") +
		headerLabel.Render("in ") + headerValue.Render(inSpark) + headerValue.Render(" "+inRate) +
		headerLabel.Render("   out ") + headerValue.Render(outSpark) + headerValue.Render(" "+outRate) +
		hint
}

// formatPagesPerSec turns a pages/sec value into a short readable label.
// Page size assumed 4 KiB (the qemu-balloon convention).
func formatPagesPerSec(pps float64) string {
	if pps <= 0 {
		return "idle"
	}
	bps := pps * 4096
	return formatRate(bps)
}

// idleHeaderView is shown for non-running VMs — just config, no live stats.
func (m Model) idleHeaderView(d lv.Domain, boxWidth int) string {
	stateStr := stateStyleFor(d.State).Render(d.State.String())

	title := headerTitle.Render(d.Name) +
		headerLabel.Render("  state ") + stateStr
	if d.OS != "" {
		title += headerLabel.Render("  os ") + headerValue.Render(d.OS)
	}

	autostart := "no"
	if d.Autostart {
		autostart = "yes"
	}
	persistent := "no"
	if d.Persistent {
		persistent = "yes"
	}

	line := headerLabel.Render("vCPUs:  ") + headerValue.Render(fmt.Sprintf("%d", d.NrVCPU)) +
		headerLabel.Render("    Max mem:  ") + headerValue.Render(formatKB(d.MaxMemKB)) +
		headerLabel.Render("    Persistent: ") + headerValue.Render(persistent) +
		headerLabel.Render("    Autostart: ") + headerValue.Render(autostart)

	body := lipgloss.JoinVertical(lipgloss.Left, title, "", line)
	return headerBox.Width(boxWidth).Render(body)
}

// hostHeaderView is the htop-style host pane shown when no VM is selected.
// It draws live host CPU, MEM, SWAP bars from /proc, plus load avg, domain
// counts, and overcommit ratios.
func (m Model) hostHeaderView(boxWidth int) string {
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

	// Title: hostname + URI + uptime.
	title := headerTitle.Render("host: "+m.snap.Hostname) +
		headerLabel.Render("  ("+m.snap.URI+")")
	if m.hostStats.UptimeSeconds > 0 {
		uptime := time.Duration(m.hostStats.UptimeSeconds * float64(time.Second))
		title += headerLabel.Render("  uptime ") + headerValue.Render(formatDuration(uptime))
	}

	inner := boxWidth - 4
	if inner < 30 {
		inner = 30
	}

	// CPU line: live host CPU% as a coloured bar + load average.
	loadStr := fmt.Sprintf("%.2f %.2f %.2f", m.hostStats.Load1, m.hostStats.Load5, m.hostStats.Load15)
	cpuValue := fmt.Sprintf(" %5.1f%%", m.hostCPUPct)
	cpuLoadSuffix := headerLabel.Render("    load ") + headerValue.Render(loadStr)
	cpuLabel := "CPU  "
	cpuBarW := inner - len(cpuLabel) - len(cpuValue) - 2 - len("    load ")*1 - len(loadStr)
	if cpuBarW < 10 {
		cpuBarW = 10
	}
	cpuLine := headerLabel.Render(cpuLabel) +
		headerLabel.Render("[") + colorBar(m.hostCPUPct, cpuBarW) + headerLabel.Render("]") +
		headerValue.Render(cpuValue) + cpuLoadSuffix

	// MEM line: htop-style multi-segment bar from /proc/meminfo.
	memLine := renderHostMemLine(m.hostStats, inner)

	// SWAP line: usage bar from /proc/meminfo.
	swapLine := renderHostSwapLine(m.hostStats, inner)

	// DOMS / overcommit line.
	domsLine := headerLabel.Render("DOMS: ") +
		headerValue.Render(fmt.Sprintf("%d", len(m.snap.Domains))) +
		headerLabel.Render("    running: ") +
		headerValue.Render(fmt.Sprintf("%d", running))
	if m.host.CPUs > 0 {
		ratio := float64(allocVCPU) / float64(m.host.CPUs)
		ratioStr := fmt.Sprintf("%d/%d (%.2f×)", allocVCPU, m.host.CPUs, ratio)
		valStyle := headerValue
		if ratio > 1.0 {
			valStyle = lipgloss.NewStyle().Foreground(colPaused)
		}
		if ratio > 2.0 {
			valStyle = lipgloss.NewStyle().Foreground(colCrashed).Bold(true)
		}
		domsLine += headerLabel.Render("    vCPU: ") + valStyle.Render(ratioStr)
	}
	if m.host.MemoryKB > 0 {
		ratio := float64(allocMemKB) / float64(m.host.MemoryKB) * 100
		ratioStr := fmt.Sprintf("%s/%s (%.0f%%)", formatKB(allocMemKB), formatKB(m.host.MemoryKB), ratio)
		valStyle := headerValue
		if ratio >= 80 {
			valStyle = lipgloss.NewStyle().Foreground(colPaused)
		}
		if ratio >= 100 {
			valStyle = lipgloss.NewStyle().Foreground(colCrashed).Bold(true)
		}
		domsLine += headerLabel.Render("    mem: ") + valStyle.Render(ratioStr)
	}

	// Footer line: CPU model.
	footer := ""
	if m.host.CPUs > 0 {
		footer = headerLabel.Render(m.host.CPUModel + " — " + fmt.Sprintf("%d cores", m.host.CPUs))
	}

	parts := []string{title, cpuLine, memLine, swapLine, domsLine}
	if footer != "" {
		parts = append(parts, footer)
	}
	return headerBox.Width(boxWidth).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

// renderHostMemLine builds the htop-style multi-segment host memory bar.
func renderHostMemLine(s lv.HostStats, inner int) string {
	if s.MemTotalKB == 0 {
		return headerLabel.Render("MEM  ") + headerValue.Render("(no /proc/meminfo)")
	}
	used := s.MemUsedKB()
	cache := s.MemCacheKB()
	usedPct := float64(used) / float64(s.MemTotalKB) * 100
	cachePct := float64(cache) / float64(s.MemTotalKB) * 100

	label := "MEM  "
	detail := fmt.Sprintf(" %s/%s", formatKB(used+cache), formatKB(s.MemTotalKB))
	barW := inner - len(label) - len(detail) - 2
	if barW < 10 {
		barW = 10
	}
	memBar := multiBar([]barSegment{
		{pct: usedPct, color: colMemUsed},
		{pct: cachePct, color: colMemCache},
	}, barW)
	line := headerLabel.Render(label) +
		headerLabel.Render("[") + memBar + headerLabel.Render("]") +
		headerValue.Render(detail)

	breakdown := "  " +
		lipgloss.NewStyle().Foreground(colMemUsed).Render("■") + headerLabel.Render(" used ") + headerValue.Render(formatKB(used)) +
		"  " +
		lipgloss.NewStyle().Foreground(colMemCache).Render("■") + headerLabel.Render(" cache ") + headerValue.Render(formatKB(cache)) +
		"  " +
		lipgloss.NewStyle().Foreground(colDimmed).Render("□") + headerLabel.Render(" free ") + headerValue.Render(formatKB(s.MemFreeKB))
	line += breakdown
	return line
}

// renderHostSwapLine builds the host swap bar from /proc/meminfo data.
func renderHostSwapLine(s lv.HostStats, inner int) string {
	if s.SwapTotalKB == 0 {
		return headerLabel.Render("SWAP ") + headerValue.Render("disabled")
	}
	used := s.SwapUsedKB()
	usedPct := float64(used) / float64(s.SwapTotalKB) * 100

	label := "SWAP "
	detail := fmt.Sprintf(" %s/%s", formatKB(used), formatKB(s.SwapTotalKB))
	barW := inner - len(label) - len(detail) - 2
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

// stripANSI is a tiny helper for measuring printable width when needed.
// (Currently unused — kept for future column padding.)
func stripANSI(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		if r == 0x1b {
			in = true
			continue
		}
		if in {
			if r == 'm' {
				in = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
