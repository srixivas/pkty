package widgets

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FilterApplyMsg is sent when the user submits a BPF filter.
type FilterApplyMsg struct {
	Expression string
}

// FilterBar is a text input for BPF filter expressions.
type FilterBar struct {
	active     bool
	input      []rune
	cursor     int
	width      int
	lastFilter string
	err        string

	promptStyle lipgloss.Style
	inputStyle  lipgloss.Style
	errorStyle  lipgloss.Style
}

// NewFilterBar creates a new filter input bar.
func NewFilterBar() *FilterBar {
	return &FilterBar{
		promptStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")),
		inputStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),
		errorStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")),
	}
}

func (f *FilterBar) Active() bool { return f.active }

// Activate opens the filter bar for input.
func (f *FilterBar) Activate() {
	f.active = true
	f.input = []rune(f.lastFilter)
	f.cursor = len(f.input)
	f.err = ""
}

// SetError displays an error message (e.g. invalid BPF syntax).
func (f *FilterBar) SetError(msg string) {
	f.err = msg
}

func (f *FilterBar) SetWidth(w int) {
	f.width = w
}

func (f *FilterBar) Update(msg tea.Msg) (*FilterBar, tea.Cmd) {
	if !f.active {
		return f, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			expr := string(f.input)
			f.lastFilter = expr
			f.active = false
			return f, func() tea.Msg {
				return FilterApplyMsg{Expression: expr}
			}
		case "esc":
			f.active = false
			f.err = ""
			return f, nil
		case "backspace":
			if f.cursor > 0 {
				f.input = append(f.input[:f.cursor-1], f.input[f.cursor:]...)
				f.cursor--
			}
		case "delete":
			if f.cursor < len(f.input) {
				f.input = append(f.input[:f.cursor], f.input[f.cursor+1:]...)
			}
		case "left":
			if f.cursor > 0 {
				f.cursor--
			}
		case "right":
			if f.cursor < len(f.input) {
				f.cursor++
			}
		case "home", "ctrl+a":
			f.cursor = 0
		case "end", "ctrl+e":
			f.cursor = len(f.input)
		case "ctrl+u":
			f.input = nil
			f.cursor = 0
		default:
			// Insert character
			if len(msg.String()) == 1 {
				ch := []rune(msg.String())[0]
				newInput := make([]rune, len(f.input)+1)
				copy(newInput, f.input[:f.cursor])
				newInput[f.cursor] = ch
				copy(newInput[f.cursor+1:], f.input[f.cursor:])
				f.input = newInput
				f.cursor++
			}
		}
	}
	return f, nil
}

func (f *FilterBar) View() string {
	if !f.active {
		if f.lastFilter != "" {
			label := f.promptStyle.Render("Filter: ")
			expr := f.inputStyle.Render(f.lastFilter)
			return label + expr
		}
		return ""
	}

	prompt := f.promptStyle.Render("BPF Filter: ")

	// Render input with cursor
	before := string(f.input[:f.cursor])
	cursorChar := " "
	if f.cursor < len(f.input) {
		cursorChar = string(f.input[f.cursor])
	}
	after := ""
	if f.cursor < len(f.input)-1 {
		after = string(f.input[f.cursor+1:])
	} else if f.cursor < len(f.input) {
		after = ""
	}

	cursor := lipgloss.NewStyle().
		Background(lipgloss.Color("252")).
		Foreground(lipgloss.Color("0")).
		Render(cursorChar)

	input := f.inputStyle.Render(before) + cursor + f.inputStyle.Render(after)

	result := prompt + input
	if f.err != "" {
		result += "  " + f.errorStyle.Render(f.err)
	}
	result += f.inputStyle.Render("  (enter: apply, esc: cancel)")

	return result
}
