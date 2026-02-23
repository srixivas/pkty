package widgets

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SearchApplyMsg is sent when the user submits a search pattern.
type SearchApplyMsg struct{ Pattern string }

// SearchBar is a text input for free-text wildcard display filters.
type SearchBar struct {
	active bool
	input  []rune
	cursor int
	width  int

	promptStyle lipgloss.Style
	inputStyle  lipgloss.Style
	hintStyle   lipgloss.Style
}

func NewSearchBar() *SearchBar {
	return &SearchBar{
		promptStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("33")),
		inputStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),
		hintStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("238")),
	}
}

func (s *SearchBar) Active() bool { return s.active }

// Activate opens the search bar for input.
func (s *SearchBar) Activate() {
	s.active = true
	s.input = nil
	s.cursor = 0
}

func (s *SearchBar) SetWidth(w int) { s.width = w }

func (s *SearchBar) Update(msg tea.KeyMsg) (*SearchBar, tea.Cmd) {
	if !s.active {
		return s, nil
	}

	switch msg.String() {
	case "enter":
		pattern := string(s.input)
		s.active = false
		s.input = nil
		s.cursor = 0
		if pattern == "" {
			return s, nil
		}
		return s, func() tea.Msg {
			return SearchApplyMsg{Pattern: pattern}
		}

	case "esc":
		s.active = false
		s.input = nil
		s.cursor = 0

	case "backspace":
		if s.cursor > 0 {
			s.input = append(s.input[:s.cursor-1], s.input[s.cursor:]...)
			s.cursor--
		}

	case "delete":
		if s.cursor < len(s.input) {
			s.input = append(s.input[:s.cursor], s.input[s.cursor+1:]...)
		}

	case "left":
		if s.cursor > 0 {
			s.cursor--
		}

	case "right":
		if s.cursor < len(s.input) {
			s.cursor++
		}

	case "home", "ctrl+a":
		s.cursor = 0

	case "end", "ctrl+e":
		s.cursor = len(s.input)

	case "ctrl+u":
		s.input = nil
		s.cursor = 0

	default:
		if len(msg.String()) == 1 {
			ch := []rune(msg.String())[0]
			newInput := make([]rune, len(s.input)+1)
			copy(newInput, s.input[:s.cursor])
			newInput[s.cursor] = ch
			copy(newInput[s.cursor+1:], s.input[s.cursor:])
			s.input = newInput
			s.cursor++
		}
	}

	return s, nil
}

func (s *SearchBar) View() string {
	prompt := s.promptStyle.Render("Search: ")

	// Render input with block cursor
	before := string(s.input[:s.cursor])
	cursorChar := " "
	if s.cursor < len(s.input) {
		cursorChar = string(s.input[s.cursor])
	}
	after := ""
	if s.cursor < len(s.input)-1 {
		after = string(s.input[s.cursor+1:])
	}

	cursorStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("252")).
		Foreground(lipgloss.Color("0"))

	inputRendered := s.inputStyle.Render(before) +
		cursorStyle.Render(cursorChar) +
		s.inputStyle.Render(after)

	hint := s.hintStyle.Render("  [IP · 1.2.3.*  domain · *.name  port · :443  proto · tcp  esc: cancel]")

	return prompt + inputRendered + hint
}
