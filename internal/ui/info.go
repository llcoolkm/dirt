package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/llcoolkm/dirt/internal/lv"
)

// infoLoadedMsg carries the result of loading a VM's structured info.
type infoLoadedMsg struct {
	name string
	info lv.DomainInfo
	err  error
}

// loadInfoCmd fetches and parses the domain XML for one VM off the
// UI thread. Mirrors loadDetailCmd but returns the structured info
// instead of the raw XML body.
func loadInfoCmd(c *lv.Client, name string) tea.Cmd {
	return func() tea.Msg {
		info, err := c.DomainInfo(name)
		return infoLoadedMsg{name: name, info: info, err: err}
	}
}

// infoView renders the per-VM info pane. One scrollable box that
// shows identity, hardware, boot, disks, NICs, graphics, and live
// numbers pulled from the current snapshot.
func (m Model) infoView() string {
	width := m.contentWidth()

	if m.infoErr != nil {
		return listBox.Width(width - borderWidth).Render(
			errorStyle.Render(" error loading info: "+m.infoErr.Error()),
		) + "\n" +
			statusBar.Width(width).Render(" " + key("esc") + " back")
	}

	lines := m.renderInfoBody(m.info)

	// Trim to the viewport height and honour scroll.
	bodyH := m.infoBodyHeight()
	total := len(lines)
	if m.infoScroll < 0 {
		m.infoScroll = 0
	}
	if m.infoScroll > total-bodyH && total > bodyH {
		m.infoScroll = total - bodyH
	}
	end := m.infoScroll + bodyH
	if end > total {
		end = total
	}
	visible := lines
	if total > bodyH {
		visible = lines[m.infoScroll:end]
	}
	// Pad out to keep the box a consistent height.
	for len(visible) < bodyH {
		visible = append(visible, "")
	}

	title := headerTitle.Render("info: ") + headerValue.Render(m.infoFor)
	if total > bodyH {
		title += headerLabel.Render(fmt.Sprintf("  ·  line %d-%d / %d",
			m.infoScroll+1, end, total))
	}

	pane := listBox.Width(width - borderWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			append([]string{title, ""}, visible...)...),
	)

	bottom := statusBar.Width(width).Render(" " +
		key("j/k") + " scroll  " + key("g/G") + " top/bottom  " +
		key("e") + " edit  " + key("p") + " perf  " +
		key("x") + " xml  " + key("esc") + " back")

	return lipgloss.JoinVertical(lipgloss.Left, pane, bottom)
}

// renderInfoBody produces the list of lines for the info pane. Kept
// separate from infoView so tests (or a future search feature) can
// work over the rendered text without scroll math in the way.
func (m Model) renderInfoBody(info lv.DomainInfo) []string {
	var lines []string

	// Short helpers for a label-value pair and a section heading.
	label := func(s string) string { return headerLabel.Render(padRight(s, 14)) }
	value := func(s string) string { return headerValue.Render(s) }
	section := func(s string) string { return headerTitle.Render(s) }
	row := func(k, v string) string {
		if v == "" {
			v = "—"
		}
		return "  " + label(k) + value(v)
	}

	// Look up live data from the current snapshot, if we have one.
	var live *lv.Domain
	if m.snap != nil {
		for i := range m.snap.Domains {
			if m.snap.Domains[i].Name == info.Name {
				live = &m.snap.Domains[i]
				break
			}
		}
	}

	// ── Identity ─────────────────────────────────────────────────
	lines = append(lines, section("identity"))
	lines = append(lines, row("name", info.Name))
	lines = append(lines, row("uuid", info.UUID))
	if info.Title != "" {
		lines = append(lines, row("title", info.Title))
	}
	if info.Description != "" {
		lines = append(lines, row("description", info.Description))
	}
	if live != nil {
		stateStr := stateStyleFor(live.State).Render(live.State.String())
		lines = append(lines, "  "+label("state")+stateStr)
		lines = append(lines, row("os", live.OS))
		if live.IP != "" {
			lines = append(lines, row("ip", live.IP))
		}
		auto := "no"
		if live.Autostart {
			auto = "yes"
		}
		pers := "no"
		if live.Persistent {
			pers = "yes"
		}
		lines = append(lines, row("autostart", auto))
		lines = append(lines, row("persistent", pers))
	}
	lines = append(lines, "")

	// ── Hardware ─────────────────────────────────────────────────
	lines = append(lines, section("hardware"))
	vcpuStr := fmt.Sprintf("%d", info.VCPUs)
	if info.MaxVCPUs > info.VCPUs && info.MaxVCPUs > 0 {
		vcpuStr = fmt.Sprintf("%d (max %d)", info.VCPUs, info.MaxVCPUs)
	}
	lines = append(lines, row("vcpus", vcpuStr))
	if info.CPUMode != "" {
		cpuStr := info.CPUMode
		if info.CPUModel != "" {
			cpuStr += " / " + info.CPUModel
		}
		lines = append(lines, row("cpu mode", cpuStr))
	}
	if live != nil {
		h := m.history[live.UUID]
		if h != nil {
			lines = append(lines, row("cpu %", fmt.Sprintf("%.1f%%", h.currentCPU())))
		}
	}
	memStr := formatKB(info.MemoryKB)
	if info.MaxMemKB > info.MemoryKB && info.MaxMemKB > 0 {
		memStr = fmt.Sprintf("%s (max %s)", formatKB(info.MemoryKB), formatKB(info.MaxMemKB))
	}
	lines = append(lines, row("memory", memStr))
	if live != nil && live.BalloonAvailableKB > 0 {
		used := live.BalloonAvailableKB - live.BalloonUnusedKB - live.BalloonDiskCachesKB
		lines = append(lines, row("balloon used", formatKB(used)))
		lines = append(lines, row("balloon free", formatKB(live.BalloonUnusedKB)))
		lines = append(lines, row("balloon cache", formatKB(live.BalloonDiskCachesKB)))
	}
	lines = append(lines, "")

	// ── Boot ─────────────────────────────────────────────────────
	lines = append(lines, section("boot"))
	lines = append(lines, row("firmware", info.Firmware))
	if len(info.BootOrder) > 0 {
		lines = append(lines, row("boot order", strings.Join(info.BootOrder, ", ")))
	}
	osType := info.OSType
	if info.OSArch != "" {
		osType = osType + " / " + info.OSArch
	}
	if info.Machine != "" {
		osType = osType + " / " + info.Machine
	}
	lines = append(lines, row("os type", osType))
	lines = append(lines, "")

	// ── Disks ────────────────────────────────────────────────────
	lines = append(lines, section(fmt.Sprintf("disks (%d)", len(info.Disks))))
	for _, disk := range info.Disks {
		attrs := []string{disk.Device}
		if disk.Bus != "" {
			attrs = append(attrs, "bus="+disk.Bus)
		}
		if disk.DriverType != "" {
			attrs = append(attrs, disk.DriverType)
		}
		if disk.ReadOnly {
			attrs = append(attrs, "read-only")
		}
		if disk.Shareable {
			attrs = append(attrs, "shareable")
		}
		head := strings.Join(attrs, " · ")
		// Append live I/O counters if available.
		if live != nil {
			if ds, ok := live.DiskStats[disk.Target]; ok {
				head += fmt.Sprintf("  ·  r %s / w %s  ·  iops r %d / w %d",
					formatBytes(float64(ds.RdBytes)),
					formatBytes(float64(ds.WrBytes)),
					ds.RdReqs, ds.WrReqs)
			}
		}
		lines = append(lines, "  "+label(disk.Target)+value(head))
		if disk.Source != "" {
			lines = append(lines, "  "+label("")+headerLabel.Render(disk.Source))
		}
	}
	if live != nil && live.TotalDiskCapacityBytes > 0 {
		lines = append(lines, "")
		lines = append(lines, "  "+label("total alloc")+value(
			fmt.Sprintf("%s / %s",
				formatBytes(float64(live.TotalDiskAllocationBytes)),
				formatBytes(float64(live.TotalDiskCapacityBytes)))))
	}
	lines = append(lines, "")

	// ── NICs ─────────────────────────────────────────────────────
	lines = append(lines, section(fmt.Sprintf("network interfaces (%d)", len(info.NICs))))
	// The MAC is a full-width token (17 chars), wider than the normal
	// label column, so we give NICs a dedicated 19-char pad instead of
	// the default 14 so the attrs column always starts aligned.
	macLabel := func(s string) string { return headerValue.Render(padRight(s, 19)) }
	for _, nic := range info.NICs {
		attrs := []string{}
		if nic.Model != "" {
			attrs = append(attrs, nic.Model)
		}
		if nic.SourceType != "" && nic.Source != "" {
			attrs = append(attrs, fmt.Sprintf("%s=%s", nic.SourceType, nic.Source))
		}
		if nic.Target != "" {
			attrs = append(attrs, "tap="+nic.Target)
		}
		line := "  " + macLabel(nic.MAC) + headerLabel.Render(strings.Join(attrs, " · "))
		// Append live NIC counters if available (keyed by tap device name).
		if live != nil && nic.Target != "" {
			if ns, ok := live.NICStats[nic.Target]; ok {
				line += headerLabel.Render("  ·  ") + headerValue.Render(
					fmt.Sprintf("rx %s / tx %s  ·  errs %d  drops %d",
						formatBytes(float64(ns.RxBytes)),
						formatBytes(float64(ns.TxBytes)),
						ns.RxErrs+ns.TxErrs,
						ns.RxDrop+ns.TxDrop))
			}
		}
		lines = append(lines, line)
	}
	lines = append(lines, "")

	// ── Graphics ─────────────────────────────────────────────────
	if len(info.Graphics) > 0 {
		lines = append(lines, section("graphics"))
		for _, g := range info.Graphics {
			portStr := "auto"
			if g.Port > 0 {
				portStr = fmt.Sprintf("%d", g.Port)
			}
			listen := g.Listen
			if listen == "" {
				listen = "—"
			}
			lines = append(lines, "  "+label(g.Type)+value(
				fmt.Sprintf("listen %s · port %s", listen, portStr)))
		}
		lines = append(lines, "")
	}

	return lines
}

// infoBodyHeight returns how many rows the info pane's scrollable
// body can occupy. Mirrors detailBodyHeight's arithmetic but tuned
// for the info layout (list box + status bar chrome).
func (m Model) infoBodyHeight() int {
	if m.height == 0 {
		return 20
	}
	// listBox: 2 border rows + title row + blank row = 4 chrome.
	// statusBar: 1 row. Safety margin: 1.
	h := m.height - 4 - 1 - 1
	if h < 4 {
		h = 4
	}
	return h
}

// handleInfoKey handles keys while in the info view.
func (m Model) handleInfoKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?":
		m.prevMode = m.mode
		m.mode = viewHelp
		return m, nil
	case "esc", "q":
		m.mode = viewMain
		return m, nil
	case "j", "down":
		m.infoScroll++
		return m, nil
	case "k", "up":
		if m.infoScroll > 0 {
			m.infoScroll--
		}
		return m, nil
	case "g", "home":
		m.infoScroll = 0
		return m, nil
	case "G", "end":
		m.infoScroll = 1 << 30 // clamped on next render
		return m, nil
	case "pgdown", "ctrl+d":
		m.infoScroll += m.infoBodyHeight() / 2
		return m, nil
	case "pgup", "ctrl+u":
		m.infoScroll -= m.infoBodyHeight() / 2
		if m.infoScroll < 0 {
			m.infoScroll = 0
		}
		return m, nil
	case "x":
		// Jump from info view directly to raw XML on the same VM.
		name := m.infoFor
		m.mode = viewDetail
		m.detailFor = name
		m.detailXML = "(loading…)"
		m.detailLines = []string{m.detailXML}
		return m, loadDetailCmd(m.client, name)
	case "e":
		// Edit the underlying XML in $EDITOR via `virsh edit`. Tea
		// suspends while the editor runs and resumes on exit; the
		// actionResultMsg handler reloads the info pane afterward,
		// so any changes are visible the moment we return.
		name := m.infoFor
		if name == "" {
			return m, nil
		}
		return m, m.runEdit(name)
	case "p", "P":
		// Jump to performance graphs for this VM.
		m.mode = viewGraphs
		return m, nil
	}
	return m, nil
}
