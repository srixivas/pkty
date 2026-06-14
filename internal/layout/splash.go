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
	owl := m.owl.View()
	return lipgloss.NewStyle().
		Background(lipgloss.Color("232")).
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(owl)
}

func (m Model) helpView() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("226")).
		MarginBottom(1)

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Width(18)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	bindings := [][2]string{
		{"1", "Focus Packet Inspector"},
		{"2", "Focus Connections"},
		{"3", "Focus DNS"},
		{"4", "Focus bottom panel"},
		{"Tab", "Cycle sub-panels / inspector panes"},
		{"n", "Toggle Inspector ↔ Network Diagram"},
		{"j / ↓", "Down"},
		{"k / ↑", "Up"},
		{"PgDn / PgUp", "Page scroll"},
		{"Enter", "Toggle display filter on row"},
		{"D", "Clear all display filters"},
		{"/", "BPF filter bar"},
		{"S", "Save pcap snapshot"},
		{"Space", "Pause / resume capture"},
		{"?", "Toggle this help"},
		{"q / Ctrl+C", "Quit"},
	}

	rows := make([]string, 0, len(bindings))
	for _, b := range bindings {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
			keyStyle.Render(b[0]),
			descStyle.Render(b[1]),
		))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 3).
		Background(lipgloss.Color("232")).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("🦉 pkty — Key Bindings"),
			lipgloss.JoinVertical(lipgloss.Left, rows...),
			lipgloss.NewStyle().Foreground(lipgloss.Color("240")).MarginTop(1).Render("press ? or esc to close"),
		))

	return lipgloss.NewStyle().
		Background(lipgloss.Color("232")).
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(box)
}
