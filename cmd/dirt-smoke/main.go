// dirt-smoke is a non-TUI smoke test for the lv package.
// It connects to libvirt, takes one snapshot, and prints what it sees.
package main

import (
	"fmt"
	"os"

	"github.com/llcoolkm/dirt/internal/lv"
)

func main() {
	c, err := lv.New(os.Getenv("LIBVIRT_DEFAULT_URI"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	fmt.Printf("connected to %s on host %s\n\n", c.URI(), c.Hostname())

	snap, err := c.Snapshot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "snapshot: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%-20s %-10s %-16s %-15s %4s %5s\n",
		"NAME", "STATE", "IP", "OS", "ID", "vCPU")
	for _, d := range snap.Domains {
		fmt.Printf("%-20s %-10s %-16s %-15s %4d %5d\n",
			d.Name, d.State, d.IP, d.OS, d.ID, d.NrVCPU)
	}
	fmt.Printf("\n%d domains total\n\n", len(snap.Domains))

	// QGA swap probe — try every running domain and report.
	fmt.Println("QGA swap probe:")
	for _, d := range snap.Domains {
		if d.State.String() != "running" {
			continue
		}
		info := c.Swap(d.Name)
		if info.Available {
			if info.HasSwap {
				fmt.Printf("  %-20s OK   total=%s used=%s free=%s\n",
					d.Name,
					formatKB(info.TotalKB), formatKB(info.UsedKB), formatKB(info.FreeKB))
			} else {
				fmt.Printf("  %-20s OK   no swap configured\n", d.Name)
			}
		} else {
			msg := "no QGA"
			if info.Err != nil {
				msg = info.Err.Error()
				if len(msg) > 60 {
					msg = msg[:60] + "…"
				}
			}
			fmt.Printf("  %-20s --   %s\n", d.Name, msg)
		}
	}
}

// formatKB is a tiny duplicate of internal/ui/format.go's helper for the smoke test.
func formatKB(kb uint64) string {
	const unit = 1024.0
	b := float64(kb)
	if b < unit {
		return fmt.Sprintf("%.0fK", b)
	}
	div, exp := unit, 0
	for n := b / unit; n >= unit && exp < 4; n /= unit {
		div *= unit
		exp++
	}
	val := b / div
	suffix := []string{"M", "G", "T", "P"}[exp]
	if val >= 10 {
		return fmt.Sprintf("%.0f%s", val, suffix)
	}
	return fmt.Sprintf("%.1f%s", val, suffix)
}
