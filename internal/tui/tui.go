// Package tui is the full-screen interface that `linkwise` opens with no
// arguments.
//
// Every screen is a pure function from the model to a string, and the only
// file that speaks to the API is data.go. That split is what lets the layout
// be tested at a fixed size with no terminal, and the navigation with no
// server.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Run takes over the terminal until the user quits.
//
// The alternate screen is used so that quitting leaves the scrollback exactly
// as it was found, which is the difference between a tool you can open in the
// middle of a session and one you have to clear afterwards.
func Run(opts Options) error {
	p := tea.NewProgram(
		New(opts),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}
