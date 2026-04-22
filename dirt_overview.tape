# dirt — libvirt TUI overview
# Run: vhs dirt_overview.tape
# Generates: images/dirt_ui.gif

Output images/dirt_ui.gif
Set Width 1400
Set Height 900
Set FontSize 14
Set Padding 5

# Launch dirt and let it connect + populate stats
Type "./dirt"
Enter
Sleep 3s

# Navigate through VMs — show the per-VM header updating
Down
Sleep 2s
Down
Sleep 2s
Down
Sleep 2s

# Sort by CPU% to show hottest VMs first
Type "8"
Sleep 2s

# Sort by name
Type "1"
Sleep 2s

# Filter to narrow the list
Type "/k8s"
Sleep 2s
Escape
Sleep 1s

# Info view — structured VM details
Up
Sleep 1s
Enter
Sleep 3s

# Scroll down in info to show disks/NICs
Down
Down
Down
Down
Down
Sleep 2s

# Jump to performance graphs from info view
Type "p"
Sleep 3s

# Cycle through graph tabs
Type "2"
Sleep 2s
Type "3"
Sleep 2s
Type "4"
Sleep 2s
Escape
Sleep 1s

# XML detail view
Type "x"
Sleep 2s

# Search in XML
Type "/"
Type "memory"
Enter
Sleep 2s
Escape
Escape
Sleep 1s

# Snapshots view via command palette (prefix matching)
Type ":s"
Sleep 500ms
Enter
Sleep 2s
Escape
Sleep 1s

# Networks view
Type ":n"
Sleep 500ms
Enter
Sleep 2s

# Drill into DHCP leases
Enter
Sleep 2s
Escape
Sleep 1s
Escape
Sleep 1s

# Storage pools
Type ":p"
Sleep 500ms
Enter
Sleep 2s

# Drill into volumes
Enter
Sleep 2s
Escape
Escape
Sleep 1s

# Hosts view
Type ":ho"
Sleep 500ms
Enter
Sleep 2s
Escape
Sleep 1s

# Help screen
Type "?"
Sleep 3s
Type "?"
Sleep 1s

# Back to VM list, show the command palette
Type ":"
Sleep 1s
Escape
Sleep 1s

# Quit
Type "q"
Sleep 500ms
