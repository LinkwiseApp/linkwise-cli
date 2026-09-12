package tui

import "strings"

// hint is one entry in the help bar: the key as typed, and what it does.
//
// The bar is generated from these rather than written out as a string, so a
// binding and its label cannot drift apart. Adding a key without adding its
// hint is the bug this shape prevents.
type hint struct {
	key   string
	label string
}

func (h hint) String() string { return h.key + ":" + h.label }

// themeHint names the theme you would get, not the one you have, which is
// what Matter's bar does and what reads correctly as an instruction.
func themeHint(dark bool) hint {
	if dark {
		return hint{"d", "light theme"}
	}
	return hint{"d", "dark theme"}
}

func hintsFor(s screen, dark bool) []hint {
	common := []hint{{"q", "quit"}}
	if s != screenHome {
		common = append(common, hint{"esc", "back"})
	}

	switch s {
	case screenHome:
		return append(common,
			hint{"/", "search"},
			hint{"j/k", "nav"},
			themeHint(dark),
			hint{"enter", "select"},
		)
	case screenBrowse, screenCollections, screenSearch:
		return append(common,
			hint{"/", "search"},
			hint{"j/k", "nav"},
			themeHint(dark),
			hint{"enter", "select"},
		)
	case screenReader:
		return append(common,
			hint{"j/k", "scroll"},
			hint{"space", "page"},
			themeHint(dark),
			hint{"?", "keys"},
		)
	default:
		return append(common, themeHint(dark), hint{"?", "keys"})
	}
}

// helpBarText joins hints and drops whole entries from the right until it
// fits.
//
// Matter's own bar runs off the edge of the window. Truncating on the
// separator instead means the bar is always a complete sentence, and `?`
// carries the rest, so nothing is discoverable only at 120 columns.
func helpBarText(hints []hint, width int) string {
	const sep = " | "
	for n := len(hints); n > 0; n-- {
		parts := make([]string, 0, n)
		for _, h := range hints[:n] {
			parts = append(parts, h.String())
		}
		joined := strings.Join(parts, sep)
		if len(joined) <= width {
			return joined
		}
	}
	return ""
}

// fullHelp is the `?` screen: every binding, grouped by where it applies.
func fullHelp(dark bool) []struct {
	Group string
	Keys  []hint
} {
	return []struct {
		Group string
		Keys  []hint
	}{
		{"Anywhere", []hint{
			{"q / ctrl+c", "quit"},
			{"esc", "back"},
			{"?", "this list"},
			themeHint(dark),
			{"/", "search"},
		}},
		{"Lists", []hint{
			{"j / down", "next"},
			{"k / up", "previous"},
			{"g / G", "first / last"},
			{"ctrl+d / ctrl+u", "half page"},
			{"enter", "open"},
			{"r", "reload"},
		}},
		{"A link", []hint{
			{"enter", "read here"},
			{"o", "open in browser"},
			{"y", "copy the url"},
			{"e", "archive, or restore"},
			{"u", "toggle read"},
			{"p", "pin, or unpin"},
		}},
		{"Reader", []hint{
			{"j / k", "scroll a line"},
			{"space / b", "scroll a page"},
			{"g / G", "top / bottom"},
		}},
	}
}
