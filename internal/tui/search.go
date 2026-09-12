package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LinkwiseApp/linkwise-cli/internal/api"
)

type search struct {
	// typing is whether the field has focus. While it does, every key is
	// text, including q and d, which are global bindings everywhere else.
	typing  bool
	query   string
	sent    string
	hits    []api.SearchHit
	cursor  int
	offset  int
	loading bool
	// searched distinguishes "no results" from "nothing asked yet", which
	// otherwise look identical and mean opposite things.
	searched bool
}

func newSearch() search { return search{} }

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.search.typing {
		switch msg.String() {
		case "esc":
			m.search.typing = false
			if m.search.query == "" {
				m.pop()
			}
			return m, nil
		case "enter":
			q := strings.TrimSpace(m.search.query)
			if q == "" {
				return m, nil
			}
			m.search.typing = false
			m.search.loading = true
			m.search.sent = q
			m.failure = ""
			return m, fetchSearch(m.client, q)
		case "ctrl+c":
			return m, tea.Quit
		case "backspace":
			if r := []rune(m.search.query); len(r) > 0 {
				m.search.query = string(r[:len(r)-1])
			}
			return m, nil
		case "ctrl+u":
			m.search.query = ""
			return m, nil
		}

		// Runes are text. Matching on the key type rather than a list of key
		// names is what lets the field take accented letters and emoji
		// without enumerating any of them.
		switch msg.Type {
		case tea.KeyRunes:
			m.search.query += string(msg.Runes)
		case tea.KeySpace:
			m.search.query += " "
		}
		return m, nil
	}

	n := len(m.search.hits)
	rows := rowsVisible(m.height - 3)

	switch msg.String() {
	case "i", "a":
		m.search.typing = true
		return m, nil
	case "j", "down":
		m.search.cursor = clamp(m.search.cursor+1, n)
	case "k", "up":
		m.search.cursor = clamp(m.search.cursor-1, n)
	case "g", "home":
		m.search.cursor = 0
	case "G", "end":
		m.search.cursor = clamp(n-1, n)
	case "enter", "l", "right":
		if n == 0 {
			return m, nil
		}
		hit := m.search.hits[clamp(m.search.cursor, n)]
		// A hit is not a link: the search rows carry an id, a title and a
		// url and nothing else. Reading one means opening the reader on a
		// link built from those, and the reader fetches the rest by id.
		return m.openReader(api.Link{
			ID:    hit.ID,
			URL:   api.Str(hit.URL, ""),
			Title: hit.Title,
		})
	case "o":
		if n > 0 {
			hit := m.search.hits[clamp(m.search.cursor, n)]
			openInBrowser(api.Str(hit.URL, ""))
			m, cmd := m.note("Opened " + hostOf(api.Str(hit.URL, "")))
			return m, cmd
		}
	}

	m.search.offset = window(m.search.cursor, m.search.offset, rows, n)
	return m, nil
}

func (m Model) applySearch(msg searchMsg) (tea.Model, tea.Cmd) {
	if msg.query != m.search.sent {
		return m, nil
	}
	m.search.loading = false
	m.search.searched = true
	if msg.err != nil {
		m.fail(msg.err)
		return m, nil
	}
	m.failure = ""
	m.search.hits = msg.hits
	m.search.cursor = 0
	m.search.offset = 0
	return m, nil
}

// resultCount avoids claiming a total the search never returned. One page is
// asked for, so a full page almost certainly means there are more, and
// saying "50 matches" there would be a number the user could not trust.
func resultCount(n int) string {
	if n >= pageSize {
		return "first " + thousands(n) + " matches"
	}
	if n == 1 {
		return "1 match"
	}
	return thousands(n) + " matches"
}

func (m Model) viewSearch(st styles, height int) []string {
	prompt := st.accent.Render("Search  ")
	field := m.search.query
	if m.search.typing {
		field += "▏"
	}
	if field == "" {
		field = st.faint.Render("type a query, then enter")
	} else {
		field = st.text.Render(field)
	}

	out := []string{indent(prompt + field), ""}

	switch {
	case m.search.loading:
		return append(out, indent(st.dim.Render("Searching…")))
	case !m.search.searched:
		return append(out, indent(st.faint.Render("Searches titles, descriptions and content.")))
	case len(m.search.hits) == 0:
		return append(out, indent(st.dim.Render("No matches for "+m.search.sent+".")))
	}

	out = append(out, indent(st.faint.Render(resultCount(len(m.search.hits))+" for "+m.search.sent)), "")

	rows := rowsVisible(height) - 2
	if rows < 1 {
		rows = 1
	}
	offset := window(m.search.cursor, m.search.offset, rows, len(m.search.hits))

	titleWidth := m.width - gutter*2 - 22
	if titleWidth < 16 {
		titleWidth = m.width - gutter*2 - 2
	}

	end := min(offset+rows, len(m.search.hits))
	for i := offset; i < end; i++ {
		hit := m.search.hits[i]
		title := api.Str(hit.Title, api.Str(hit.URL, hit.ID))
		host := st.faint.Render(cell(hostOf(api.Str(hit.URL, "")), 20))

		if i == m.search.cursor {
			out = append(out, indent(st.accent.Render("> ")+
				st.selected.Render(pad(truncate(title, titleWidth), titleWidth))+" "+host))
			continue
		}
		out = append(out, indent("  "+cell(title, titleWidth)+" "+host))
	}
	return out
}
