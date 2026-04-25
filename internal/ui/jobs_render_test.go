package ui

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func jobsFixture() Model {
	now := time.Now()
	return Model{
		jobs: map[string]*Job{
			"job-1": {
				ID:        "job-1",
				Kind:      "snapshot create",
				Target:    "alpha",
				StartedAt: now.Add(-30 * time.Second),
				Progress:  0.45,
			},
			"job-2": {
				ID:         "job-2",
				Kind:       "migration",
				Target:     "beta",
				StartedAt:  now.Add(-5 * time.Minute),
				FinishedAt: now.Add(-1 * time.Minute),
			},
			"job-3": {
				ID:         "job-3",
				Kind:       "clone",
				Target:     "gamma",
				StartedAt:  now.Add(-2 * time.Minute),
				FinishedAt: now.Add(-30 * time.Second),
				Err:        errors.New("permission denied"),
			},
		},
		width:  200,
		height: 40,
	}
}

func TestJobsViewListsActiveAndRecent(t *testing.T) {
	m := jobsFixture()
	out := stripANSI(m.jobsView())
	for _, want := range []string{"alpha", "beta", "gamma", "snapshot create", "migration", "clone"} {
		if !strings.Contains(out, want) {
			t.Errorf("jobs view missing %q\n%s", want, out)
		}
	}
}

func TestJobsViewSurfacesError(t *testing.T) {
	m := jobsFixture()
	out := stripANSI(m.jobsView())
	if !strings.Contains(out, "permission denied") {
		t.Errorf("expected failed-job error in output, got:\n%s", out)
	}
}

func TestJobsViewEmpty(t *testing.T) {
	m := Model{
		jobs:   map[string]*Job{},
		width:  200,
		height: 40,
	}
	out := stripANSI(m.jobsView())
	if !strings.Contains(out, "no jobs") {
		t.Errorf("expected 'no jobs' hint, got:\n%s", out)
	}
}
