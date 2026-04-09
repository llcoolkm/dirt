package ui

import (
	"testing"
	"time"
)

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0B"},
		{500, "500B"},
		{1024, "1.0K"},
		{1536, "1.5K"},
		{10 * 1024, "10K"},
		{1024 * 1024, "1.0M"},
		{1024 * 1024 * 1024, "1.0G"},
		{6 * 1024 * 1024 * 1024, "6.0G"},
		{1024 * 1024 * 1024 * 1024, "1.0T"},
	}
	for _, c := range cases {
		got := formatBytes(c.in)
		if got != c.want {
			t.Errorf("formatBytes(%g) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatKB(t *testing.T) {
	if got := formatKB(1024); got != "1.0M" {
		t.Errorf("formatKB(1024) = %q, want 1.0M", got)
	}
	if got := formatKB(1024 * 1024); got != "1.0G" {
		t.Errorf("formatKB(1MB) = %q, want 1.0G", got)
	}
}

func TestFormatRate(t *testing.T) {
	if got := formatRate(0); got != "0 B/s" {
		t.Errorf("formatRate(0) = %q, want 0 B/s", got)
	}
	if got := formatRate(500); got != "500 B/s" {
		t.Errorf("formatRate(500) = %q, want 500 B/s", got)
	}
	if got := formatRate(2048); got != "2.0K/s" {
		t.Errorf("formatRate(2048) = %q, want 2.0K/s", got)
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		seconds int
		want    string
	}{
		{0, "—"},
		{30, "30s"},
		{90, "1m30s"},
		{3600, "1h0m"},
		{3700, "1h1m"},
		{86400, "1d0h"},
		{86400 + 3600*5, "1d5h"},
		{86400 * 12, "12d0h"},
	}
	for _, c := range cases {
		got := formatDuration(time.Duration(c.seconds) * time.Second)
		if got != c.want {
			t.Errorf("formatDuration(%ds) = %q, want %q", c.seconds, got, c.want)
		}
	}
}

func TestFormatLatencyUs(t *testing.T) {
	cases := []struct {
		us   float64
		want string
	}{
		{0, "—"},
		{12, "12µs"},
		{500, "500µs"},
		{1000, "1.0ms"},
		{2500, "2.5ms"},
	}
	for _, c := range cases {
		got := formatLatencyUs(c.us)
		if got != c.want {
			t.Errorf("formatLatencyUs(%g) = %q, want %q", c.us, got, c.want)
		}
	}
}
