# Future improvements

Ideas discussed but not yet implemented.

## Medium-term

- **Info view visual enrichment** — the upper-right area of the info view
  is sparse. Ideas: inline CPU/memory sparkline or mini-graph, ASCII art
  OS logo (penguin for Linux, Windows flag, BSD daemon, etc.), or a
  compact resource summary widget.
- **Per-disk breakdown** — which disk is hot? Show individual disk rows in
  the info view instead of summing across all disks. Include a capacity
  bar (allocated/total) per disk, coloured like the pool bars.
- **Per-NIC breakdown** — same for network interfaces.
- **Export** — `:export csv` or `:export json` to dump the current table
  or historical stats to a file.
- **Bridge stats** — host-side network counters from
  `/sys/class/net/<bridge>/statistics/` in the networks view.
- **Selectable columns in main table** — let the user choose which
  columns appear in the domains table (show/hide, perhaps reorder).
  Possible UX: `:columns` command to toggle a picker, or persisted
  preference in config.
- **Grouping / folding in domains table** — collapse rows into groups
  by OS, state, pool, host, or tag. Each group shows a header with
  aggregate counts/resources and can be folded/unfolded. Possible UX:
  `:group os` / `:group state` / `:group none`, with a key to
  toggle the fold at the cursor.
- **Long-table behaviour** — verify what happens when the domains
  table exceeds the terminal height. Needs scrolling (with the
  selection staying visible), a viewport indicator (e.g. `12–30/47`),
  and sane interaction with the info view and footer. Test with many
  domains on short terminals.

## Longer-term

- **Detach devices** — `DetachDisk` / `DetachNIC` API is wired but not
  yet exposed in the UI.

## Won't do (for now)

- **Domain creation wizard** — too complex for a monitoring TUI; use
  `virt-install` or `virt-manager`.
- **Model struct sub-struct grouping** — ~300 mechanical edits for
  marginal clarity gain. The `prefixed*` flat-field convention is
  clear enough at current scale. Reconsider only if Model grows past
  ~60 fields.
