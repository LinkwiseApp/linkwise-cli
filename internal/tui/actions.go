package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LinkwiseApp/linkwise-cli/internal/browser"
)

// openInBrowser is a variable so a test can watch for the call without
// launching anything. Nothing else in the package needs to know that.
var openInBrowser = browser.Open

var copyToClipboard = browser.Copy

// copyURL reports what happened either way. A clipboard that silently did
// nothing is worse than one that says there is no clipboard here, which is
// the normal case over ssh.
func (m Model) copyURL(url string) (Model, tea.Cmd) {
	if url == "" {
		return m.note("No URL on this row.")
	}
	if copyToClipboard(url) {
		return m.note("Copied " + truncate(url, 60))
	}
	return m.note("No clipboard on this machine. The URL is " + truncate(url, 60))
}
