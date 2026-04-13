package ui

import "github.com/charmbracelet/lipgloss"

// palette groups the colour values a theme must provide.
type palette struct {
	fg       lipgloss.TerminalColor
	muted    lipgloss.TerminalColor
	border   lipgloss.TerminalColor
	accent   lipgloss.TerminalColor
	running  lipgloss.TerminalColor // green
	paused   lipgloss.TerminalColor // yellow
	crashed  lipgloss.TerminalColor // red
	dimmed   lipgloss.TerminalColor
	selectBG lipgloss.TerminalColor
	selectFG lipgloss.TerminalColor
	memUsed  lipgloss.TerminalColor
	memCache lipgloss.TerminalColor
	swap     lipgloss.TerminalColor
}

// Built-in palettes.
var themes = map[string]palette{
	"default": {
		fg:       lipgloss.AdaptiveColor{Light: "236", Dark: "252"},
		muted:    lipgloss.AdaptiveColor{Light: "244", Dark: "244"},
		border:   lipgloss.AdaptiveColor{Light: "250", Dark: "238"},
		accent:   lipgloss.Color("13"),
		running:  lipgloss.Color("10"),
		paused:   lipgloss.Color("11"),
		crashed:  lipgloss.Color("9"),
		dimmed:   lipgloss.Color("242"),
		selectBG: lipgloss.Color("237"),
		selectFG: lipgloss.Color("15"),
		memUsed:  lipgloss.Color("10"),
		memCache: lipgloss.Color("11"),
		swap:     lipgloss.Color("13"),
	},
	"light": {
		fg:       lipgloss.Color("235"),
		muted:    lipgloss.Color("245"),
		border:   lipgloss.Color("250"),
		accent:   lipgloss.Color("5"),
		running:  lipgloss.Color("2"),
		paused:   lipgloss.Color("3"),
		crashed:  lipgloss.Color("1"),
		dimmed:   lipgloss.Color("248"),
		selectBG: lipgloss.Color("254"),
		selectFG: lipgloss.Color("0"),
		memUsed:  lipgloss.Color("2"),
		memCache: lipgloss.Color("3"),
		swap:     lipgloss.Color("5"),
	},
	"solarized": {
		fg:       lipgloss.Color("244"),  // base0
		muted:    lipgloss.Color("245"),  // base1
		border:   lipgloss.Color("240"),  // base01
		accent:   lipgloss.Color("125"),  // magenta
		running:  lipgloss.Color("64"),   // green
		paused:   lipgloss.Color("136"),  // yellow
		crashed:  lipgloss.Color("160"),  // red
		dimmed:   lipgloss.Color("241"),  // base00
		selectBG: lipgloss.Color("236"),  // base02
		selectFG: lipgloss.Color("230"),  // base3
		memUsed:  lipgloss.Color("64"),   // green
		memCache: lipgloss.Color("136"),  // yellow
		swap:     lipgloss.Color("125"),  // magenta
	},
	"gruvbox": {
		fg:       lipgloss.Color("223"),  // fg
		muted:    lipgloss.Color("246"),  // gray
		border:   lipgloss.Color("239"),  // bg2
		accent:   lipgloss.Color("175"),  // purple
		running:  lipgloss.Color("142"),  // green
		paused:   lipgloss.Color("214"),  // yellow
		crashed:  lipgloss.Color("167"),  // red
		dimmed:   lipgloss.Color("245"),  // gray
		selectBG: lipgloss.Color("237"),  // bg1
		selectFG: lipgloss.Color("229"),  // fg0
		memUsed:  lipgloss.Color("142"),  // green
		memCache: lipgloss.Color("214"),  // yellow
		swap:     lipgloss.Color("175"),  // purple
	},
}

// ApplyTheme sets the global colour variables and rebuilds styles from
// the named theme. Unknown names fall back to "default".
func ApplyTheme(name string) {
	p, ok := themes[name]
	if !ok {
		p = themes["default"]
	}

	colFG = p.fg
	colMuted = p.muted
	colBorder = p.border
	colAccent = p.accent
	colRunning = p.running
	colPaused = p.paused
	colCrashed = p.crashed
	colDimmed = p.dimmed
	colSelectBG = p.selectBG
	colSelectFG = p.selectFG
	colMemUsed = p.memUsed
	colMemCache = p.memCache
	colSwap = p.swap

	rebuildStyles()
}

// rebuildStyles re-creates all composite lipgloss styles from the
// current colour variables. Called once at startup and again if the
// theme changes.
func rebuildStyles() {
	headerBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colBorder).
		Padding(0, 1)

	listBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colBorder).
		Padding(0, 1)

	statusBar = lipgloss.NewStyle().
		Foreground(colMuted)

	headerTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(colSelectFG)

	headerLabel = lipgloss.NewStyle().
		Foreground(colMuted)

	headerValue = lipgloss.NewStyle().
		Foreground(colFG)

	listHeaderRow = lipgloss.NewStyle().
		Bold(true).
		Foreground(colMuted).
		Underline(true)

	rowSelected = lipgloss.NewStyle().
		Background(colSelectBG).
		Foreground(colSelectFG).
		Bold(true)

	stateRunning = lipgloss.NewStyle().Foreground(colRunning)
	statePaused = lipgloss.NewStyle().Foreground(colPaused)
	stateCrashed = lipgloss.NewStyle().Foreground(colCrashed).Bold(true)
	stateShutoff = lipgloss.NewStyle().Foreground(colDimmed)

	errorStyle = lipgloss.NewStyle().
		Foreground(colCrashed).
		Bold(true)

	flashStyle = lipgloss.NewStyle().
		Foreground(colAccent).
		Bold(true)

	keyHint = lipgloss.NewStyle().
		Foreground(colAccent).
		Bold(true)

	matchStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("237")).
		Foreground(lipgloss.Color("11")).
		Bold(true)

	matchCurrentStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("11")).
		Foreground(lipgloss.Color("0")).
		Bold(true)

	// Update graph styles that depend on colour vars.
	graphStyleRead = lipgloss.NewStyle().Foreground(colRunning)
	graphStyleWrite = lipgloss.NewStyle().Foreground(colCrashed)
}
