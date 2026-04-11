# Future improvements

Ideas discussed but not yet implemented. The current plan, in order:
mouse support → snapshot tree → config file → detail view with graphs.
Everything else is listed below under its own bucket.

## Next: mouse support

Click to select a row in any table (main VM list, hosts, networks,
pools, snapshots). Scroll wheel to navigate up/down. Maybe double-click
to enter the detail view. Bubble Tea already wires mouse events via
`tea.MouseMsg` and dirt already uses `tea.WithMouseCellMotion()`, so
the work is mostly message-handling and row-index arithmetic.

## Then: snapshot tree visualisation

Indented tree showing parent/child relationships in the snapshots view
instead of the flat list. Low-effort, high-quality-of-life for anyone
with more than a couple of snapshots per VM.

## Then: config file (`~/.config/dirt/config.yaml`)

Persistent settings: refresh interval, colour theme, default sort,
column visibility. Sits alongside the `hosts` file added in v0.5.1.
Needs a yaml dependency (`gopkg.in/yaml.v3`).

## Then: detail view with graphs (v0.6?)

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
- **DHCP lease drill-down** in networks view (Enter on a network shows
  leases with hostname, MAC, IP, expiry).
- **Volume creation/deletion** in the pools view.
- **Colour theme customisation** — light/dark/solarized/gruvbox.

## Longer-term

- **Anomaly detection** — flash alerts when a VM exceeds its CPU/memory/IO
  baseline for a sustained period.
- **Export** — `:export csv` or `:export json` to dump the current table
  or historical stats to a file.
- **Bridge stats** — host-side network counters from
  `/sys/class/net/<bridge>/statistics/` in the networks view.

## Won't do (for now)

- **Domain creation wizard** — too complex for a monitoring TUI; use
  `virt-install` or `virt-manager`.
- **Live migration** — requires multi-host support first.
