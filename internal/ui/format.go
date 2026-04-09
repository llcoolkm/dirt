package ui

import "fmt"

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
