package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LinkwiseApp/linkwise-cli/internal/api"
)

type collections struct {
	items   []api.Collection
	cursor  int
	offset  int
	loading bool
}

func (m Model) updateCollections(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(m.collections.items)
	rows := rowsVisible(m.height - 3)

	switch msg.String() {
	case "j", "down":
		m.collections.cursor = clamp(m.collections.cursor+1, n)
	case "k", "up":
		m.collections.cursor = clamp(m.collections.cursor-1, n)
	case "g", "home":
		m.collections.cursor = 0
	case "G", "end":
		m.collections.cursor = clamp(n-1, n)
	case "r":
		m.collections.loading = true
		return m, fetchCollections(m.client)
	case "enter", "l", "right":
		if n == 0 {
			return m, nil
		}
		return m.openBrowse(filterCollection(m.collections.items[clamp(m.collections.cursor, n)]))
	}

	m.collections.offset = window(m.collections.cursor, m.collections.offset, rows, n)
	return m, nil
}

func (m Model) viewCollections(st styles, height int) []string {
	out := []string{indent(st.accent.Render("Library: collections")), ""}

	if len(m.collections.items) == 0 {
		if m.collections.loading {
			return append(out, indent(st.dim.Render("Loading…")))
		}
		return append(out, indent(st.dim.Render("No collections yet.")))
	}

	rows := rowsVisible(height)
	offset := window(m.collections.cursor, m.collections.offset, rows, len(m.collections.items))
	nameWidth := m.width - gutter*2 - 9
	if nameWidth < 10 {
		nameWidth = 10
	}

	end := min(offset+rows, len(m.collections.items))
	for i := offset; i < end; i++ {
		c := m.collections.items[i]

		// Right-aligned, so a column of counts that run from one digit to
		// four reads down its last digit rather than ragged.
		count := ""
		if c.LinkCount != nil {
			count = st.faint.Render(rightCell(thousands(*c.LinkCount), 6))
		}

		if i == m.collections.cursor {
			out = append(out, indent(st.accent.Render("> ")+
				st.selected.Render(pad(truncate(c.Name(), nameWidth), nameWidth))+" "+count))
			continue
		}
		out = append(out, indent("  "+cell(c.Name(), nameWidth)+" "+count))
	}
	return out
}
