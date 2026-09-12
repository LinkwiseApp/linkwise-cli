package tui

import (
	"strings"

	"github.com/LinkwiseApp/linkwise-cli/internal/api"
)

type account struct {
	me      api.Me
	usage   api.Usage
	loading bool
	loaded  bool
}

func (m Model) viewAccount(st styles, height int) []string {
	out := []string{indent(st.accent.Render("Me")), ""}

	if m.account.loading && !m.account.loaded {
		return append(out, indent(st.dim.Render("Loading…")))
	}
	if !m.account.loaded {
		return append(out, indent(st.dim.Render("Nothing loaded. Press r to try again.")))
	}

	me := m.account.me
	rows := [][2]string{
		{"Name", api.Str(me.Name, "not set")},
		{"Email", me.Email},
		{"Username", api.Str(me.Username, "not set")},
	}

	if s := me.Subscription; s != nil {
		plan := planLabel(s.Plan)
		if s.IsTrial != nil && *s.IsTrial {
			plan += " (trial)"
		}
		rows = append(rows, [2]string{"Plan", plan})
		if s.RenewsAt != nil {
			rows = append(rows, [2]string{"Renews", api.Day(*s.RenewsAt)})
		}
	}

	// How this call was authenticated, because the commonest confusion on
	// this screen is which credential is in play when a scope is missing.
	rows = append(rows, [2]string{"Signed in via", me.Auth.Via})
	if scopes := me.Auth.ScopeList(); len(scopes) > 0 {
		rows = append(rows, [2]string{"Scopes", strings.Join(scopes, ", ")})
	}

	for _, r := range rows {
		out = append(out, indent(st.faint.Render(cell(r[0], 16))+st.text.Render(truncate(r[1], m.width-gutter-16))))
	}

	out = append(out, "", indent(st.faint.Render("Credits")))
	out = append(out,
		indent(st.faint.Render(cell("Chat", 16))+st.text.Render(credits(m.account.usage.Chat))),
		indent(st.faint.Render(cell("Embedding", 16))+st.text.Render(credits(m.account.usage.Embedding))),
	)
	return out
}

// planLabel turns the stored identifier into the word the pricing page uses.
// An unrecognised value is shown as it arrived rather than guessed at, so a
// plan added later reads as itself instead of as a wrong label.
func planLabel(plan string) string {
	switch plan {
	case "plan_free", "free":
		return "Free"
	case "plan_pro", "pro":
		return "Pro"
	case "":
		return "unknown"
	default:
		return plan
	}
}

// credits reads "1,200 left of 5,000", and says so plainly when the server
// sent nothing, rather than printing a confident zero.
func credits(c api.Credits) string {
	if c.Remaining == nil && c.Assigned == nil {
		return "none"
	}
	left, total := "0", "0"
	if c.Remaining != nil {
		left = thousands(*c.Remaining)
	}
	if c.Assigned != nil {
		total = thousands(*c.Assigned)
	}
	return left + " left of " + total
}
