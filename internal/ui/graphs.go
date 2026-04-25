package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/NimbleMarkets/ntcharts/linechart/timeserieslinechart"
)

// Graph sub-tab indices.
const (
	graphTabCPU  = 0
	graphTabMEM  = 1
	graphTabDISK = 2
	graphTabNET  = 3
	graphTabMax  = 3
)

// Every chart uses the same height for a clean, consistent layout.
const chartHeight = 8

// Cap data points to keep chart rendering snappy.
const maxDataPoints = 120

var graphTabNames = []string{"CPU", "MEM", "DISK", "NET"}

// Colors: read/rx = green, write/tx = red.
var (
	graphStyleRead  = lipgloss.NewStyle().Foreground(colRunning) // green
	graphStyleWrite = lipgloss.NewStyle().Foreground(colCrashed) // red
)

// vcpuColors is the per-vCPU graph colour cycle. Populated by
// rebuildStyles on theme apply so it tracks the active palette.
var vcpuColors []lipgloss.Color

// chartAxisStyle and chartLabelStyle colour the X/Y axes and value
// labels of the ntcharts time-series charts. Populated by
// rebuildStyles, themed via colMuted.
var (
	chartAxisStyle  lipgloss.Style
	chartLabelStyle lipgloss.Style
)

// graphsView renders the performance graphs pane.
func (m Model) graphsView() string {
	width := m.contentWidth()

	d, ok := m.currentDomain()
	if !ok {
		pane := listBox.Width(width - borderWidth).Render(
			errorStyle.Render(" no VM selected"))
		bottom := statusBar.Width(width).Render(" " + key("esc") + " back")
		return lipgloss.JoinVertical(lipgloss.Left, pane, bottom)
	}

	h := m.history[d.UUID]
	if h == nil {
		h = &domHistory{}
	}

	// Title: VM name + sample window + tab bar.
	title := headerTitle.Render("performance: ") + headerValue.Render(d.Name)
	samples := len(h.cpu)
	if samples > 0 {
		secs := samples * int(m.refreshInterval.Seconds())
		if secs < 1 {
			secs = samples
		}
		title += headerLabel.Render(fmt.Sprintf("  ·  %d samples (%ds)", samples, secs))
	}
	titleRow := title + "    " + renderGraphTabs(m.graphsTab)

	body := m.graphsCache
	if body == "" {
		body = headerLabel.Render("  (waiting for data…)")
	}

	pane := listBox.Width(width - borderWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left, titleRow, "", body))

	bottom := statusBar.Width(width).Render(" " +
		key("1-4") + " tab  " +
		key("h/l") + " prev/next  " +
		key("esc") + " back")

	return lipgloss.JoinVertical(lipgloss.Left, pane, bottom)
}

func renderGraphTabs(active int) string {
	var parts []string
	for i, name := range graphTabNames {
		if i == active {
			parts = append(parts, headerTitle.Render("["+name+"]"))
		} else {
			parts = append(parts, headerLabel.Render(" "+name+" "))
		}
	}
	return strings.Join(parts, "")
}

// ────────────────────────── Tab renderers ──────────────────────────

func renderCPUTab(h *domHistory, w int, interval time.Duration) string {
	if len(h.cpu) == 0 {
		return headerLabel.Render("  (waiting for samples…)")
	}

	// 1. Aggregate CPU%.
	c1 := chartBlock("CPU %", fmt.Sprintf("%.1f%%", currentRate(h.cpu)),
		h.cpu, w, chartHeight, 0, 100, true,
		graphStyleRead, pctLabelFmt, interval)

	// 2. Per-vCPU breakdown.
	var c2 string
	if len(h.vcpuPct) > 0 {
		c2 = renderVCPUChart(h, w, interval)
	} else {
		c2 = headerTitle.Render("Per-vCPU") + "\n" +
			headerLabel.Render("  (waiting for per-vCPU data…)")
	}

	// 3. User vs System.
	var c3 string
	if len(h.cpuUser) > 0 || len(h.cpuSystem) > 0 {
		c3 = overlayBlockFixed("User / System",
			"user", colRunning, fmtPct(h.cpuUser),
			"system", colCrashed, fmtPct(h.cpuSystem),
			h.cpuUser, h.cpuSystem, w, chartHeight,
			0, 100, true,
			graphStyleRead, graphStyleWrite,
			pctLabelFmt, interval)
	} else {
		c3 = headerTitle.Render("User / System") + "\n" +
			headerLabel.Render("  (waiting for user/system data…)")
	}

	return lipgloss.JoinVertical(lipgloss.Left, c1, "", c2, "", c3)
}

func renderVCPUChart(h *domHistory, w int, interval time.Duration) string {
	n := len(h.vcpuPct)
	// Build title with per-vCPU current values.
	title := headerTitle.Render("Per-vCPU")
	for i := 0; i < n; i++ {
		col := vcpuColors[i%len(vcpuColors)]
		cur := "—"
		if len(h.vcpuPct[i]) > 0 {
			cur = fmt.Sprintf("%.0f%%", h.vcpuPct[i][len(h.vcpuPct[i])-1])
		}
		title += "  " + lipgloss.NewStyle().Foreground(col).Render(fmt.Sprintf("v%d %s", i, cur))
	}

	// Build multi-series chart.
	cap := w
	if cap > maxDataPoints {
		cap = maxDataPoints
	}

	// Find longest series for time range.
	maxLen := 0
	for i := 0; i < n; i++ {
		if l := len(h.vcpuPct[i]); l > maxLen {
			maxLen = l
		}
	}
	if maxLen == 0 {
		return title + "\n" + headerLabel.Render("  (no data)")
	}
	if maxLen > cap {
		maxLen = cap
	}

	now := time.Now()
	tMin := now.Add(-time.Duration(maxLen) * interval)

	chart := timeserieslinechart.New(w, chartHeight,
		timeserieslinechart.WithTimeRange(tMin, now),
		timeserieslinechart.WithXYSteps(5, 3),
		timeserieslinechart.WithXLabelFormatter(relativeXLabelFmt(now)),
		timeserieslinechart.WithYLabelFormatter(wrapLabelFmt(pctLabelFmt)),
		timeserieslinechart.WithAxesStyles(chartAxisStyle, chartLabelStyle),
	)

	for i := 0; i < n; i++ {
		name := fmt.Sprintf("v%d", i)
		col := vcpuColors[i%len(vcpuColors)]
		chart.SetDataSetStyle(name, lipgloss.NewStyle().Foreground(col))
		pushSeries(&chart, name, tail(h.vcpuPct[i], cap), now, interval)
	}

	// Set Y range AFTER pushing to override auto-adjustment.
	chart.SetYRange(0, 100)
	chart.SetViewYRange(0, 100)

	chart.DrawBrailleAll()
	return title + "\n" + chart.View()
}

func renderMEMTab(h *domHistory, w int, interval time.Duration) string {
	c1 := chartBlock("Used %", fmtPct(h.memUsedPct),
		h.memUsedPct, w, chartHeight, 0, 100, true,
		lipgloss.NewStyle().Foreground(colMemUsed), pctLabelFmt, interval)

	c2 := chartBlock("Cache %", fmtPct(h.memCachePct),
		h.memCachePct, w, chartHeight, 0, 100, true,
		lipgloss.NewStyle().Foreground(colMemCache), pctLabelFmt, interval)

	c3 := overlayBlock("Swap activity",
		"in", colSwap, fmtPPS(h.swapIn),
		"out", colPaused, fmtPPS(h.swapOut),
		h.swapIn, h.swapOut, w, chartHeight,
		lipgloss.NewStyle().Foreground(colSwap),
		lipgloss.NewStyle().Foreground(colPaused),
		autoLabelFmt, interval)

	var c4 string
	if len(h.swapUsedPct) > 0 {
		c4 = chartBlock("Swap used %", fmtPct(h.swapUsedPct),
			h.swapUsedPct, w, chartHeight, 0, 100, true,
			lipgloss.NewStyle().Foreground(colSwap), pctLabelFmt, interval)
	} else {
		c4 = headerTitle.Render("Swap used %") + "\n" +
			headerLabel.Render("  (needs qemu-guest-agent)")
	}

	return lipgloss.JoinVertical(lipgloss.Left, c1, "", c2, "", c3, "", c4)
}

func renderDISKTab(h *domHistory, w int, interval time.Duration) string {
	c1 := overlayBlock("Throughput",
		"read", colRunning, fmtRate(h.blockRd),
		"write", colCrashed, fmtRate(h.blockWr),
		h.blockRd, h.blockWr, w, chartHeight,
		graphStyleRead, graphStyleWrite,
		rateLabelFmt, interval)

	c2 := overlayBlock("IOPS",
		"read", colRunning, fmtFloat(h.blockRdOps),
		"write", colCrashed, fmtFloat(h.blockWrOps),
		h.blockRdOps, h.blockWrOps, w, chartHeight,
		graphStyleRead, graphStyleWrite,
		autoLabelFmt, interval)

	c3 := overlayBlock("Latency",
		"read", colRunning, fmtLatency(h.rdLatencyUs),
		"write", colCrashed, fmtLatency(h.wrLatencyUs),
		h.rdLatencyUs, h.wrLatencyUs, w, chartHeight,
		graphStyleRead, graphStyleWrite,
		latencyLabelFmt, interval)

	return lipgloss.JoinVertical(lipgloss.Left, c1, "", c2, "", c3)
}

func renderNETTab(h *domHistory, w int, interval time.Duration) string {
	c1 := overlayBlock("Speed",
		"rx", colRunning, fmtRate(h.netRx),
		"tx", colCrashed, fmtRate(h.netTx),
		h.netRx, h.netTx, w, chartHeight,
		graphStyleRead, graphStyleWrite,
		rateLabelFmt, interval)

	c2 := overlayBlock("Packets",
		"rx", colRunning, fmtPPS(h.netRxPps),
		"tx", colCrashed, fmtPPS(h.netTxPps),
		h.netRxPps, h.netTxPps, w, chartHeight,
		graphStyleRead, graphStyleWrite,
		autoLabelFmt, interval)

	return lipgloss.JoinVertical(lipgloss.Left, c1, "", c2)
}

// ────────────────────────── Side-by-side layout ──────────────────────────

// sideBySideRow describes one metric row: e.g. "Throughput" with read + write.
type sideBySideRow struct {
	metric   string            // e.g. "Throughput"
	labelA   string            // e.g. "read" or "rx"
	labelB   string            // e.g. "write" or "tx"
	dataA    []float64
	dataB    []float64
	labelFmt func(float64) string
}

// renderSideBySideTab renders rows of paired charts: left = series A
// (green), right = series B (red). Both charts in a row share the
// same Y-axis scale so they're visually comparable.
func renderSideBySideTab(w int, interval time.Duration, rows ...sideBySideRow) string {
	gap := 3
	colW := (w - gap) / 2
	spacer := strings.Repeat(" ", gap)
	col := lipgloss.NewStyle().Width(colW)

	var parts []string
	for i, r := range rows {
		if i > 0 {
			parts = append(parts, "")
		}

		// Shared Y max across both sides so the scales match.
		mxA := sliceMax(r.dataA)
		mxB := sliceMax(r.dataB)
		mx := mxA
		if mxB > mx {
			mx = mxB
		}
		if mx <= 0 {
			mx = 1
		}
		yMax := mx * 1.1

		fmtA := fmtAuto(r.dataA, r.labelFmt)
		fmtB := fmtAuto(r.dataB, r.labelFmt)

		left := chartBlockFixed(r.metric+" "+r.labelA, fmtA,
			r.dataA, colW, chartHeight, 0, yMax,
			graphStyleRead, r.labelFmt, interval)

		right := chartBlockFixed(r.metric+" "+r.labelB, fmtB,
			r.dataB, colW, chartHeight, 0, yMax,
			graphStyleWrite, r.labelFmt, interval)

		parts = append(parts,
			lipgloss.JoinHorizontal(lipgloss.Top, col.Render(left), spacer, col.Render(right)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// chartBlockFixed renders a titled chart with a fixed Y range (always sets view range).
func chartBlockFixed(name, current string, data []float64,
	w, h int, yMin, yMax float64,
	style lipgloss.Style, labelFmt func(float64) string,
	interval time.Duration) string {

	title := headerTitle.Render(name) + "  " + headerValue.Render(current)
	chart := buildChart(data, w, h, yMin, yMax, true, style, labelFmt, interval)
	return title + "\n" + chart
}

// fmtAuto formats the latest value using the row's label formatter.
func fmtAuto(data []float64, fn func(float64) string) string {
	if len(data) == 0 {
		return "—"
	}
	return fn(data[len(data)-1])
}

// ────────────────────────── Block builders ──────────────────────────

func chartBlock(name, current string, data []float64,
	w, h int, yMin, yMax float64, fixedY bool,
	style lipgloss.Style, labelFmt func(float64) string,
	interval time.Duration) string {

	title := headerTitle.Render(name) + "  " + headerValue.Render(current)
	chart := buildChart(data, w, h, yMin, yMax, fixedY, style, labelFmt, interval)
	return title + "\n" + chart
}

func overlayBlock(name, labelA string, colA lipgloss.TerminalColor, curA,
	labelB string, colB lipgloss.TerminalColor, curB string,
	dataA, dataB []float64, w, h int,
	styleA, styleB lipgloss.Style,
	labelFmt func(float64) string,
	interval time.Duration) string {

	return overlayBlockFixed(name, labelA, colA, curA, labelB, colB, curB,
		dataA, dataB, w, h, 0, 0, false, styleA, styleB, labelFmt, interval)
}

func overlayBlockFixed(name, labelA string, colA lipgloss.TerminalColor, curA,
	labelB string, colB lipgloss.TerminalColor, curB string,
	dataA, dataB []float64, w, h int,
	yMin, yMax float64, fixedY bool,
	styleA, styleB lipgloss.Style,
	labelFmt func(float64) string,
	interval time.Duration) string {

	title := headerTitle.Render(name) + "  " +
		lipgloss.NewStyle().Foreground(colA).Render(labelA+" "+curA) + "  " +
		lipgloss.NewStyle().Foreground(colB).Render(labelB+" "+curB)
	chart := buildOverlayChart(dataA, dataB, w, h, yMin, yMax, fixedY, styleA, styleB, labelFmt, interval)
	return title + "\n" + chart
}

// ────────────────────────── Chart builders ──────────────────────────

func buildChart(data []float64, w, h int, yMin, yMax float64, fixedY bool,
	style lipgloss.Style, labelFmt func(float64) string,
	interval time.Duration) string {

	cap := w
	if cap > maxDataPoints {
		cap = maxDataPoints
	}
	samples := tail(data, cap)
	if len(samples) == 0 {
		return headerLabel.Render("  (no data)")
	}

	now := time.Now()
	n := len(samples)
	tMin := now.Add(-time.Duration(n) * interval)

	opts := []timeserieslinechart.Option{
		timeserieslinechart.WithStyle(style),
		timeserieslinechart.WithTimeRange(tMin, now),
		timeserieslinechart.WithXYSteps(5, 3),
		timeserieslinechart.WithXLabelFormatter(relativeXLabelFmt(now)),
		timeserieslinechart.WithAxesStyles(chartAxisStyle, chartLabelStyle),
	}
	if fixedY {
		opts = append(opts, timeserieslinechart.WithYRange(yMin, yMax))
	}
	if labelFmt != nil {
		opts = append(opts,
			timeserieslinechart.WithYLabelFormatter(wrapLabelFmt(labelFmt)))
	}

	chart := timeserieslinechart.New(w, h, opts...)

	pushSeries(&chart, "", samples, now, interval)

	// Set Y range AFTER pushing data so it overrides any auto-adjustment
	// ntcharts made during Push (auto-range is on by default).
	if fixedY {
		chart.SetYRange(yMin, yMax)
		chart.SetViewYRange(yMin, yMax)
	} else {
		mx := sliceMax(samples)
		if mx <= 0 {
			mx = 1
		}
		yMax := mx * 1.1
		chart.SetYRange(0, yMax)
		chart.SetViewYRange(0, yMax)
	}

	chart.DrawBraille()
	return chart.View()
}

func buildOverlayChart(dataA, dataB []float64, w, h int,
	yMin, yMax float64, fixedY bool,
	styleA, styleB lipgloss.Style,
	labelFmt func(float64) string,
	interval time.Duration) string {

	cap := w
	if cap > maxDataPoints {
		cap = maxDataPoints
	}
	samplesA := tail(dataA, cap)
	samplesB := tail(dataB, cap)
	if len(samplesA) == 0 && len(samplesB) == 0 {
		return headerLabel.Render("  (no data)")
	}

	n := len(samplesA)
	if len(samplesB) > n {
		n = len(samplesB)
	}
	now := time.Now()
	tMin := now.Add(-time.Duration(n) * interval)

	opts := []timeserieslinechart.Option{
		timeserieslinechart.WithTimeRange(tMin, now),
		timeserieslinechart.WithXYSteps(5, 3),
		timeserieslinechart.WithXLabelFormatter(relativeXLabelFmt(now)),
		timeserieslinechart.WithAxesStyles(chartAxisStyle, chartLabelStyle),
	}
	if labelFmt != nil {
		opts = append(opts,
			timeserieslinechart.WithYLabelFormatter(wrapLabelFmt(labelFmt)))
	}

	chart := timeserieslinechart.New(w, h, opts...)

	chart.SetStyle(styleA)
	chart.SetDataSetStyle("b", styleB)
	pushSeries(&chart, "", samplesA, now, interval)
	pushSeries(&chart, "b", samplesB, now, interval)

	// Set Y range AFTER pushing data to override auto-adjustment.
	if fixedY {
		chart.SetYRange(yMin, yMax)
		chart.SetViewYRange(yMin, yMax)
	} else {
		mx := sliceMax(samplesA)
		if m2 := sliceMax(samplesB); m2 > mx {
			mx = m2
		}
		if mx <= 0 {
			mx = 1
		}
		autoMax := mx * 1.1
		chart.SetYRange(0, autoMax)
		chart.SetViewYRange(0, autoMax)
	}

	chart.DrawBrailleAll()
	return chart.View()
}

func pushSeries(chart *timeserieslinechart.Model, name string,
	data []float64, now time.Time, interval time.Duration) {
	n := len(data)
	for i, v := range data {
		tp := timeserieslinechart.TimePoint{
			Time:  now.Add(-time.Duration(n-1-i) * interval),
			Value: v,
		}
		if name == "" {
			chart.Push(tp)
		} else {
			chart.PushDataSet(name, tp)
		}
	}
}

// ────────────────────────── Label formatters ──────────────────────────

func wrapLabelFmt(fn func(float64) string) func(int, float64) string {
	return func(_ int, v float64) string { return fn(v) }
}

// relativeXLabelFmt returns a formatter that shows time relative to `ref`,
// e.g. "-5m", "-3m", "-1m", "0m". Suitable for a rolling 5-minute window.
func relativeXLabelFmt(ref time.Time) func(int, float64) string {
	return func(_ int, v float64) string {
		t := time.Unix(int64(v), 0)
		d := t.Sub(ref)
		secs := int(d.Seconds())
		if secs >= 0 {
			return "now"
		}
		mins := -secs / 60
		remSecs := (-secs) % 60
		if mins > 0 && remSecs >= 15 {
			return fmt.Sprintf("-%dm%ds", mins, remSecs)
		}
		if mins > 0 {
			return fmt.Sprintf("-%dm", mins)
		}
		return fmt.Sprintf("-%ds", -secs)
	}
}

// yLabelWidth is the fixed width for all Y-axis labels. Every formatter
// right-pads to this width so the chart area starts at the same column
// regardless of the magnitude of the values displayed.
const yLabelWidth = 8

func padLabel(s string) string {
	for len(s) < yLabelWidth {
		s = " " + s
	}
	return s
}

func pctLabelFmt(v float64) string    { return padLabel(fmt.Sprintf("%.0f%%", v)) }
func rateLabelFmt(v float64) string   { return padLabel(formatRate(v)) }
func latencyLabelFmt(v float64) string {
	if v < 1000 {
		return padLabel(fmt.Sprintf("%.0fµs", v))
	}
	return padLabel(fmt.Sprintf("%.1fms", v/1000))
}
func autoLabelFmt(v float64) string {
	if v >= 1000000 {
		return padLabel(fmt.Sprintf("%.1fM", v/1000000))
	}
	if v >= 1000 {
		return padLabel(fmt.Sprintf("%.1fK", v/1000))
	}
	return padLabel(fmt.Sprintf("%.0f", v))
}

// ────────────────────────── Value formatters ──────────────────────────

func fmtPct(s []float64) string {
	if len(s) == 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", s[len(s)-1])
}

func fmtRate(s []float64) string {
	if len(s) == 0 {
		return "—"
	}
	return formatRate(s[len(s)-1])
}

func fmtFloat(s []float64) string {
	if len(s) == 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f", s[len(s)-1])
}

func fmtPPS(s []float64) string {
	if len(s) == 0 {
		return "—"
	}
	v := s[len(s)-1]
	if v >= 1000 {
		return fmt.Sprintf("%.1fK/s", v/1000)
	}
	return fmt.Sprintf("%.0f/s", v)
}

func fmtLatency(s []float64) string {
	if len(s) == 0 {
		return "—"
	}
	return latencyLabelFmt(s[len(s)-1])
}

func sliceMax(s []float64) float64 {
	mx := 0.0
	for _, v := range s {
		if v > mx {
			mx = v
		}
	}
	return mx
}

// ────────────────────────── Key handler ──────────────────────────

func (m Model) handleGraphsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?":
		m.prevMode = m.mode
		m.mode = viewHelp
		return m, nil
	case "esc", "q":
		m.mode = viewMain
		return m, nil
	case "1":
		m.graphsTab = graphTabCPU
		m.graphsDirty = true
	case "2":
		m.graphsTab = graphTabMEM
		m.graphsDirty = true
	case "3":
		m.graphsTab = graphTabDISK
		m.graphsDirty = true
	case "4":
		m.graphsTab = graphTabNET
		m.graphsDirty = true
	case "h", "left":
		if m.graphsTab <= 0 {
			m.graphsTab = graphTabMax
		} else {
			m.graphsTab--
		}
		m.graphsDirty = true
	case "l", "right":
		if m.graphsTab >= graphTabMax {
			m.graphsTab = 0
		} else {
			m.graphsTab++
		}
		m.graphsDirty = true
	}
	return m, nil
}
