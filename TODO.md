# Future improvements

Ideas discussed but not yet implemented.

## Medium-term

- **Per-disk breakdown** — which disk is hot? Show individual disk rows in
  the info view instead of summing across all disks.
- **Per-NIC breakdown** — same for network interfaces.
- **DHCP lease drill-down** in networks view (Enter on a network shows
  leases with hostname, MAC, IP, expiry).
- **Volume creation/deletion** in the pools view.
- **Colour theme customisation** — light/dark/solarized/gruvbox via
  `config.yaml` (theme field is already reserved).

## Longer-term

- **Anomaly detection** — flash alerts when a VM exceeds its CPU/memory/IO
  baseline for a sustained period.
- **Export** — `:export csv` or `:export json` to dump the current table
  or historical stats to a file.
- **Bridge stats** — host-side network counters from
  `/sys/class/net/<bridge>/statistics/` in the networks view.
- **Live migration** — multi-host switching is in place; next step would be
  `virDomainMigrate` between connected hosts.

## Won't do (for now)

- **Domain creation wizard** — too complex for a monitoring TUI; use
  `virt-install` or `virt-manager`.
