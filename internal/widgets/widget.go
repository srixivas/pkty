package widgets

import tea "github.com/charmbracelet/bubbletea"

// Widget is the interface every netdash widget must implement.
// Widgets receive events through bubbletea's Msg system and render
// their own view string for placement in the layout.
type Widget interface {
	// Name returns a human-readable widget name for config and display.
	Name() string

	// Init returns an initial command (or nil).
	Init() tea.Cmd

	// Update processes a bubbletea Msg and returns updated state + command.
	Update(msg tea.Msg) (Widget, tea.Cmd)

	// View renders the widget to a string.
	View() string

	// SetSize informs the widget of its allocated dimensions.
	SetSize(width, height int)
}

// Focusable is implemented by widgets that support keyboard focus and can
// produce a display filter from their currently selected item.
type Focusable interface {
	Widget
	SetFocused(bool)
	Focused() bool
	SelectedFilter() *DisplayFilter
}
