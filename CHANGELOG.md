# Changelog

All notable changes to dirt are documented here.

## v0.5.0 — 2026-04-10

**Layout overhaul, new metrics, safety confirmations, bug fixes.**

### Layout
- Side-by-side host + VM header boxes (stacks on narrow terminals < 110 cols)
- Both boxes balanced to 7 lines
- Width-aware DISK/NET sparklines — squeeze to fit instead of wrapping
- Column headers now aligned with data rows (was off by 1 cell)

### Host box
- Real CPU brand from `/proc/cpuinfo` (was just `x86_64` from libvirt)
- Host OS from `/etc/os-release` in title
- CPU topology subtitle: sockets · cores · threads
- Cores ordered before uptime (consistent with VM box)

### VM box
- New **STORE line**: allocated/total disk, disk count, live IOPS r/w
- Disk inventory via `virDomainGetBlockInfo` (excludes cdroms)
- Decluttered MEM/DISK/NET (IOPS, latency, pps removed from inline)

### Main table
- New **MEM%** column (balloon-derived used memory %)
- New **IO-R** / **IO-W** columns (live IOPS)
- Sort keys 1–9 match column order; IP, OS, UPTIME now sortable
- `⏎` glyph replaced with readable `Enter`

### Snapshots view
- **SIZE** column via `qemu-img info` (vm-state-size per snapshot)
- Name input rejects spaces and invalid chars
- Empty-name validation with flash error

### Safety
- **Shutdown** (`S`) now requires `y` to confirm
- **Reboot** (`r`) now requires `y` to confirm
- **Network stop** (`S` in `:net`) requires `y` to confirm
- **Pool stop** (`S` in `:pool`) requires `y` to confirm

### Navigation
- `?` help works in every view, dismiss returns to originating view

### Guest uptime
- QGA-based guest uptime (survives in-VM reboots)
- Real qemu process start time from `/proc/<pid>` mtime

### Bug fixes (from Codex review)
- Snapshot revert/delete confirms were no-ops (routing order)
- Host CPU% delta was one tick stale
- Stale async results could overwrite current selection's data

## v0.4.3 — 2026-04-09

- Real VM uptime from qemu process start time (`/proc/<pid>` mtime)
- Falls back to dirt-side estimate for remote URIs

## v0.4.2 — 2026-04-09

- Fix: snapshot revert/delete confirmations were silent no-ops
- Fix: host CPU% delta computed against wrong sample
- Fix: stale async loader results could overwrite current view

## v0.4.1 — 2026-04-09

- Fix: `go install` builds now report correct version via `runtime/debug.ReadBuildInfo`

## v0.4.0 — 2026-04-09

**10 new operational metrics.**

- Host header: live CPU% bar from `/proc/stat`, multi-segment memory bar, swap bar, load average, host uptime, vCPU and memory overcommit ratios with colour warnings
- Per-VM header: disk IOPS + average latency, network packets/sec, error/drop counts, major-fault sparkline, VM uptime
- Main table: UPTIME column (replaces ID)
- Networks view: DHCP lease count column
- Pools view: capacity bar coloured yellow ≥80%, red ≥95%

## v0.3.0 — 2026-04-09

**Networks, storage pools, and volumes views.**

- `:net` — libvirt networks (start/stop/autostart toggle, DHCP leases)
- `:pool` — storage pools with capacity bars
- Drill into a pool's volumes via Enter
- Help modal updated with new view sections

## v0.2.0 — 2026-04-09

**Snapshots view + command palette.**

- New viewMode enum replaces detailMode/showHelp bools
- `:` command palette (k9s style): `:snap`, `:vm`, `:help`, `:q`
- Snapshot view with create/revert/delete (with confirms) and refresh

## v0.1.1 — 2026-04-09

- `e` to edit XML in `$EDITOR` (`virsh edit`)
- `x` to undefine stopped domains (with confirm)
- Aggregate host stats in default header (CPU model, total RAM, allocated)
- Connecting splash on startup
- CLI flags: `--uri`, `--refresh`, `--version`
- Unit tests on parsers, format helpers, sparkline/bar/multiBar

## v0.1.0 — 2026-04-09

**First release.**

- Live VM table: name, state, IP, OS, vCPU, memory, CPU%
- htop-style header: CPU bar, multi-segment MEM bar (used/cache/free), swap (QGA-aware), disk/net sparklines
- Sortable columns (1–5)
- Full domain lifecycle: start, shutdown, destroy (confirm), reboot, pause, resume
- Console launch via `virsh console`
- Detail view: scrollable XML, incremental `/` search, match highlights, position indicator
- Vim-style keybindings, 2-second auto-refresh
- OS detection from libosinfo metadata, IP from DHCP lease → ARP → QGA fallback
- GPL v3.0-or-later license
