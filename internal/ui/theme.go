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
	mark     lipgloss.TerminalColor
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
		mark:     lipgloss.Color("14"),
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
		mark:     lipgloss.Color("6"),
	},
	// Solarized — hue from the canonical palette but lifted in
	// brightness so the foreground stays readable on a default dark
	// terminal. The original base0 (244) renders almost grey.
	"solarized": {
		fg:       lipgloss.Color("250"), // brightened base0
		muted:    lipgloss.Color("245"), // base1
		border:   lipgloss.Color("240"), // base01
		accent:   lipgloss.Color("162"), // bright magenta
		running:  lipgloss.Color("70"),  // brighter green
		paused:   lipgloss.Color("172"), // brighter yellow
		crashed:  lipgloss.Color("167"), // brighter red
		dimmed:   lipgloss.Color("242"), // base00
		selectBG: lipgloss.Color("237"), // dark backdrop
		selectFG: lipgloss.Color("254"), // base2 — very light
		memUsed:  lipgloss.Color("70"),
		memCache: lipgloss.Color("172"),
		swap:     lipgloss.Color("162"),
		mark:     lipgloss.Color("37"), // solarized cyan
	},
	// Solarized Light — canonical Solarized for terminals with a
	// light (base3 230 / base2 254) background. base00 fg on the
	// terminal's white-ish backdrop, with the same hue accents as
	// solarized dark.
	"solarized_light": {
		fg:       lipgloss.Color("241"), // base00
		muted:    lipgloss.Color("245"), // base1
		border:   lipgloss.Color("245"), // base1
		accent:   lipgloss.Color("125"), // magenta
		running:  lipgloss.Color("64"),  // green
		paused:   lipgloss.Color("136"), // yellow
		crashed:  lipgloss.Color("160"), // red
		dimmed:   lipgloss.Color("245"), // base1
		selectBG: lipgloss.Color("254"), // base2 — light highlight
		selectFG: lipgloss.Color("235"), // base02 — dark text
		memUsed:  lipgloss.Color("64"),
		memCache: lipgloss.Color("136"),
		swap:     lipgloss.Color("125"),
		mark:     lipgloss.Color("37"), // solarized cyan
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
		mark:     lipgloss.Color("108"),  // aqua
	},
	// Greyscale — shades of bone and ash. States differ by brightness.
	"shades": {
		fg:       lipgloss.Color("252"), // light grey
		muted:    lipgloss.Color("244"), // mid grey
		border:   lipgloss.Color("240"), // dim grey
		accent:   lipgloss.Color("255"), // near-white
		running:  lipgloss.Color("253"), // bright (alive)
		paused:   lipgloss.Color("246"), // mid (held)
		crashed:  lipgloss.Color("231"), // pure white, bolded in style
		dimmed:   lipgloss.Color("240"), // dim (shutoff)
		selectBG: lipgloss.Color("238"), // dark grey row
		selectFG: lipgloss.Color("231"), // pure white text
		memUsed:  lipgloss.Color("250"),
		memCache: lipgloss.Color("244"),
		swap:     lipgloss.Color("240"),
		mark:     lipgloss.Color("231"), // pure white
	},
	// Pure black and white. No shades — the terminal's own default
	// foreground on its own default background, with bold / reverse /
	// italic carrying the weight hue once did. Austere.
	"mono": {
		fg:       lipgloss.NoColor{},
		muted:    lipgloss.NoColor{},
		border:   lipgloss.NoColor{},
		accent:   lipgloss.NoColor{},
		running:  lipgloss.NoColor{},
		paused:   lipgloss.NoColor{},
		crashed:  lipgloss.NoColor{},
		dimmed:   lipgloss.NoColor{},
		selectBG: lipgloss.NoColor{},
		selectFG: lipgloss.NoColor{},
		memUsed:  lipgloss.NoColor{},
		memCache: lipgloss.NoColor{},
		swap:     lipgloss.NoColor{},
		mark:     lipgloss.NoColor{},
	},
	// Phosphor green — a CRT's glow. All hue, no greys. Bumped one
	// notch brighter than the strict greenscale to keep every cell
	// recognisable as green even on dim displays.
	"phosphor": {
		fg:       lipgloss.Color("46"),  // pure green
		muted:    lipgloss.Color("40"),  // bright green (was 34)
		border:   lipgloss.Color("28"),  // mid green (was 22)
		accent:   lipgloss.Color("82"),  // electric
		running:  lipgloss.Color("46"),
		paused:   lipgloss.Color("40"),  // bright (was 34)
		crashed:  lipgloss.Color("82"),  // electric — bolded
		dimmed:   lipgloss.Color("34"),  // medium-dim (was 28)
		selectBG: lipgloss.Color("22"),  // deep green backdrop
		selectFG: lipgloss.Color("82"),
		memUsed:  lipgloss.Color("46"),
		memCache: lipgloss.Color("34"),
		swap:     lipgloss.Color("28"),
		mark:     lipgloss.Color("82"),
	},
}

// isMonoTheme reports whether the currently applied theme is the pure
// two-tone one. Mono relies on attribute-based differentiation
// (reverse / bold / italic / underline / faint) since hue is absent.
var isMonoTheme bool

// currentTheme is the name of the last-applied theme. Used to pick
// theme-consistent series colours for graphs and other places that
// can't be expressed through the fixed palette fields.
var currentTheme = "default"

// init applies the default theme so package-level slices like
// vcpuColors are populated before anything renders — even when
// WithConfig is skipped (tests, embedded uses).
func init() { ApplyTheme("default") }

// ApplyTheme sets the global colour variables and rebuilds styles from
// the named theme. Unknown names fall back to "default".
func ApplyTheme(name string) {
	p, ok := themes[name]
	if !ok {
		p = themes["default"]
		name = "default"
	}
	isMonoTheme = name == "mono"
	currentTheme = name

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
	colMark = p.mark

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

	// In mono there is no hue, so selection and marks lean on
	// reverse / bold / underline, and states on italic / faint /
	// bold — the attributes the terminal can still render without
	// colour.
	if isMonoTheme {
		rowSelected = lipgloss.NewStyle().Reverse(true).Bold(true)
		markStyle = lipgloss.NewStyle().Bold(true).Underline(true)
		stateRunning = lipgloss.NewStyle()
		statePaused = lipgloss.NewStyle().Italic(true)
		stateCrashed = lipgloss.NewStyle().Bold(true).Reverse(true)
		stateShutoff = lipgloss.NewStyle().Faint(true)
	} else {
		rowSelected = lipgloss.NewStyle().
			Background(colSelectBG).
			Foreground(colSelectFG).
			Bold(true)
		markStyle = lipgloss.NewStyle().
			Foreground(colMark).
			Bold(true)
		stateRunning = lipgloss.NewStyle().Foreground(colRunning)
		statePaused = lipgloss.NewStyle().Foreground(colPaused)
		stateCrashed = lipgloss.NewStyle().Foreground(colCrashed).Bold(true)
		stateShutoff = lipgloss.NewStyle().Foreground(colDimmed)
	}

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
		Background(colSelectBG).
		Foreground(colAccent).
		Bold(true)

	matchCurrentStyle = lipgloss.NewStyle().
		Background(colAccent).
		Foreground(colSelectFG).
		Bold(true)

	// Update graph styles that depend on colour vars.
	graphStyleRead = lipgloss.NewStyle().Foreground(colRunning)
	graphStyleWrite = lipgloss.NewStyle().Foreground(colCrashed)
	chartAxisStyle = lipgloss.NewStyle().Foreground(colMuted)
	chartLabelStyle = lipgloss.NewStyle().Foreground(colMuted)
	vcpuColors = vcpuColorsFor(currentTheme)
}

// vcpuColorsFor returns the per-vCPU colour cycle for a given theme.
// Rainbow for coloured themes; shades of the theme's signature hue
// when hue itself is the theme.
func vcpuColorsFor(theme string) []lipgloss.Color {
	switch theme {
	case "phosphor":
		// Bright to dim, all green — CRT flavour.
		return []lipgloss.Color{
			lipgloss.Color("46"),  // bright
			lipgloss.Color("82"),  // electric
			lipgloss.Color("118"), // lime
			lipgloss.Color("154"), // pale lime
			lipgloss.Color("40"),
			lipgloss.Color("34"),
			lipgloss.Color("28"),
			lipgloss.Color("22"), // deep
		}
	case "mono":
		// Pure B&W has no hue to cycle; every series uses the
		// default foreground. Series are distinguished by position
		// in the legend rather than colour.
		out := make([]lipgloss.Color, 8)
		for i := range out {
			out[i] = lipgloss.Color("")
		}
		return out
	default:
		return []lipgloss.Color{
			lipgloss.Color("10"),  // green
			lipgloss.Color("11"),  // yellow
			lipgloss.Color("9"),   // red
			lipgloss.Color("13"),  // magenta
			lipgloss.Color("14"),  // cyan
			lipgloss.Color("12"),  // blue
			lipgloss.Color("208"), // orange
			lipgloss.Color("15"),  // white
		}
	}
}
