package ui

import "github.com/charmbracelet/lipgloss"

// Color palette — kept terminal-friendly so it survives any color scheme.
// Colour variables — set by ApplyTheme() from theme.go.
var (
	colFG       lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "236", Dark: "252"}
	colMuted    lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "244", Dark: "244"}
	colBorder   lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "250", Dark: "238"}
	colAccent   lipgloss.TerminalColor = lipgloss.Color("13")
	colRunning  lipgloss.TerminalColor = lipgloss.Color("10")
	colPaused   lipgloss.TerminalColor = lipgloss.Color("11")
	colCrashed  lipgloss.TerminalColor = lipgloss.Color("9")
	colDimmed   lipgloss.TerminalColor = lipgloss.Color("242")
	colSelectBG lipgloss.TerminalColor = lipgloss.Color("237")
	colSelectFG lipgloss.TerminalColor = lipgloss.Color("15")
	colMemUsed  lipgloss.TerminalColor = lipgloss.Color("10")
	colMemCache lipgloss.TerminalColor = lipgloss.Color("11")
	colSwap     lipgloss.TerminalColor = lipgloss.Color("13")
)

// Box styles.
var (
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
	statePaused  = lipgloss.NewStyle().Foreground(colPaused)
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

	// Search match highlight (any matching substring in the detail view).
	matchStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("237")).
			Foreground(lipgloss.Color("11")).
			Bold(true)

	// Stronger highlight for the *current* match (the one the cursor is on).
	matchCurrentStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("11")).
				Foreground(lipgloss.Color("0")).
				Bold(true)
)
