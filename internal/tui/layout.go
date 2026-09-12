package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// minWidth and minHeight are the smallest usable window. Below either, the
// interface says so in one line rather than painting a scrambled layout that
// looks like a crash.
const (
	minWidth  = 60
	minHeight = 16
)

// gutter is the left margin every screen shares, which is what keeps the
// wordmark, the menu and the list columns on one vertical line.
const gutter = 2

// truncate cuts a string to a display width, measured in cells rather than
// runes so that a CJK title or an emoji does not overflow the column.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

// pad right-fills to a display width. Columns are built by padding plain
// text and styling afterwards, because styling first and padding second
// would count the escape sequences as characters.
func pad(s string, w int) string {
	gap := w - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}

// cell truncates and pads in one step, which is what every column wants.
func cell(s string, w int) string { return pad(truncate(s, w), w) }

// rightCell is the same for a number, which reads far better aligned on its
// last digit than on its first.
func rightCell(s string, w int) string {
	s = truncate(s, w)
	gap := w - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	return strings.Repeat(" ", gap) + s
}

// indent puts the shared left gutter on a line.
func indent(s string) string { return strings.Repeat(" ", gutter) + s }

// lines splits a rendered block, tolerating the trailing newline that
// lipgloss sometimes leaves.
func lines(s string) []string {
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

// clamp keeps an index inside a slice, and is deliberately tolerant of an
// empty one: a list that has not loaded yet still has a cursor.
func clamp(i, n int) int {
	if n <= 0 {
		return 0
	}
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

// window works out which slice of a list is on screen, given where the
// cursor is and how many rows fit. It scrolls by the minimum needed to keep
// the cursor visible, which is what makes j and k feel like a cursor rather
// than a paging control.
func window(cursor, offset, height, n int) int {
	if height <= 0 || n <= 0 {
		return 0
	}
	if offset > n-height {
		offset = n - height
	}
	if offset < 0 {
		offset = 0
	}
	if cursor < offset {
		return cursor
	}
	if cursor >= offset+height {
		return cursor - height + 1
	}
	return offset
}

// wrap reflows text to a width and returns it as lines, which is what the
// reader scrolls over. lipgloss does the word wrapping, so no wrapping rule
// is reimplemented here.
func wrap(s string, w int) []string {
	if w <= 0 {
		return nil
	}
	var out []string
	// Paragraph by paragraph, because lipgloss collapses a block into one
	// wrapped run and the blank lines between paragraphs are most of what
	// makes an article readable.
	for _, para := range strings.Split(s, "\n") {
		if strings.TrimSpace(para) == "" {
			out = append(out, "")
			continue
		}
		out = append(out, lines(lipgloss.NewStyle().Width(w).Render(para))...)
	}
	return out
}
