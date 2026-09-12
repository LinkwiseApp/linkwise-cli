package tui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// action is what pressing enter on a menu row does. An enum rather than a
// func field, so the menu stays comparable and a golden test of the home
// screen does not have to construct closures.
type action int

const (
	actBrowseAll action = iota
	actBrowseUnread
	actBrowseArchived
	actBrowseCollections
	actSearch
	actAccount
)

type menuItem struct {
	section string
	label   string
	count   string // key into the counts map, empty for rows with no count
	act     action
}

// menu is the home screen, in order. Sections are a field on the row rather
// than a nesting level, because the only thing the sections do is print a
// heading when the previous row had a different one.
// Only the collections row carries a count, because it is the only one the
// API can answer cheaply: the number is the length of a list already
// fetched, where a library count would be a request that returns nothing.
var menu = []menuItem{
	{"Library", "Browse All", "", actBrowseAll},
	{"Library", "Browse Unread", "", actBrowseUnread},
	{"Library", "Browse Archived", "", actBrowseArchived},
	{"Library", "Browse Collections", "collections", actBrowseCollections},
	{"Search", "Search", "", actSearch},
	{"Me", "Me", "", actAccount},
}

type home struct {
	cursor int
	offset int
	counts map[string]int
}

// menuLineIndex is the line a menu row is drawn on, counting the section
// headings and the blank line that precedes each one.
//
// The menu is not a flat list on screen, so scrolling it means scrolling
// rendered lines rather than rows. Deriving the mapping from the same table
// the view walks is what keeps the two from disagreeing.
func menuLineIndex(target int) int {
	line, section := 0, ""
	for i, item := range menu {
		if item.section != section {
			if section != "" {
				line++
			}
			line++
			section = item.section
		}
		if i == target {
			return line
		}
		line++
	}
	return line
}

func menuLineCount() int { return menuLineIndex(len(menu)-1) + 1 }

// wordmarkLines is what the home screen spends above the menu: five rows of
// blocks, the tagline, and a blank.
const wordmarkLines = 7

func newHome() home { return home{counts: map[string]int{}} }

func (m Model) updateHome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.home.cursor = clamp(m.home.cursor+1, len(menu))
	case "k", "up":
		m.home.cursor = clamp(m.home.cursor-1, len(menu))
	case "g", "home":
		m.home.cursor = 0
	case "G", "end":
		m.home.cursor = len(menu) - 1
	case "enter", "l", "right":
		return m.choose(menu[clamp(m.home.cursor, len(menu))].act)
	}

	// On a short window the menu is taller than the space under the
	// wordmark, and a cursor that has scrolled off the screen is a cursor
	// nobody can use.
	m.home.offset = window(menuLineIndex(m.home.cursor), m.home.offset,
		m.height-3-wordmarkLines, menuLineCount())
	return m, nil
}

func (m Model) choose(a action) (tea.Model, tea.Cmd) {
	m.failure = ""
	switch a {
	case actBrowseAll:
		return m.openBrowse(filterAll())
	case actBrowseUnread:
		return m.openBrowse(filterUnread())
	case actBrowseArchived:
		return m.openBrowse(filterArchived())
	case actBrowseCollections:
		m.push(screenCollections)
		if len(m.collections.items) == 0 && !m.collections.loading {
			m.collections.loading = true
			return m, fetchCollections(m.client)
		}
		return m, nil
	case actSearch:
		m.push(screenSearch)
		m.search.typing = true
		return m, nil
	case actAccount:
		m.push(screenAccount)
		if !m.account.loaded && !m.account.loading {
			m.account.loading = true
			return m, fetchAccount(m.client)
		}
		return m, nil
	}
	return m, nil
}

func (m Model) viewHome(st styles, height int) []string {
	var out []string

	// Centred on the window. The mark is 47 columns against a 60-column
	// minimum, so it always fits and never needs a narrow fallback.
	left := (m.width - wordmarkWidth) / 2
	if left < gutter {
		left = gutter
	}
	margin := strings.Repeat(" ", left)

	for _, row := range banner("LINKWISE") {
		out = append(out, margin+st.text.Render(row))
	}
	out = append(out, margin+st.dim.Render(tagline), "")

	var rows []string
	section := ""
	for i, item := range menu {
		if item.section != section {
			if section != "" {
				rows = append(rows, "")
			}
			rows = append(rows, margin+st.section.Render(item.section))
			section = item.section
		}

		count := ""
		if item.count != "" {
			if n, ok := m.home.counts[item.count]; ok {
				count = st.faint.Render(rightCell(thousands(n), 6))
			}
		}

		// The label column is the same width selected or not, so the counts
		// stay in one line down the screen as the cursor moves.
		label := cell(item.label, 20)
		marker := "  "
		if i == m.home.cursor {
			marker = st.accent.Render("> ")
			label = st.selected.Render(label)
		}
		rows = append(rows, margin+marker+label+"  "+count)
	}

	avail := height - wordmarkLines
	offset := window(menuLineIndex(m.home.cursor), m.home.offset, avail, len(rows))
	if end := offset + avail; end < len(rows) {
		rows = rows[offset:end]
	} else if offset < len(rows) {
		rows = rows[offset:]
	}

	return append(out, rows...)
}

// thousands groups a count so that a library of four figures reads at a
// glance. No locale handling: the separator the docs and the app already use
// is a comma.
func thousands(n int) string {
	s := strconv.Itoa(n)
	if n < 0 || len(s) <= 3 {
		return s
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	return strings.Join(append([]string{s}, parts...), ",")
}
