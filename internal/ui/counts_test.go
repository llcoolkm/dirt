package ui

import "testing"

func TestAccumulateCount(t *testing.T) {
	cases := []struct {
		name   string
		digits []uint8
		want   int
	}{
		{"single digit", []uint8{5}, 5},
		{"two digits", []uint8{2, 0}, 20},
		{"leading zero meaningful only when count pending",
			[]uint8{1, 0, 0}, 100},
		{"clamp at 9999",
			[]uint8{9, 9, 9, 9, 5}, 9999},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := &Model{}
			for _, d := range c.digits {
				m.accumulateCount(d)
			}
			if m.pendingCount != c.want {
				t.Errorf("pendingCount=%d, want %d", m.pendingCount, c.want)
			}
		})
	}
}

func TestConsumeCountResets(t *testing.T) {
	m := &Model{pendingCount: 7}
	got := m.consumeCount()
	if got != 7 {
		t.Errorf("consumeCount returned %d, want 7", got)
	}
	if m.pendingCount != 0 {
		t.Errorf("pendingCount=%d after consume, want 0", m.pendingCount)
	}
}

func TestConsumeCountDefaultsToOne(t *testing.T) {
	m := &Model{}
	if got := m.consumeCount(); got != 1 {
		t.Errorf("zero count should default to 1, got %d", got)
	}
}

func TestDirOrDown(t *testing.T) {
	cases := []struct {
		lastDir int
		want    int
	}{
		{0, +1},  // unset → down
		{+1, +1}, // down stays
		{-1, -1}, // up stays
		{99, +1}, // anything weird → down
	}
	for _, c := range cases {
		m := Model{lastDir: c.lastDir}
		if got := m.dirOrDown(); got != c.want {
			t.Errorf("lastDir=%d: dirOrDown=%d, want %d", c.lastDir, got, c.want)
		}
	}
}
