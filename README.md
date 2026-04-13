# dirt — a libvirt TUI

A terminal UI for managing libvirt / QEMU / KVM virtual machines.

![dirt screenshot](images/dirt_ui.png)

`dirt` is a single-binary Go program built on [Bubble Tea](https://github.com/charmbracelet/bubbletea), [lipgloss](https://github.com/charmbracelet/lipgloss), and the [official libvirt-go bindings](https://gitlab.com/libvirt/libvirt-go-module). It connects to your local (or remote) libvirt daemon and gives you a live, keyboard-driven view of every domain — with CPU and memory bars, disk and network sparklines, and full lifecycle control from a single keypress.

## Why it exists

I've always missed a good TUI for libvirt/kvm, so I vibecoded this together with Claude Code on my phone while having breakfast.

## Features

- **Live VM table** — name, state, IP, OS, vCPU, memory, CPU%, uptime
- **Sortable columns** — `1`–`5` to sort by name / state / vCPU / memory / CPU%, press again to reverse
- **Host header** (when no VM is selected): live host CPU%, multi-segment memory bar, swap bar, load average, vCPU + memory overcommit ratios
- **Per-VM header** for the highlighted VM:
  - **CPU bar** with percent (green / yellow / red by load)
  - **Memory bar**, multi-segment: green for *used*, yellow for *cache*, dim for *free*, plus a major-fault sparkline
  - **Swap bar** when `qemu-guest-agent` is installed in the guest; activity sparklines as a fallback
  - **Disk read/write** sparklines + bytes/sec + IOPS + average latency
  - **Network rx/tx** sparklines + bytes/sec + packets/sec + error/drop counts
  - **Uptime** since dirt first observed the VM running
- **Performance graphs** — tabbed braille time-series charts for CPU, memory, disk I/O, and network (`:perf`), 5-minute rolling window
- **Snapshot management** — list, create, revert, delete (`:snap`)
- **Networks view** — start/stop/autostart toggle, DHCP lease drill-down with hostname/MAC/IP/expiry (`:net`)
- **Storage pools view** — capacity bars with colour warnings, drill into volumes (`:pool`)
- **Full domain lifecycle** from single keypresses
- **Live serial console** via `virsh console` (Tea suspends, virsh runs, Tea resumes on detach)
- **Colour themes** — default (dark), light, solarized, gruvbox via `config.yaml`
- **Detail view** with full XML, scrollable, and **incremental `/` search** with match highlights and a position indicator
- **Vim-style keybindings** throughout
- **Auto-refresh** every 2 seconds; instant refresh after any action
- **OS detection** from libosinfo metadata (Ubuntu, Debian, Fedora, RHEL, Arch, openSUSE, Windows, BSD, …)
- **IP address detection** via DHCP lease → ARP → QGA fallback chain

## Requirements

**On the host running dirt:**

- Linux (tested on Ubuntu, should work on any distro with libvirt)
- A running libvirt daemon with at least one defined domain
- Membership in the `libvirt` group — so the user can talk to `qemu:///system` without sudo
- **Go 1.21+** and **libvirt development headers** (for building; dirt uses cgo bindings)
- **virt-viewer** *(optional)* — for the graphical console via the `C` key, the only way to reach Windows guests

**Inside guests** *(optional but recommended)*:

- **qemu-guest-agent** — unlocks swap usage, guest uptime over remote URIs, and in-guest reboot detection. Without it, dirt falls back to less accurate sources.

## Installation

### Quick install on Ubuntu / Debian

```sh
sudo apt install -y golang libvirt-dev pkg-config virt-viewer
sudo usermod -aG libvirt $USER         # then log out and back in
go install github.com/llcoolkm/dirt@latest
export PATH=$PATH:~/go/bin             # add to ~/.bashrc to persist
dirt
```

No git clone is required — `go install` pulls the source from the module proxy, compiles it against your local libvirt headers, and drops the binary at `~/go/bin/dirt`.

### Pinning a specific version

```sh
go install github.com/llcoolkm/dirt@v0.6.0   # exact tag
go install github.com/llcoolkm/dirt@main     # bleeding edge
```

### Building from a working copy

Useful if you want to hack on dirt:

```sh
git clone https://github.com/llcoolkm/dirt
cd dirt
go build -o dirt .
sudo install -m 0755 dirt /usr/local/bin/dirt
```

### QGA in guests (for full feature coverage)

Inside each guest you want dirt to show swap usage / guest uptime / in-guest reboot detection:

```sh
sudo apt install -y qemu-guest-agent
sudo systemctl enable --now qemu-guest-agent
```

Then verify from the host:

```sh
virsh qemu-agent-command <domain> '{"execute":"guest-ping"}'
```

A `{"return":{}}` response means the channel is live; dirt will pick it up on the next refresh.

## Usage

```sh
dirt
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--uri <uri>` | `$LIBVIRT_DEFAULT_URI` or `qemu:///system` | libvirt URI to connect to |
| `--refresh <duration>` | `1s` | refresh interval (clamped to 200ms minimum) |
| `--version` | — | print version and exit |

Examples:

```sh
dirt --refresh 1s
dirt --uri qemu+ssh://root@otherhost/system
LIBVIRT_DEFAULT_URI=qemu+ssh://root@otherhost/system dirt
```

## Keybindings

Press `?` inside `dirt` for the full help modal. The essentials:

### Navigation
| Key | Action |
|-----|--------|
| `j` / `↓` | move down |
| `k` / `↑` | move up |
| `g` / `Home` | jump to top |
| `G` / `End` | jump to bottom |
| `Ctrl-d` / `PgDn` | page down |
| `Ctrl-u` / `PgUp` | page up |
| *left click* | select a row in any list |
| *scroll wheel* | move the selection up / down |

### Filter & Sort
| Key | Action |
|-----|--------|
| `/` | filter VM list by substring |
| `Esc` | clear filter |
| `1` | sort by name |
| `2` | sort by state |
| `3` | sort by vCPU |
| `4` | sort by memory |
| `5` | sort by CPU% |
| *(same key)* | press again to reverse direction |

### Lifecycle (selected VM)
| Key | Action |
|-----|--------|
| `s` | start (if stopped) |
| `S` | graceful shutdown |
| `D` | destroy — force off (asks `y` to confirm) |
| `R` | reboot (asks `y` to confirm) |
| `p` | pause / resume (toggle, asks `y` to pause) |
| `o` | SSH into guest (requires detected IP) |
| `c` | open serial console (`Ctrl-]` to detach) — Linux-friendly |
| `v` | open graphical console via `virt-viewer` — works for Windows too |
| `e` | edit XML in `$EDITOR` (`virsh edit`) |
| `x` | open raw XML detail view |
| `Enter` | open info view |
| `U` | undefine — delete a stopped VM (asks `y` to confirm) |

### Command palette & view switching
| Key | Action |
|-----|--------|
| `:` | open command palette (the available commands are listed next to the prompt and narrow as you type) |
| `Tab` | cycle forward through top-level views: main → hosts → networks → pools → snapshots → main |
| `Shift-Tab` | cycle the same ring backwards |
| `:snap` | snapshots of selected VM |
| `:net` | libvirt networks |
| `:pool` | storage pools (and drill-down into volumes) |
| `:host` | list of known libvirt endpoints (switch hypervisors) |
| `:vm` | back to VM list |
| `:help` | open help screen |
| `:q` | quit |

### Snapshots view
| Key | Action |
|-----|--------|
| `j` / `k` | navigate snapshots |
| `c` | create snapshot (prompts for name) |
| `r` | revert to snapshot (asks `y` to confirm) |
| `D` / `x` | delete snapshot (asks `y` to confirm) |
| `R` / `F5` | refresh list |
| `esc` / `q` | back to VM list |

### Networks view
| Key | Action |
|-----|--------|
| `j` / `k` | navigate networks |
| `s` / `S` | start / stop network |
| `a` | toggle autostart |
| `Enter` | show DHCP leases (hostname, MAC, IP, expiry) |
| `R` / `F5` | refresh list |
| `esc` / `q` | back to VM list |

### Pools / Volumes view
| Key | Action |
|-----|--------|
| `j` / `k` | navigate pools or volumes |
| `s` / `S` | start / stop pool |
| `Enter` | drill into pool's volumes |
| `R` / `F5` | refresh |
| `esc` / `q` | back |

### Hosts view
| Key | Action |
|-----|--------|
| `j` / `k` | navigate hosts |
| `Enter` | connect to selected host (async, with 5s timeout) |
| `a` | add a new host — two-step prompt (`name`, then `uri`) |
| `e` | open the hosts file in `$EDITOR`; reloads on exit |
| `R` / `F5` | re-probe all hosts |
| `D` / `x` | remove selected host (asks `y` to confirm) |
| `esc` / `q` | back to VM list |

The hosts list is persisted in `~/.config/dirt/hosts` (plain-text, one `<name> <uri>` per line), seeded on first launch with whichever URI dirt was started against.

### Info view
Structured per-VM pane opened with `Enter` from the main list or info view. Shows identity (UUID, state, OS, IP, autostart, persistent), hardware (vCPUs, CPU mode, live CPU%, memory, balloon breakdown), boot (firmware, boot order, machine type), every disk (target, bus, format, source path, RO/shareable flags, total allocated/capacity), every network interface (MAC, model, source bridge/network, tap device), and graphics channels (SPICE/VNC port, listen).

| Key | Action |
|-----|--------|
| `Enter` | open info pane |
| `j` / `k` | scroll one line |
| `PgUp` / `PgDn` | scroll half page |
| `g` / `G` | jump to top / bottom |
| `e` | edit XML in `$EDITOR` (`virsh edit`) — the pane refreshes when you return |
| `x` | jump to raw XML for this VM |
| `esc` / `q` | close info view |

### Performance graphs
Tabbed braille time-series charts for the selected VM with four sub-views: **1 CPU** (aggregate, per-vCPU, user/system), **2 MEM** (used%, cache%, swap activity, swap used%), **3 DISK** (throughput, IOPS, latency), **4 NET** (speed, packets). The X-axis shows relative time (-5m, -3m, …). The history window holds 300 samples (5 minutes at the default 1s refresh).

| Key | Action |
|-----|--------|
| `:perf` | open via command palette |
| `p` | open from info view |
| `1`/`2`/`3`/`4` | jump to CPU / MEM / DISK / NET |
| `h`/`l` or `←`/`→` | cycle between tabs |
| `esc` / `q` | back to VM list |

### XML detail view
Raw live XML from libvirt. Useful for debugging or when you want the
fields the info pane does not surface. Opened with `x` from the main
VM list, or from inside the info view.

| Key | Action |
|-----|--------|
| `x` | open XML view from the main list |
| `j` / `k` / arrows | scroll by line |
| `PgUp` / `PgDn` / `←` / `→` | scroll by page |
| `g` / `Home` | top of XML |
| `G` / `End` | bottom of XML |
| `/` | incremental search |
| `n` / `N` | next / previous match |
| `Esc` | clear search; second `Esc` closes detail |

### Application
| Key | Action |
|-----|--------|
| `?` | toggle help modal |
| `q` / `Ctrl-c` | quit |

## Configuration

dirt seeds two files under `~/.config/dirt/` on first launch:

### `config.yaml`

Persistent user-level preferences. All fields optional; missing ones fall back to dirt's built-in defaults.

```yaml
# Snapshot tick rate. Floor is 200ms.
refresh: 1s

# Colour theme: default, light, solarized, gruvbox
theme: default

list:
  # Initial sort column. One of: name, state, ip, os, vcpu, mem,
  # mem_pct, cpu, uptime.
  sort_by: state

  # Reverse the natural sort direction. "Natural" is A→Z for text
  # columns and largest-first for numeric columns (so a fresh press
  # of `8` puts the hottest VMs on top, and sort_reverse: true puts
  # them at the bottom).
  sort_reverse: false

  # Toggle optional columns. NAME, STATE, IP are required and
  # cannot be hidden. Any column absent from this map stays visible,
  # so you only need to list the ones you want to hide.
  columns:
    os:      true
    vcpu:    true
    mem:     true
    mem_pct: true
    cpu:     true
    uptime:  true
    io_r:    true
    io_w:    true
```

CLI flags (e.g. `--refresh`) override the config file for the current session.

### `hosts`

Plain-text list of libvirt endpoints — see [Hosts view](#hosts-view) above.

## How memory and swap stats work

`dirt` reads several sources to populate the header pane:

### Memory (always available)

`dirt` calls `virConnectGetAllDomainStats` once per refresh, which gives a batched read of CPU, balloon, block, and interface stats for every running domain in a single round-trip. Crucially, on first sight of each running domain `dirt` issues `virDomainSetMemoryStatsPeriod(2, DOMAIN_MEM_LIVE)` so the QEMU balloon driver pushes fresh stats every 2 seconds. Without this, balloon stats default to *on demand*, which makes the cache values stale.

Used / cache / free are computed as:

```
used  = available - unused - disk_caches
cache = disk_caches
free  = unused
```

…using the standard libvirt balloon metrics.

### Swap (requires qemu-guest-agent)

The libvirt balloon driver only exposes cumulative `swap_in` / `swap_out` page counters — useful for *activity* but not *usage*. Real swap totals require running code inside the guest.

When `qemu-guest-agent` is installed and the virtio-serial channel is wired up, `dirt` calls `guest-exec /usr/bin/cat /proc/meminfo` via `virDomainQemuAgentCommand`, polls `guest-exec-status` until the cat exits, base64-decodes the output, and parses `SwapTotal` / `SwapFree` to draw a proper usage bar.

To install QGA in a guest:

```sh
sudo apt install qemu-guest-agent
sudo systemctl start qemu-guest-agent
```

Then verify from the host:

```sh
virsh qemu-agent-command <domain> '{"execute":"guest-ping"}'
```

A successful `{"return":{}}` means the channel is live and `dirt` will pick it up on the next refresh.

## Architecture

```
dirt/
├── main.go                 entry point
├── internal/
│   ├── lv/client.go        thin libvirt wrapper
│   └── ui/
│       ├── model.go        Bubble Tea Model + Update + key routing
│       ├── view.go         root render + status bar + detail view
│       ├── header.go       host + VM stats pane
│       ├── list.go         VM table with selection & sort
│       ├── graphs.go       performance charts (2×2 grid view)
│       ├── help.go         modal help screen
│       ├── history.go      rolling sparkline buffer + rate computation
│       ├── sparkline.go    Unicode block sparklines & multi-segment bars
│       ├── format.go       byte/rate humanizer
│       └── styles.go       lipgloss palette
└── cmd/
    └── dirt-smoke/         non-TUI smoke test for the lv layer
```

## Caveats and known limits
- **Remote host header** — for remote URIs (`qemu+ssh://`, `qemu+tls://`, …) dirt uses libvirt's own node APIs for CPU and memory. Swap, load average, and host uptime are not exposed by those APIs, so those fields show `—` and the header title is tagged `(remote)`.
- **Remote VM uptime** without `qemu-guest-agent` reads `—`. Locally, dirt uses the qemu process start time; remotely, only QGA can yield a true uptime.
- **Memory bar accuracy** depends on the guest's balloon driver. Without one, falls back to allocated memory (which always reads as 100% in libvirt's eyes).
- **Console detach** uses `Ctrl-]` (the `virsh console` default).
- **Windows guests** have no useful serial console by default — use `v` (virt-viewer) rather than `c` (virsh console).

## Author

km <km@grogg.org>

## License

`dirt` is free software released under the **GNU General Public License v3.0 or
later**. See [`LICENSE`](LICENSE) for the full text.

Copyright © 2026 km <km@grogg.org>

This program is free software: you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation, either version 3 of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License for more details.
