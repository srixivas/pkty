package layout

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type splashDoneMsg struct{}

func splashTimer() tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(time.Time) tea.Msg {
		return splashDoneMsg{}
	})
}

func (m Model) splashView() string {
	owl := m.boar.View()
	return lipgloss.NewStyle().
		Background(lipgloss.Color("232")).
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(owl)
}
