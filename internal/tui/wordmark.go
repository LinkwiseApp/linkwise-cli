package tui

// glyphs is a five by five block font, only for the letters LINKWISE needs.
//
// Hand-drawn rather than pulled from a figlet font file, because a figlet
// dependency would ship hundreds of fonts to render eight letters, and
// because the spacing here was chosen so that the whole wordmark is 47
// columns and still centres inside an 80-column terminal.
var glyphs = map[rune][5]string{
	'L': {
		"█    ",
		"█    ",
		"█    ",
		"█    ",
		"█████",
	},
	'I': {
		"█████",
		"  █  ",
		"  █  ",
		"  █  ",
		"█████",
	},
	'N': {
		"█   █",
		"██  █",
		"█ █ █",
		"█  ██",
		"█   █",
	},
	'K': {
		"█   █",
		"█  █ ",
		"███  ",
		"█  █ ",
		"█   █",
	},
	'W': {
		"█   █",
		"█   █",
		"█ █ █",
		"██ ██",
		"█   █",
	},
	'S': {
		"█████",
		"█    ",
		"█████",
		"    █",
		"█████",
	},
	'E': {
		"█████",
		"█    ",
		"████ ",
		"█    ",
		"█████",
	},
}

// wordmarkWidth is what banner() produces for LINKWISE: eight glyphs of five
// columns with a single column between them.
const wordmarkWidth = 8*5 + 7

// banner renders a word in the block font. An unknown letter becomes blank
// columns rather than a panic, so this can never be the thing that crashes
// the home screen.
func banner(word string) []string {
	rows := make([]string, 5)
	for i, r := range word {
		g, ok := glyphs[r]
		if !ok {
			g = [5]string{"     ", "     ", "     ", "     ", "     "}
		}
		for row := range rows {
			if i > 0 {
				rows[row] += " "
			}
			rows[row] += g[row]
		}
	}
	return rows
}

// tagline sits under the wordmark, in place of Matter's Le Guin quote.
//
// Both fit the narrowest window the interface will draw at all: the blocks
// are 47 columns and the tagline is 36, against a 60-column minimum. That is
// why there is no narrow fallback here.
const tagline = "Save it now. Actually read it later."
