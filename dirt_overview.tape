# dirt — libvirt TUI overview
# Run: vhs dirt_overview.tape
# Generates: images/dirt_ui.gif

Output images/dirt_ui.gif
Set Width 1400
Set Height 650
Set FontSize 14
Set Padding 5

# Launch dirt and let it connect + populate stats
Type "./dirt"
Enter
Sleep 2s

# Navigate through VMs — show the per-VM header updating
Down
Sleep 1.5s
Down
Sleep 1.5s
Down
Sleep 1.5s

# Sort by CPU% then by name
Type "8"
Sleep 1.5s
Type "1"
Sleep 1.5s

# Filter to narrow the list
Type "/k8s"
Sleep 1.5s
Escape
Sleep 1s

# Info view — structured VM details
Up
Sleep 500ms
Enter
Sleep 2s

# Scroll down in info to show disks/NICs
Down
Down
Down
Down
Down
Sleep 1.5s

# Jump to performance graphs from info view
Type "p"
Sleep 2s

# Cycle through graph tabs
Type "2"
Sleep 1.5s
Type "3"
Sleep 1.5s
Type "4"
Sleep 1.5s
Escape
Sleep 1s

# XML detail view
Type "x"
Sleep 1.5s

# Search in XML
Type "/"
Type "memory"
Enter
Sleep 1.5s
Escape
Escape
Sleep 1s

# Snapshots view via command palette (prefix matching)
Type ":s"
Sleep 500ms
Enter
Sleep 1.5s
Escape
Sleep 500ms

# Networks view
Type ":n"
Sleep 500ms
Enter
Sleep 1.5s

# Drill into DHCP leases
Enter
Sleep 1.5s
Escape
Sleep 500ms
Escape
Sleep 500ms

# Storage pools (`:po` — `:p` alone is ambiguous with `:perf`)
Type ":po"
Sleep 500ms
Enter
Sleep 1.5s

# Drill into volumes
Enter
Sleep 1.5s
Escape
Escape
Sleep 500ms

# Hosts view
Type ":ho"
Sleep 500ms
Enter
Sleep 1.5s
Escape
Sleep 500ms

# Help screen
Type "?"
Sleep 2s
Type "?"
Sleep 500ms

# Show the command palette briefly
Type ":"
Sleep 1s
Escape
Sleep 500ms

# Quit
Type "q"
Sleep 500ms
