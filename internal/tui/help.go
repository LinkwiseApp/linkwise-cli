package tui

import tea "github.com/charmbracelet/bubbletea"

// The help screen exists because the bar across the top truncates. Every
// binding is listed here, so no key is discoverable only on a wide window.
func (m Model) viewHelp(st styles, height int) []string {
	out := []string{indent(st.accent.Render("Keys")), ""}
	for _, group := range fullHelp(m.dark) {
		out = append(out, indent(st.faint.Render(group.Group)))
		for _, h := range group.Keys {
			out = append(out, indent("  "+st.text.Render(cell(h.key, 18))+st.dim.Render(h.label)))
		}
		out = append(out, "")
	}
	return out
}

func (m Model) updateAccount(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "r" && !m.account.loading {
		m.account.loading = true
		return m, fetchAccount(m.client)
	}
	return m, nil
}
