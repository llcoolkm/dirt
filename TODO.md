# Future improvements

Ideas discussed but not yet implemented. Roughly prioritised.

## Next up: detail view with graphs (v0.6?)

A VMware-style **performance detail view** for a selected VM, showing ASCII
time-series graphs. All the rolling history data is already tracked in
`domHistory` — the feature just needs a dedicated view mode with chart
rendering.

Graphs to show:
- **CPU%** over time (line chart)
- **Disk I/O**: read/write MB/s + IOPS + average latency (stacked or overlaid)
- **Network**: rx/tx MB/s + packets/sec
- **Memory**: used/cache/free over time

The current history window is 30 samples × 2s = 1 minute. For meaningful
graphs, extend to 150–300 samples (5–10 minutes). Consider a configurable
`--history` flag.

## Medium-term

- **Per-disk breakdown** — which disk is hot? Show individual disk rows in
  detail view instead of summing across all disks.
- **Per-NIC breakdown** — same for network interfaces.
- **Snapshot tree visualisation** — indented tree showing parent/child
  relationships instead of a flat list.
- **DHCP lease drill-down** in networks view (Enter on a network shows
  leases with hostname, MAC, IP, expiry).
- **Volume creation/deletion** in the pools view.
- **Config file** (`~/.config/dirt/config.yaml`) for persistent settings:
  refresh interval, colour theme, default sort, column visibility.
- **Colour theme customisation** — light/dark/solarized/gruvbox.

## Longer-term

- **Multi-host support** — switch between multiple libvirt URIs within one
  dirt session. Tab bar or `:host` command.
- **Remote URI enhancements** — for `qemu+ssh://`, use QGA for everything
  that currently reads `/proc` on the host (CPU model, OS, uptime).
- **Anomaly detection** — flash alerts when a VM exceeds its CPU/memory/IO
  baseline for a sustained period.
- **Export** — `:export csv` or `:export json` to dump the current table
  or historical stats to a file.
- **Mouse support** — click to select a VM, scroll with wheel.
- **Bridge stats** — host-side network counters from
  `/sys/class/net/<bridge>/statistics/` in the networks view.

## Won't do (for now)

- **Domain creation wizard** — too complex for a monitoring TUI; use
  `virt-install` or `virt-manager`.
- **Live migration** — requires multi-host support first.
