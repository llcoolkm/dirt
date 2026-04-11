package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/llcoolkm/dirt/internal/lv"
)

// Column widths for the snapshot table. NAME is wider now that it also
// carries the tree prefix; the old separate PARENT column is gone since
// the relationship is implicit in the indentation.
const (
	snapNameW    = 42
	snapStateW   = 10
	snapSizeW    = 8
	snapCurrentW = 8
	snapWhenW    = 19
)

// sortSnapshotsAsTree rearranges snaps into parent-first DFS order with
// siblings ordered by creation time ascending, and returns a parallel
// slice of tree-prefix strings that visualise the parent/child
// relationship. Root snapshots (Parent == "") get an empty prefix;
// descendants get glyphs like "├─ " and "└─ ", with deeper levels
// padded by "│  " (when an ancestor still has later siblings) or "   "
// (when it does not).
func sortSnapshotsAsTree(snaps []lv.DomainSnapshot) ([]lv.DomainSnapshot, []string) {
	if len(snaps) == 0 {
		return nil, nil
	}

	// Group children by parent name.
	children := make(map[string][]lv.DomainSnapshot, len(snaps))
	for _, s := range snaps {
		children[s.Parent] = append(children[s.Parent], s)
	}
	for k := range children {
		kids := children[k]
		sort.Slice(kids, func(i, j int) bool {
			return kids[i].CreatedAt.Before(kids[j].CreatedAt)
		})
	}

	out := make([]lv.DomainSnapshot, 0, len(snaps))
	prefixes := make([]string, 0, len(snaps))

	// walk performs a DFS. ancestorsHaveMore tracks the have-more
	// state of every visible ancestor STRICTLY ABOVE the current node's
	// parent, so the number of pad cells equals the node's visible
	// depth minus 1. When recursing from the phantom root into a root
	// snapshot, we pass nil (not append(..., !isLast)) so the root's
	// non-existent column does not contribute padding to descendants.
	var walk func(parent string, ancestorsHaveMore []bool)
	walk = func(parent string, ancestorsHaveMore []bool) {
		kids := children[parent]
		for i, kid := range kids {
			isLast := i == len(kids)-1

			var pb strings.Builder
			for _, hasMore := range ancestorsHaveMore {
				if hasMore {
					pb.WriteString("│  ")
				} else {
					pb.WriteString("   ")
				}
			}
			// Root snapshots (direct children of the phantom root) get
			// no branch glyph — they sit flush against the left edge.
			if parent != "" {
				if isLast {
					pb.WriteString("└─ ")
				} else {
					pb.WriteString("├─ ")
				}
			}

			out = append(out, kid)
			prefixes = append(prefixes, pb.String())

			if parent == "" {
				// Root children start with an empty ancestor list.
				walk(kid.Name, nil)
			} else {
				walk(kid.Name, append(ancestorsHaveMore, !isLast))
			}
		}
	}

	// Roots: every snapshot whose Parent is empty, plus any "orphan"
	// with a parent we do not have in the list (defensive — can happen
	// if a snapshot is deleted mid-refresh).
	have := make(map[string]bool, len(snaps))
	for _, s := range snaps {
		have[s.Name] = true
	}
	orphanParents := make(map[string]bool)
	for _, s := range snaps {
		if s.Parent != "" && !have[s.Parent] {
			orphanParents[s.Parent] = true
		}
	}

	walk("", nil)
	for orphan := range orphanParents {
		walk(orphan, nil)
	}

	return out, prefixes
}

// snapshotsView renders the per-domain snapshot tree.
func (m Model) snapshotsView() string {
	width := m.contentWidth()

	title := headerTitle.Render("snapshots: ") + headerValue.Render(m.snapshotsFor)

	// Header row. Leading space matches the per-row indent below.
	headerRow := listHeaderRow.Render(" " + strings.Join([]string{
		padRight("NAME", snapNameW),
		padRight("STATE", snapStateW),
		padLeft("SIZE", snapSizeW),
		padRight("CURRENT", snapCurrentW),
		padRight("CREATED", snapWhenW),
	}, "  "))

	// Body.
	var rows []string
	rows = append(rows, headerRow)

	if m.snapshotsErr != nil {
		rows = append(rows, "")
		rows = append(rows, errorStyle.Render("  error: "+m.snapshotsErr.Error()))
	} else if len(m.snapshots) == 0 {
		rows = append(rows, "")
		rows = append(rows, lipgloss.NewStyle().Foreground(colDimmed).Italic(true).
			Render("  no snapshots — press "+keyHint.Render("c")+" to create one"))
	} else {
		// Build the tree prefix list from the already-sorted slice.
		// snapshotsLoadedMsg handler has sorted m.snapshots in DFS
		// order, so walking it here reproduces the same prefixes.
		_, prefixes := sortSnapshotsAsTree(m.snapshots)
		for i, s := range m.snapshots {
			prefix := ""
			if i < len(prefixes) {
				prefix = prefixes[i]
			}
			rows = append(rows, renderSnapshotRow(s, prefix, i == m.snapshotsSel))
		}
	}

	pane := listBox.Width(width - borderWidth).Render(lipgloss.JoinVertical(lipgloss.Left, append([]string{title, ""}, rows...)...))

	// Status / input prompt at the bottom.
	var bottom string
	if m.snapshotInput {
		prompt := keyHint.Render("name: ") + m.snapshotName +
			lipgloss.NewStyle().Foreground(colAccent).Render("█") +
			headerLabel.Render("   (a-z 0-9 _ - . only · enter to create, esc to cancel)")
		bottom = statusBar.Width(width).Render(" " + prompt)
	} else if m.confirming {
		msg := errorStyle.Render(fmt.Sprintf(" ⚠ %s snapshot “%s”? ", m.confirmAction, m.confirmName)) +
			keyHint.Render("y") + statusBar.Render(" to confirm, any other key to cancel")
		bottom = statusBar.Width(width).Render(msg)
	} else if m.flash != "" && time.Now().Before(m.flashUntil) {
		bottom = statusBar.Width(width).Render(" " + flashStyle.Render(m.flash))
	} else {
		bottom = statusBar.Width(width).Render(" " +
			key("j/k") + " nav  " + key("c") + " create  " + key("r") + " revert  " +
			key("D") + " delete  " + key("esc") + " back  " + key("?") + " help")
	}

	return lipgloss.JoinVertical(lipgloss.Left, pane, bottom)
}

// renderSnapshotRow renders one snapshot row, optionally highlighted.
// prefix carries the tree glyphs ("├─ ", "│  └─ ", etc.) produced by
// sortSnapshotsAsTree and is prepended to the snapshot name.
func renderSnapshotRow(s lv.DomainSnapshot, prefix string, selected bool) string {
	current := ""
	if s.IsCurrent {
		current = "*"
	}
	when := ""
	if !s.CreatedAt.IsZero() {
		when = s.CreatedAt.Format("2006-01-02 15:04:05")
	}
	size := "—"
	if s.SizeBytes > 0 {
		size = formatBytes(float64(s.SizeBytes))
	}
	state := stateColorBySnapshotState(s.State).Render(padRight(s.State, snapStateW))

	// Tree prefix + name, truncated to the column width. lipgloss.Width
	// is aware of the multi-byte tree glyphs so truncation is correct.
	name := truncate(prefix+s.Name, snapNameW)

	cols := []string{
		padRight(name, snapNameW),
		state,
		padLeft(size, snapSizeW),
		padRight(current, snapCurrentW),
		padRight(when, snapWhenW),
	}
	row := strings.Join(cols, "  ")
	if selected {
		// Strip color in selected mode for consistent inversion.
		cols[1] = padRight(s.State, snapStateW)
		row = strings.Join(cols, "  ")
		return rowSelected.Render(" " + row)
	}
	return " " + row
}

// stateColorBySnapshotState colours snapshot states (running/shutoff/paused).
func stateColorBySnapshotState(s string) lipgloss.Style {
	switch s {
	case "running":
		return stateRunning
	case "paused", "pmsuspended":
		return statePaused
	case "shutoff", "shutdown", "":
		return stateShutoff
	case "crashed":
		return stateCrashed
	}
	return headerValue
}
