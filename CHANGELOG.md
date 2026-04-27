# Changelog

All notable changes to dirt are documented here.

## v0.9.0 — 2026-04-27

**Bulk operations expanded, persistent preferences, more columns, more themes, header-click sort.**

### Marks and bulk operations
- `:resume` palette command — bulk-resume every marked paused VM (or the cursor row when no marks are set).
- Single-target actions (`o` ssh, `c` console, `v` viewer, `e` edit, `x` xml, `Enter` info, `C` clone, `A` attach, `M` migrate) now flash *"<action> is single-target — acting on cursor row"* when marks are set, then proceed on the cursor.

### Hot-plug
- `X` detaches a disk by target device (`vdb`, `vdc`, …) or a NIC by MAC. Reuses the attach prompt machinery with a `detach` verb.

### Bridge stats
- Two new RX/TX rate columns in the `:net` view, fed by `/sys/class/net/<bridge>/statistics/`. Remote libvirt URIs without local /sys mapping render as `—` rather than failing.

### Columns
- `:columns` opens a runtime picker mode listing every column in `vmColumns`. SPACE / Enter toggles, `a` shows all, `n` hides every non-required, Esc returns. Required columns refuse to hide.
- New columns: `cpu_bar`, `mem_bar`, `disk_bar` — coloured mini-bars (`[█████]`) honouring the active theme. `disk_bar` uses storage thresholds (80 / 95 %).
- New columns: `net_rate` (combined `↓rx ↑tx`), `autostart`, `persistent`, `arch`, `tag`. All default off.

### Grouping
- `:group arch` and `:group tag` join `:group os` and `:group state`. Domains without a tag fall under `(untagged)`.
- `tag` is a sortable column; metadata convention: `<metadata><dirt:tags xmlns:dirt="https://github.com/llcoolkm/dirt/tags/0.1">a,b,c</dirt:tags></metadata>` (set via `virsh edit`).

### Export
- `:export csv|json [path]` dumps the current filtered VM list, honouring active sort and column visibility. Default destination is `$HOME/dirt-export-<timestamp>.<ext>`.

### Per-VM info pane
- Each disk now shows a coloured capacity bar beside its source path (driven by `virDomainGetBlockInfo`).
- Each NIC shows a live `↓rx / ↑tx / pps` line under its identity row.
- The title bar carries 16-cell CPU and MEM sparklines plus current values, right-aligned.

### Themes
- Three new themes: `shades` (greyscale), `mono` (pure two-tone, attribute-driven), `phosphor` (CRT green).
- `solarized` brightened so `fg` is readable on dark terminals; new `solarized_light` carries the canonical Solarized Light palette for bright terminals.
- ntcharts axis / label styles, `matchStyle` / `matchCurrentStyle`, `vcpuColors`, and every table cell now follow the active theme.

### Configuration
- `:config` opens `config.yaml` in `$EDITOR` and reloads on exit.
- `:save` (alias `:w`, `:write`) persists current runtime preferences (theme, sort, columns, mark-advance) back to `config.yaml`. `:wq` (or `:x`) saves and quits.
- New `list.mark_advance` config field — `directional` (default, follows last cursor motion), `down` (always advance downward), or `none` (pure toggle, no movement).
- `:columns reset` restores default column visibility without touching the config file.

### Mouse
- Click a column header to sort by it. Click the active sort column to toggle direction. Works on the main VM list, `:net`, `:pool`, and `:host`. Non-sortable columns are ignored.

### Bug fixes
- Selected-row highlight now spans the whole row even when cells contain coloured ANSI (bar columns, state cells). Previously the highlight stopped at the first inner `\x1b[m` reset.

### Tests
- Test suite expanded — `internal/ui` coverage 4.7 % → 30.1 %; new tests cover marks, bulk dispatch, numeric counts, palette completion, group key derivation, bridge rate math, sort/filter, themes, viewport indicator, mark glyph rendering, attach/detach state machine, columns picker, confirm/typed-confirm dialog, and Update routing.

## v0.8.0 — 2026-04-25

**Bulk operations, vim numeric prefixes, palette overhaul, three new themes.**

### Marks and bulk operations
- `SPACE` toggles a mark on the cursor row and advances. The advance direction follows the last cursor motion (`j`/`k`/`g`/`G`), so a run of SPACEs can mark a block in either direction.
- Marks are keyed by UUID and survive sort, filter, and snapshot refresh; pruned automatically when a domain disappears.
- Mode indicator `✓ N marked` right-aligned in the status bar, with a numeric-prefix indicator alongside when a count is pending. Narrow terminals truncate hints rather than the indicator.
- Lifecycle keys (`s`, `S`, `D`, `R`, `p`, `U`) act on the marked set when any marks exist; the cursor row otherwise. Confirmation reads `N VMs (X hidden by filter)?`.
- Mass undefine above 20 demands a typed phrase confirmation (`undefine N` or `undefine N delete` to also remove storage). One stray keystroke cannot annihilate a large set.
- Single result flash for bulk: `✓ shutdown 15 VMs`, or `shutdown: 13 ok, 2 failed — <first 3 errors>; +N more`.

### Vim-style numeric prefixes
- Digits `0`–`9` accumulate into a count consumed by the next motion: `5j`, `20G`, `5<space>` (mark next 5).
- `ctrl-d` / `ctrl-u` / `pgdown` / `pgup` honour the count; default 10 if none.
- `Esc` clears layered state vim-style: pending count, then marks, then filter.
- `0` alone is a no-op (no phantom counts).

### Long-table viewport indicator
- When the domains list exceeds the visible window, the column header gains a right-aligned `12–30/47` indicator. Drops silently on terminals too narrow to host it.

### Palette overhaul
- New: `:mark [all|invert|none]`, `:sort <col> [desc|asc]`, `:theme <name>`, `:unmark`.
- `1`–`9` sort bindings removed (freed for counts). Sort moved to `:sort <id>`.
- `ctrl-a` mark-all binding removed (screen / tmux conflict). Use `:mark all`.
- Palette hint stays alive after a space — sub-arg menu shows the typed command's valid values, narrowing as you type.
- `Tab` completes top-level commands and sub-args; falls back to longest common prefix when matches are ambiguous.

### New themes
- `shades` — greyscale, states distinguished by brightness.
- `mono` — pure two-tone using only the terminal's default fg/bg. States differentiated by `italic` / `faint` / `bold` / `reverse`. Marks lean on `bold + underline`.
- `phosphor` — CRT green from deep `22` to electric `82`, all eight per-vCPU graph series in shades of green.

### Theme coverage fixes
- Every table (domains, pools, volumes, networks, leases, snapshots, hosts) now wraps non-state cells in `colFG` so themes paint the entire row, not just the state column.
- ntcharts axis and label styles now pull from the theme's `colMuted`.
- Search highlights in the `:xml` detail view honour the active theme.
- Status-bar key-hint descriptions re-assert `colMuted` after each `keyHint.Render` reset.

## v0.7.0 — 2026-04-20

**Jobs system, live migration, clone, hot-plug, volume CRUD, anomaly detection.**

### Jobs system
- Background job abstraction for long-running operations — visible in the status bar and in a dedicated `:jobs` view with progress bars, byte counters, and elapsed time.
- Snapshot create, revert, and delete run as jobs with real progress from `virDomainGetJobStats`.
- Generic `runDomainJob` helper so any blocking libvirt call becomes a job in a few lines.
- Completed jobs are pruned after 10 minutes.

### Live migration
- `M` on a running VM opens a destination picker (the hosts list, current host filtered out). Confirm with Enter to start a peer-to-peer live migration.
- Flags: `LIVE`, `PEER2PEER`, `PERSIST_DEST`, `UNDEFINE_SOURCE`, `AUTO_CONVERGE`, and `NON_SHARED_DISK` for cross-host storage copy.
- 1 Hz progress polling via `virDomainGetJobStats`; cancellation via `virDomainAbortJob`.

### VM clone
- `C` on a stopped VM opens an inline name prompt. Runs `virt-clone --auto-clone` as a background job — disk images are sparse-copied, MAC addresses regenerated.

### Hot-plug devices
- `A` on a running VM opens a device picker: `d` for disk (path + target prompt), `n` for NIC (network name prompt). Executes via `virDomainAttachDeviceFlags` with `LIVE + CONFIG`.
- Detach API wired (`DetachDisk`, `DetachNIC`) but not yet exposed in the UI.

### Volume CRUD
- `c` in the volumes view creates a new qcow2 volume (two-step name + size prompt, e.g. `10G`, `500M`). `D` deletes with confirmation. Both go through `virStorageVol*` APIs.

### Undefine with storage deletion
- `U` now shows a two-choice prompt: `y` keeps disks, `d` deletes all disk images via `virStorageVolLookupByPath` + `virStorageVolDelete`. Entirely through libvirt — works locally and remotely. Disks not managed by any pool are skipped with a warning.

### Anomaly detection
- Flash warning in the status bar when any VM sustains CPU% or memory-used% above 90% for 5+ consecutive samples. Yields to user-initiated flash messages.

### Command palette improvements
- **Prefix matching**: `:j` → jobs, `:pe` → perf, `:s` → snap. Ambiguous prefixes (`:h` → help/host) wait for more characters.
- **Tab completion**: fills in the unique match so you can see what you're about to execute.
- Hardcoded aliases removed — one canonical name per command, prefix matching handles the rest.

### Bug fixes
- User/system CPU graph Y-axis showed 1663% due to timing jitter — clamped to [0, 100] and chart uses fixed Y range.
- Y-axis labels showed 67% instead of 50% — ntcharts `yStep` is a row stride, fixed to 3 for clean 0/50/100.
- Migration P2P flag was missing — "direct migration not supported" error on every attempt.

## v0.6.2 — 2026-04-16

**Code review findings: defence-in-depth, correctness, polish.**

### Security / correctness
- **Stale async results after host switch** — every async message now carries the URI of the libvirt client that produced it. Update() discards results from the previous client via a new `stale()` helper. Also guards `m.snap == nil` in the swap handler, which could otherwise nil-deref if a late swap reply arrived after `applyConnected` cleared state.
- **SSH target IP validation** — reject any value `net.ParseIP` cannot parse before `exec.Command("ssh", ip)`. A hostile DHCP lease, ARP entry, or QGA response could previously smuggle an ssh option (e.g. `-oProxyCommand=…`) since ssh treats a leading `-` as a flag.
- **qemu-img path injection** — add `--` separator before the disk path so a path beginning with `-` (from a hostile or malformed remote libvirt domain XML) cannot be parsed as a flag.
- **Host probe client leak** — when `probeHostCmd` hit its 3s timeout but `lv.New()` eventually succeeded, the late client was orphaned. Mirror `connectCmd`'s cleanup: close it in a background goroutine.

### UX / polish
- **Honor EDITOR values with flags** — `nvim -f`, `code --wait`, `emacs -nw` now work correctly. `$EDITOR` is split on whitespace via `strings.Fields` so flags become separate argv entries.
- **Rune-aware backspace** — byte-slice backspace mangled multi-byte UTF-8 (Swedish å/ö, Spanish ñ) by chopping mid-codepoint. New `runeBackspace()` helper applied to filter, command palette, detail search, snapshot name, and host add inputs.
- **Hosts view polish** — empty-state no longer references the non-existent `:host add` command (now points at `a` and `e` keys). Status bar compacts for narrow terminals (< 80 cols: essentials only; < 110 cols: medium; otherwise full).

### Refactoring
- **`navSelect` helper** — extract the identical j/k/g/G/Home/End selection navigation from five view handlers into a single helper in `list.go`. Net reduction: ~60 lines.

## v0.6.1 — 2026-04-15

**Per-disk/NIC stats, DHCP leases, colour themes, SSH.**

### Info view — per-disk and per-NIC live stats
- Each disk row now shows cumulative read/write bytes and IOPS beside its target name (vda, vdb, …).
- Each NIC row shows cumulative rx/tx bytes and error/drop counts beside its MAC.
- Backed by new `DiskStats` and `NICStats` maps on `lv.Domain`, populated from the per-block and per-net arrays already returned by `virConnectGetAllDomainStats`.

### Networks — DHCP lease drill-down
- Press `Enter` on an active network in `:net` to see every DHCP lease: hostname, MAC, IP, expiry time.
- New `ListDHCPLeases` method on `lv.Client` wraps libvirt's `Network.GetDHCPLeases`.
- `esc` returns to the networks list; `R` refreshes.

### Colour themes
- New `theme` field in `~/.config/dirt/config.yaml` accepts: `default` (dark), `light`, `solarized`, `gruvbox`.
- All colour variables in `styles.go` are now typed as `lipgloss.TerminalColor` and reassigned by `ApplyTheme()`, which also rebuilds every composite style.

### SSH into guest
- New `o` (open) key on a running VM with a detected IP suspends dirt and execs `ssh <ip>`. The TUI resumes when the session ends.
- Mirrors the existing `c` (console) and `v` (virt-viewer) patterns via `tea.ExecProcess`.

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
