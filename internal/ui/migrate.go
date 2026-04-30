package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/llcoolkm/dirt/internal/backend"
	"github.com/llcoolkm/dirt/internal/config"
	"github.com/llcoolkm/dirt/internal/lv"
)

// ────────────────────────── Destination picker ──────────────────────────

// migrateView draws the destination-host picker modal. Reuses the
// existing hosts list — the current host is greyed out, everything
// else is a candidate.
func (m Model) migrateView() string {
	width := m.contentWidth()
	height := m.height
	if height == 0 {
		height = 24
	}

	title := headerTitle.Render("migrate: ") + headerValue.Render(m.migrateFrom) +
		headerLabel.Render("    pick a destination host")

	rows := []string{title, ""}

	candidates := m.migrateCandidates()
	if len(candidates) == 0 {
		rows = append(rows, errorStyle.Render("  no other hosts configured — add one in :host first"))
	} else {
		for i, h := range candidates {
			rows = append(rows, renderMigrateRow(h, i == m.migrateSel, m.hostsProbe[h.Name]))
		}
	}

	rows = append(rows, "", "  "+
		key("j/k")+headerLabel.Render(" nav  ")+
		key("Enter")+headerLabel.Render(" confirm  ")+
		key("esc")+headerLabel.Render(" cancel"))

	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colBorder).
		Padding(1, 2).
		Render(body)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

func renderMigrateRow(h config.Host, selected bool, probe hostProbeStatus) string {
	mark := "  "
	if selected {
		mark = headerTitle.Render("▶ ")
	}
	status := headerLabel.Render("— unknown")
	switch probe.state {
	case probeOK:
		status = stateRunning.Render("✓ reachable")
	case probeFailed:
		status = stateCrashed.Render("✗ " + errText(probe.err))
	}
	row := mark + padRight(h.Name, 16) + headerLabel.Render(padRight(h.URI, 40)) + "  " + status
	if selected {
		return rowSelected.Render(row)
	}
	return row
}

func errText(e error) string {
	if e == nil {
		return "unreachable"
	}
	s := e.Error()
	if len(s) > 30 {
		s = s[:27] + "…"
	}
	return s
}

// migrateCandidates returns every host in the list except the one we're
// currently connected to.
func (m Model) migrateCandidates() []config.Host {
	currentURI := ""
	if m.client != nil {
		currentURI = m.client.URI()
	}
	out := make([]config.Host, 0, len(m.hosts))
	for _, h := range m.hosts {
		if h.URI == currentURI {
			continue
		}
		out = append(out, h)
	}
	return out
}

// ────────────────────────── Key handler ──────────────────────────

func (m Model) handleMigrateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	candidates := m.migrateCandidates()

	switch msg.String() {
	case "?":
		m.prevMode = m.mode
		m.mode = viewHelp
		return m, nil
	case "esc", "q":
		m.mode = viewMain
		m.migrateFrom = ""
		m.migrateSel = 0
		return m, nil
	case "enter":
		if len(candidates) == 0 {
			return m, nil
		}
		if m.migrateSel >= len(candidates) {
			m.migrateSel = len(candidates) - 1
		}
		dest := candidates[m.migrateSel]
		source := m.migrateFrom
		m.mode = viewMain
		m.migrateFrom = ""
		m.migrateSel = 0
		return m, startMigrationCmd(m.client, source, dest)
	}

	if navSelect(msg.String(), &m.migrateSel, len(candidates)) {
		return m, nil
	}
	return m, nil
}

// ────────────────────────── Migration orchestration ──────────────────────────

// startMigrationCmd builds a Job, registers it via jobStartedMsg, and
// kicks off the two goroutines that drive the actual migration:
//   - runMigration (blocking MigrateToURI3 call)
//   - pollMigrationProgress (1Hz GetJobStats loop)
//
// Both goroutines communicate back to the UI via messages.
func startMigrationCmd(c backend.Backend, source string, dest config.Host) tea.Cmd {
	return func() tea.Msg {
		jobID := fmt.Sprintf("migrate-%s-%d", source, time.Now().UnixNano())
		job := &Job{
			ID:        jobID,
			Kind:      "migrate",
			Target:    source,
			Detail:    "→ " + dest.Name,
			Phase:     "starting",
			Progress:  -1,
			StartedAt: time.Now(),
			Cancel: func() {
				_ = c.AbortMigration(source)
			},
		}

		// Run the migration in its own goroutine and fire a one-shot
		// jobDoneMsg through the program channel. Tea surfaces it via
		// the program passed to startMigrationCmd's caller.
		go func() {
			err := c.Migrate(source, lv.MigrateOptions{
				DestURI:      dest.URI,
				CopyStorage:  detectCopyStorage(c, dest),
				MaxDowntimeMs: 300,
			})
			teaProgram.Send(jobDoneMsg{id: jobID, err: err})
		}()

		// Progress polling — 1Hz until done.
		go func() {
			tick := time.NewTicker(1 * time.Second)
			defer tick.Stop()
			for range tick.C {
				running, info, err := c.MigrationProgress(source)
				if err != nil || !running {
					return
				}
				total := info.DataTotal
				done := info.DataProcessed
				progress := -1.0
				if total > 0 {
					progress = float64(done) / float64(total)
				}
				phase := "transferring"
				if info.MemRemaining < info.MemTotal/100 && info.MemTotal > 0 {
					phase = "finalising"
				}
				if info.Iteration > 5 {
					phase = fmt.Sprintf("transferring (iter %d)", info.Iteration)
				}
				teaProgram.Send(jobProgressMsg{
					id:        jobID,
					phase:     phase,
					progress:  progress,
					dataDone:  done,
					dataTotal: total,
				})
			}
		}()

		return jobStartedMsg{job: job}
	}
}

// detectCopyStorage is a heuristic: if the destination host URI differs
// from the source, and we can't confirm shared storage, assume we need
// to copy. Conservative — false positives cost a slower migration, false
// negatives fail outright. Users can override via explicit flags in a
// future version.
func detectCopyStorage(source backend.Backend, dest config.Host) bool {
	// Local-to-local (same URI) never needs copy.
	if source.URI() == dest.URI {
		return false
	}
	// Both qemu:///system typically means a shared /var/lib/libvirt/images
	// in a NAS setup or nothing. We can't know without probing, so
	// default to copy for cross-host migration unless the URI says
	// otherwise.
	if strings.HasPrefix(dest.URI, "qemu+ssh://") ||
		strings.HasPrefix(dest.URI, "qemu+tls://") {
		return true
	}
	return false
}

