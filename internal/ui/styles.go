package ui

import "github.com/charmbracelet/lipgloss"

// Color palette — kept terminal-friendly so it survives any color scheme.
var (
	colFG       = lipgloss.AdaptiveColor{Light: "236", Dark: "252"}
	colMuted    = lipgloss.AdaptiveColor{Light: "244", Dark: "244"}
	colBorder   = lipgloss.AdaptiveColor{Light: "250", Dark: "238"}
	colAccent   = lipgloss.Color("13")  // bright magenta
	colRunning  = lipgloss.Color("10")  // bright green
	colPaused   = lipgloss.Color("11")  // bright yellow
	colCrashed  = lipgloss.Color("9")   // bright red
	colDimmed   = lipgloss.Color("242") // gray
	colSelectBG = lipgloss.Color("237") // dark gray
	colSelectFG = lipgloss.Color("15")  // bright white
	colMemUsed  = lipgloss.Color("10")  // green — actively used
	colMemCache = lipgloss.Color("11")  // yellow — page cache / buffers (htop style)
	colSwap     = lipgloss.Color("13")  // magenta — swap activity
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
