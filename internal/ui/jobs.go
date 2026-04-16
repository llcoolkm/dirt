package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Job is a long-running background operation surfaced in the status bar
// and the :jobs view. Kind / Target are user-facing labels; Cancel is
// called when the master aborts the job.
type Job struct {
	ID         string          // unique, e.g. "migrate-vm1-1702000000"
	Kind       string          // "migrate" / "snap-create" / "snap-delete"
	Target     string          // primary subject — VM name
	Detail     string          // "→ otherhost" / "@ snap-name"
	Phase      string          // "transferring memory" / "finalising"
	Progress   float64         // 0.0–1.0, or -1 for unbounded
	DataDone   uint64          // bytes transferred so far
	DataTotal  uint64          // total bytes (0 = unknown)
	StartedAt  time.Time
	FinishedAt time.Time       // zero until done
	Err        error
	Cancel     func()          // may be nil for non-cancellable jobs
}

// Running reports whether the job is still active.
func (j *Job) Running() bool { return j.FinishedAt.IsZero() }

// Elapsed returns how long the job has been running, or its total
// duration if it has finished.
func (j *Job) Elapsed() time.Duration {
	if j.FinishedAt.IsZero() {
		return time.Since(j.StartedAt)
	}
	return j.FinishedAt.Sub(j.StartedAt)
}

// ────────────────────────── Messages ──────────────────────────

// jobStartedMsg announces a new job, inserted into m.jobs.
type jobStartedMsg struct{ job *Job }

// jobProgressMsg updates an existing job's phase / progress / bytes.
type jobProgressMsg struct {
	id        string
	phase     string
	progress  float64
	dataDone  uint64
	dataTotal uint64
}

// jobDoneMsg marks a job finished (success or failure).
type jobDoneMsg struct {
	id  string
	err error
}

// ────────────────────────── Helpers ──────────────────────────

// activeJobs returns jobs that are currently running, sorted by start
// time (oldest first). Recent completed jobs are returned by recentJobs.
func (m Model) activeJobs() []*Job {
	out := make([]*Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		if j.Running() {
			out = append(out, j)
		}
	}
	sort.Slice(out, func(i, k int) bool { return out[i].StartedAt.Before(out[k].StartedAt) })
	return out
}

// recentJobs returns finished jobs, newest first, capped at n.
func (m Model) recentJobs(n int) []*Job {
	out := make([]*Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		if !j.Running() {
			out = append(out, j)
		}
	}
	sort.Slice(out, func(i, k int) bool {
		return out[i].FinishedAt.After(out[k].FinishedAt)
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// pruneOldJobs removes finished jobs older than ttl, keeping the jobs
// map bounded over long sessions.
func (m *Model) pruneOldJobs(ttl time.Duration) {
	cutoff := time.Now().Add(-ttl)
	for id, j := range m.jobs {
		if !j.Running() && j.FinishedAt.Before(cutoff) {
			delete(m.jobs, id)
		}
	}
}

// ────────────────────────── Status bar segment ──────────────────────────

// jobsStatusSegment renders a compact summary of active jobs suitable
// for embedding in the bottom status bar. Empty string when no jobs
// are active so the caller can conditionally prepend a separator.
func (m Model) jobsStatusSegment(maxWidth int) string {
	active := m.activeJobs()
	if len(active) == 0 {
		return ""
	}
	parts := make([]string, 0, len(active))
	for _, j := range active {
		parts = append(parts, compactJobLabel(j))
	}
	full := headerLabel.Render("jobs: ") + strings.Join(parts, headerLabel.Render(" · "))
	if lipgloss.Width(full) <= maxWidth {
		return full
	}
	// Too wide — show just the count and the most recent job.
	latest := active[len(active)-1]
	short := fmt.Sprintf("jobs: %d · %s", len(active), compactJobLabel(latest))
	return headerLabel.Render(short)
}

func compactJobLabel(j *Job) string {
	switch {
	case j.Progress >= 0 && j.Progress < 1:
		return fmt.Sprintf("%s %s [%d%%]", j.Kind, j.Target, int(j.Progress*100))
	case j.Phase != "":
		return fmt.Sprintf("%s %s [%s]", j.Kind, j.Target, j.Phase)
	default:
		return fmt.Sprintf("%s %s", j.Kind, j.Target)
	}
}

// ────────────────────────── :jobs view ──────────────────────────

func (m Model) jobsView() string {
	width := m.contentWidth()

	title := headerTitle.Render("jobs")

	active := m.activeJobs()
	recent := m.recentJobs(20)

	var rows []string

	if len(active) > 0 {
		rows = append(rows, headerLabel.Render(fmt.Sprintf("active (%d)", len(active))))
		for _, j := range active {
			rows = append(rows, renderActiveJobRow(j, width-4))
		}
		rows = append(rows, "")
	}

	rows = append(rows, headerLabel.Render(fmt.Sprintf("recent (%d)", len(recent))))
	if len(recent) == 0 && len(active) == 0 {
		rows = append(rows, "", lipgloss.NewStyle().Foreground(colDimmed).Italic(true).Render("  no jobs yet"))
	}
	for _, j := range recent {
		rows = append(rows, renderRecentJobRow(j, width-4))
	}

	pane := listBox.Width(width - borderWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left, append([]string{title, ""}, rows...)...))

	bottom := statusBar.Width(width).Render(" " +
		key("R") + " refresh  " + key("esc") + " back")

	return lipgloss.JoinVertical(lipgloss.Left, pane, bottom)
}

func renderActiveJobRow(j *Job, width int) string {
	// Two-line layout: header + progress bar.
	kindCol := padRight(j.Kind, 12)
	target := padRight(j.Target, 18)
	detail := j.Detail
	elapsed := formatDuration(j.Elapsed())
	line1 := "  " + headerValue.Render(kindCol) + "  " + headerValue.Render(target) +
		"  " + headerLabel.Render(detail) +
		headerLabel.Render("  ·  ") + headerValue.Render(elapsed)

	// Progress segment (bar + bytes + phase).
	barW := width - 20
	if barW < 20 {
		barW = 20
	}
	if barW > 60 {
		barW = 60
	}

	var segment string
	if j.Progress >= 0 {
		segment = headerLabel.Render("    [") +
			colorBar(j.Progress*100, barW) +
			headerLabel.Render("] ") +
			headerValue.Render(fmt.Sprintf("%3.0f%%", j.Progress*100))
		if j.DataTotal > 0 {
			segment += headerLabel.Render("  ·  ") +
				headerValue.Render(fmt.Sprintf("%s/%s",
					formatBytes(float64(j.DataDone)),
					formatBytes(float64(j.DataTotal))))
		}
	} else if j.Phase != "" {
		segment = "    " + headerLabel.Render(j.Phase)
	} else {
		segment = "    " + headerLabel.Render("starting…")
	}

	return line1 + "\n" + segment
}

func renderRecentJobRow(j *Job, width int) string {
	mark := stateRunning.Render("✓")
	if j.Err != nil {
		mark = stateCrashed.Render("✗")
	}
	kindCol := padRight(j.Kind, 12)
	target := padRight(j.Target, 18)
	detail := padRight(j.Detail, 18)
	elapsed := formatDuration(j.Elapsed())

	right := headerLabel.Render(elapsed)
	if j.Err != nil {
		right = errorStyle.Render(truncate(j.Err.Error(), width-60))
	}
	return "  " + mark + "  " + headerValue.Render(kindCol) + "  " +
		headerValue.Render(target) + "  " + headerLabel.Render(detail) + "  " + right
}

// ────────────────────────── Key handler ──────────────────────────

func (m Model) handleJobsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?":
		m.prevMode = m.mode
		m.mode = viewHelp
		return m, nil
	case "esc", "q":
		m.mode = viewMain
		return m, nil
	case "R", "F5":
		// No-op: jobs update via messages, but the refresh key keeps
		// muscle memory working across views.
		return m, nil
	}
	return m, nil
}
