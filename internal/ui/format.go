package ui

import (
	"fmt"
	"time"
)

// formatBytes turns a byte count into a short human string: 1.2K, 8.0M, 6.1G.
func formatBytes(b float64) string {
	const unit = 1024.0
	if b < unit {
		return fmt.Sprintf("%.0fB", b)
	}
	div, exp := unit, 0
	for n := b / unit; n >= unit && exp < 4; n /= unit {
		div *= unit
		exp++
	}
	val := b / div
	suffix := []string{"K", "M", "G", "T", "P"}[exp]
	if val >= 10 {
		return fmt.Sprintf("%.0f%s", val, suffix)
	}
	return fmt.Sprintf("%.1f%s", val, suffix)
}

// formatKB takes kilobytes and returns a human string.
func formatKB(kb uint64) string {
	return formatBytes(float64(kb) * 1024)
}

// formatRate turns bytes/sec into "12 MB/s" or "847 B/s".
func formatRate(bytesPerSec float64) string {
	if bytesPerSec < 1024 {
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	}
	return formatBytes(bytesPerSec) + "/s"
}

// formatDuration produces a short duration label:
//   < 1m   →  "Ns"
//   < 1h   →  "MmSs"
//   < 1d   →  "HhMm"
//   ≥ 1d   →  "Dd Hh"
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh%dm", h, m)
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd%dh", days, hours)
}

// formatLatencyUs takes a duration in microseconds and returns "0.5ms" or "12µs".
func formatLatencyUs(us float64) string {
	if us <= 0 {
		return "—"
	}
	if us < 1000 {
		return fmt.Sprintf("%.0fµs", us)
	}
	return fmt.Sprintf("%.1fms", us/1000)
}

