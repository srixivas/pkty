package layout

import (
	"strings"
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

// splashOwlLines is a larger version of the owl for the full-screen splash.
// Row index 1 (eyes) gets bright yellow; all others get amber.
var splashOwlLines = [8]string{
	`            /\___/\            `,
	`           (( o v o ))          `,
	`           ()(     )()          `,
	`            (       )           `,
	`           /|       |\          `,
	`          / |       | \         `,
	`            |       |           `,
	`            ^       ^           `,
}

var (
	splashBodyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	splashEyeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true)
	splashTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	splashSubStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
)

func (m Model) splashView() string {
	rows := make([]string, len(splashOwlLines))
	for i, line := range splashOwlLines {
		if i == 1 {
			rows[i] = splashEyeStyle.Render(line)
		} else {
			rows[i] = splashBodyStyle.Render(line)
		}
	}
	owl := strings.Join(rows, "\n")
	title := splashTitleStyle.PaddingTop(2).Render("pkty")
	sub := splashSubStyle.Render("terminal network inspector")
	content := lipgloss.JoinVertical(lipgloss.Center, owl, title, sub)

	return lipgloss.NewStyle().
		Background(lipgloss.Color("232")).
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)
}
