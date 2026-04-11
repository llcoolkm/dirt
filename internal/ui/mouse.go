package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleMouse routes a tea.MouseMsg to the currently-active view.
// Ignores motion/release events and any clicks while a text input is
// active (filter, command palette, snapshot name, host input, detail
// search) — otherwise the user's typing would jump to the mouse.
func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.isTextInputting() {
		return m, nil
	}

	// Wheel events do not have a press/release action — they always
	// arrive as a single "press". We treat them as selection nav and
	// ignore their Y coordinate so the cursor behaves like j/k.
	if msg.Button == tea.MouseButtonWheelUp {
		return m.mouseWheel(-1)
	}
	if msg.Button == tea.MouseButtonWheelDown {
		return m.mouseWheel(+1)
	}

	// For real button events we only react to the press to avoid
	// double-firing on release and to ignore the motion stream that
	// tea.WithMouseCellMotion() delivers while the cursor moves.
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	if msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	switch m.mode {
	case viewMain:
		return m.mouseClickMain(msg.Y)
	case viewHosts:
		if idx, ok := clickedSubviewRow(msg.Y, len(m.hosts)); ok {
			m.hostsSel = idx
		}
	case viewNetworks:
		if idx, ok := clickedSubviewRow(msg.Y, len(m.networks)); ok {
			m.networksSel = idx
		}
	case viewPools:
		if idx, ok := clickedSubviewRow(msg.Y, len(m.pools)); ok {
			m.poolsSel = idx
		}
	case viewVolumes:
		if idx, ok := clickedSubviewRow(msg.Y, len(m.volumes)); ok {
			m.volumesSel = idx
		}
	case viewSnapshots:
		if idx, ok := clickedSubviewRow(msg.Y, len(m.snapshots)); ok {
			m.snapshotsSel = idx
		}
	}
	return m, nil
}

// mouseWheel handles wheel up/down events across every list-like view.
// dir is -1 for wheel up, +1 for wheel down.
func (m Model) mouseWheel(dir int) (tea.Model, tea.Cmd) {
	switch m.mode {
	case viewMain:
		doms := m.visibleDomains()
		next := m.selected + dir
		if next < 0 || next >= len(doms) {
			return m, nil
		}
		m.selected = next
	case viewHosts:
		next := m.hostsSel + dir
		if next < 0 || next >= len(m.hosts) {
			return m, nil
		}
		m.hostsSel = next
	case viewNetworks:
		next := m.networksSel + dir
		if next < 0 || next >= len(m.networks) {
			return m, nil
		}
		m.networksSel = next
	case viewPools:
		next := m.poolsSel + dir
		if next < 0 || next >= len(m.pools) {
			return m, nil
		}
		m.poolsSel = next
	case viewVolumes:
		next := m.volumesSel + dir
		if next < 0 || next >= len(m.volumes) {
			return m, nil
		}
		m.volumesSel = next
	case viewSnapshots:
		next := m.snapshotsSel + dir
		if next < 0 || next >= len(m.snapshots) {
			return m, nil
		}
		m.snapshotsSel = next
	case viewDetail:
		// Wheel in the XML detail view scrolls the body, one line per tick.
		m.detailScroll += dir
		if m.detailScroll < 0 {
			m.detailScroll = 0
		}
	}
	return m, nil
}

// mouseClickMain handles a left-click in the main VM list. The layout
// is: header pane (9 rows wide / 18 rows stacked) + list box top border
// + column-header row + data rows. A click below the list bottom border
// is ignored.
func (m Model) mouseClickMain(y int) (tea.Model, tea.Cmd) {
	doms := m.visibleDomains()
	if len(doms) == 0 {
		return m, nil
	}
	first := m.headerPaneHeight() + 2 // list top border + column-header row
	idx := y - first + m.offset
	if idx < 0 || idx >= len(doms) {
		return m, nil
	}
	m.selected = idx
	return m, nil
}

// clickedSubviewRow maps a terminal Y coordinate to a row index in the
// subview tables (hosts, networks, pools, snapshots, volumes). Their
// layout is: top border + title + blank + column-header row + data
// rows, so the first data row is at Y=4. Returns ok=false when the
// click fell outside the data area.
func clickedSubviewRow(y, n int) (int, bool) {
	if n == 0 {
		return 0, false
	}
	const first = 4
	idx := y - first
	if idx < 0 || idx >= n {
		return 0, false
	}
	return idx, true
}

// headerPaneHeight returns the total number of terminal rows taken by
// the header pane (host + per-VM boxes) at the current window width.
// Must agree with headerView's layout choices in header.go.
func (m Model) headerPaneHeight() int {
	// Each box is 1 (top border) + 7 (content rows) + 1 (bottom border) = 9.
	const boxHeight = 9
	if m.contentWidth() < sideBySideMinWidth {
		return 2 * boxHeight // stacked: host on top, VM below
	}
	return boxHeight // side-by-side: one box's height
}
