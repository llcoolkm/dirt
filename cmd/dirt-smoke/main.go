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

	// Host stats probe.
	if hs, err := c.HostStats(); err == nil {
		fmt.Printf("Host: mem=%s/%s  swap=%s/%s  load=%.2f %.2f %.2f  uptime=%.0fs\n\n",
			formatKB(hs.MemUsedKB()), formatKB(hs.MemTotalKB),
			formatKB(hs.SwapUsedKB()), formatKB(hs.SwapTotalKB),
			hs.Load1, hs.Load5, hs.Load15, hs.UptimeSeconds)
	}

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

	// Snapshot probe — list snapshots of every running domain.
	fmt.Println("Snapshot probe:")
	for _, d := range snap.Domains {
		if d.State.String() != "running" {
			continue
		}
		snaps, err := c.ListSnapshots(d.Name)
		if err != nil {
			fmt.Printf("  %-20s err: %v\n", d.Name, err)
			continue
		}
		fmt.Printf("  %-20s %d snapshots\n", d.Name, len(snaps))
		for _, s := range snaps {
			cur := ""
			if s.IsCurrent {
				cur = " *current*"
			}
			fmt.Printf("    - %s  state=%s  parent=%s%s\n", s.Name, s.State, s.Parent, cur)
		}
	}
	fmt.Println()

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
	fmt.Println()

	// Networks probe.
	fmt.Println("Networks:")
	if nets, err := c.ListNetworks(); err == nil {
		for _, n := range nets {
			fmt.Printf("  %-15s active=%-5v autostart=%-5v bridge=%-10s forward=%s\n",
				n.Name, n.Active, n.Autostart, n.Bridge, n.Forward)
		}
	} else {
		fmt.Printf("  err: %v\n", err)
	}
	fmt.Println()

	// Pools probe.
	fmt.Println("Storage pools:")
	if pools, err := c.ListStoragePools(); err == nil {
		for _, p := range pools {
			fmt.Printf("  %-25s state=%-10s type=%-6s cap=%s alloc=%s avail=%s\n",
				p.Name, p.State, p.Type, formatKB(p.Capacity/1024), formatKB(p.Allocation/1024), formatKB(p.Available/1024))
			if p.State == "running" {
				if vols, err := c.ListVolumes(p.Name); err == nil {
					for _, v := range vols {
						fmt.Printf("      vol %-30s cap=%s\n", v.Name, formatKB(v.Capacity/1024))
					}
				}
			}
		}
	} else {
		fmt.Printf("  err: %v\n", err)
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
