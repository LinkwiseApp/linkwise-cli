package tui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Theme is every colour the interface uses, resolved for one background.
//
// Two explicit palettes rather than lipgloss.AdaptiveColor, because the
// `d` key has to be able to override what the terminal reports. A user on a
// dark terminal who wants the light palette should get it, and adaptive
// colours answer to the terminal alone.
type Theme struct {
	Dark bool

	Accent   lipgloss.TerminalColor
	Text     lipgloss.TerminalColor
	Dim      lipgloss.TerminalColor
	Faint    lipgloss.TerminalColor
	Selected lipgloss.TerminalColor
	Good     lipgloss.TerminalColor
	Bad      lipgloss.TerminalColor
}

// Brand blue is #1E2DF2. It stays exactly that on a light background and
// lifts to a periwinkle on a dark one, where the flat brand value is too
// close to the background to read as an accent at all.
func lightTheme() Theme {
	return Theme{
		Dark:     false,
		Accent:   lipgloss.Color("#1E2DF2"),
		Text:     lipgloss.Color("#111111"),
		Dim:      lipgloss.Color("#6B6B70"),
		Faint:    lipgloss.Color("#9A9AA0"),
		Selected: lipgloss.Color("#E6E7FF"),
		Good:     lipgloss.Color("#0F7A43"),
		Bad:      lipgloss.Color("#B3261E"),
	}
}

func darkTheme() Theme {
	return Theme{
		Dark:     true,
		Accent:   lipgloss.Color("#8B93FF"),
		Text:     lipgloss.Color("#E8E8EC"),
		Dim:      lipgloss.Color("#9A9AA4"),
		Faint:    lipgloss.Color("#61616B"),
		Selected: lipgloss.Color("#24263F"),
		Good:     lipgloss.Color("#4ADE80"),
		Bad:      lipgloss.Color("#FF6B6B"),
	}
}

func themeFor(dark bool) Theme {
	if dark {
		return darkTheme()
	}
	return lightTheme()
}

// noColor reports whether every style should collapse to plain text.
//
// lipgloss reads NO_COLOR through termenv already, but only at the package's
// own initialisation, and the TUI sets its renderer explicitly. Checking here
// keeps the promise the README makes about no-color.org in one obvious place.
func noColor() bool { return os.Getenv("NO_COLOR") != "" }

// styles are the handful of derived styles used in more than one screen.
// Built per frame from the theme, which is cheap and means a theme toggle
// needs no invalidation anywhere.
type styles struct {
	accent   lipgloss.Style
	text     lipgloss.Style
	dim      lipgloss.Style
	faint    lipgloss.Style
	good     lipgloss.Style
	bad      lipgloss.Style
	selected lipgloss.Style
	section  lipgloss.Style
	bar      lipgloss.Style
}

func (t Theme) styles() styles {
	if noColor() {
		plain := lipgloss.NewStyle()
		return styles{
			accent: plain, text: plain, dim: plain, faint: plain,
			good: plain, bad: plain, selected: plain, section: plain, bar: plain,
		}
	}
	return styles{
		accent:   lipgloss.NewStyle().Foreground(t.Accent),
		text:     lipgloss.NewStyle().Foreground(t.Text),
		dim:      lipgloss.NewStyle().Foreground(t.Dim),
		faint:    lipgloss.NewStyle().Foreground(t.Faint),
		good:     lipgloss.NewStyle().Foreground(t.Good),
		bad:      lipgloss.NewStyle().Foreground(t.Bad),
		selected: lipgloss.NewStyle().Foreground(t.Text).Background(t.Selected).Bold(true),
		section:  lipgloss.NewStyle().Foreground(t.Faint),
		bar:      lipgloss.NewStyle().Foreground(t.Dim),
	}
}
