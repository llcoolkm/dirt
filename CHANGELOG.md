# Changelog

All notable changes to dirt are documented here.

## v0.6.0 — 2026-04-12

**Performance graphs, info view, config file, mouse, snapshot trees, keybinding overhaul.**

### Performance graphs (`:perf`)
- Tabbed braille time-series charts via ntcharts — four sub-views switched with `1`-`4` or `h`/`l`:
  - **CPU**: aggregate %, per-vCPU breakdown (colour per vCPU), user vs system
  - **MEM**: used %, cache %, swap in/out activity, swap used % (QGA)
  - **DISK**: throughput, IOPS, latency (read green / write red, overlaid)
  - **NET**: speed, packets (rx green / tx red, overlaid)
- Relative X-axis labels (`-5m`, `-3m`, …, `now`) with minute tick marks
- Fixed 8-char Y-axis labels for consistent chart alignment
- Charts cached in `Update()` — tab switching is instant
- New libvirt stats: `cpu.user`, `cpu.system`, per-vCPU CPU times
- History window extended from 30 to 300 samples (5 minutes at 1s refresh)

### Info view (`Enter`)
- VMware-style structured pane: identity, hardware (vCPUs, CPU mode, live CPU%, balloon breakdown), boot (firmware, boot order, machine type), disks, NICs, graphics
- `e` edits XML via `virsh edit` from within info view; pane refreshes on return
- `p` jumps to performance graphs; `x` jumps to raw XML
- Scrollable with `j`/`k`, `g`/`G`, `PgUp`/`PgDn`

### Config file (`~/.config/dirt/config.yaml`)
- Persistent settings: refresh interval, default sort column + direction, column visibility
- Seeded with commented defaults on first launch
- CLI flags override config for the session

### Mouse support
- Click to select in any list view (main, hosts, networks, pools, snapshots)
- Scroll wheel navigates up/down
- Guarded during text input (filter, command palette, snapshot name)

### Snapshot tree
- Snapshots rendered as an indented tree with box-drawing glyphs instead of a flat list
- Parent/child relationships visualised; orphans handled gracefully

### Keybinding overhaul
- Clear separation: `:commands` for view switching, keys for actions, `Enter`/`x` for inspection
- `d` dropped as alias for `Enter` (the view is "info", not "details")
- `P` shortcut for perf dropped from main list (use `:perf`)
- `R` = reboot (was `r`) — dangerous actions get shift keys
- `p` toggles pause/resume (pause confirms with `y`, resume is instant)
- `Shift-Tab` cycles views backwards
- Command palette shows available commands next to `:` prompt, narrowing as you type

### Other
- Default refresh changed from 2s to 1s
- Undefine rebound from `x` to `U`; `x` opens raw XML view

## v0.5.1 — 2026-04-11

**Multi-host support, graphical console, and honest remote metrics.**

### Multi-host
- New **`:host`** command palette and view: list known libvirt endpoints, connect with `Enter`, re-probe with `R`, remove with `D`, add with `a` (two-step prompt for nickname and URI) or edit the file directly with `e`.
- Host list persisted at `~/.config/dirt/hosts` (plain-text, one `<name> <uri>` per line). Seeded on first launch with the URI dirt was started against.
- Async host switching via `tea.Cmd` with a 5s timeout — slow SSH never freezes the UI. On success, per-domain history / swap / uptime state is reset so the new host is shown truthfully.
- Parallel "probe all hosts" on view open and on `R` refresh: each host gets a short-lived `NewConnect → GetHostname → Snapshot → Close` triple, results stream into the table as they arrive.
- Palette shortcuts: `:host <name>` (connect by nickname), `:host <uri>` (ad-hoc), `:host add <name> <uri>`, `:host rm <name>`.

### Remote-aware header
- `HostStats()` branches on the URI: local `qemu:///…` keeps reading `/proc/*`; remote URIs use `virNodeGetCPUStats` and `virNodeGetMemoryStats` via libvirt's node APIs.
- Host title gains a subtle `(remote)` tag when the sample came from libvirt's node APIs.
- Swap, load average, host uptime, and `SReclaimable` are not exposed by node APIs — those fields now display `—` instead of silently lying.

### Honest uptime
- When neither QGA nor the local `/proc/<pid>.mtime` source is available, the UPTIME column now shows `—` instead of a misleading `≥Ns` observation window. Per-VM title omits the uptime entirely.
- For remote connections, QGA uptime is now probed for **every** running VM (not just the highlighted one), so the list populates with real uptimes wherever QGA is installed.

### Graphical console
- New **`v`** key in the VM list opens `virt-viewer` as a detached GUI subprocess — dirt stays usable while the viewer window is open. Works for any guest OS, including Windows, because it attaches to SPICE/VNC instead of a serial port.

### Navigation
- **`Tab`** cycles top-level views: main → hosts → networks → pools → snapshots → main. Suppressed while typing into a text input (filter, command, snapshot name, host input, detail search).

### Table layout
- VM table columns now drop from the right on narrow terminals instead of wrapping. `NAME`, `STATE`, `IP` are always kept; everything else falls out in priority order as the terminal shrinks.
- View titles no longer carry duplicate key hints — the bottom status bar is now the single source of truth for keybindings in every mode, matching the main VM view's style.

### Fixes
- Every `headerBox` / `listBox` in dirt rendered two characters wider than intended because lipgloss's `Style.Width(N)` sets *content* width and adds the border on top. For side-by-side header boxes this clipped the host box's entire right edge — top-right corner, every row's right border, and bottom-right corner fell off the terminal. Introduced a `borderWidth` constant and subtracted it at every box callsite.

### Docs
- README reorganised: explicit requirements split between host and guests; a copy-paste "Quick install on Ubuntu / Debian" block; new **Hosts view** keybindings section; caveats updated for remote limitations.
- Help modal updated with `v`, `Tab`, all four `:host` forms, and a full **Hosts view** section.

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
