package widgets

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Placeholder is a stub widget used during scaffold/development.
// It renders a bordered box with the widget name centered.
type Placeholder struct {
	name   string
	width  int
	height int
	style  lipgloss.Style
}

// NewPlaceholder creates a placeholder widget with the given name.
func NewPlaceholder(name string, borderColor string) *Placeholder {
	return &Placeholder{
		name:  name,
		style: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(borderColor)).
			Padding(0, 1),
	}
}

func (p *Placeholder) Name() string { return p.name }

func (p *Placeholder) Init() tea.Cmd { return nil }

func (p *Placeholder) Update(msg tea.Msg) (Widget, tea.Cmd) {
	return p, nil
}

func (p *Placeholder) View() string {
	innerW := p.width - 4 // border + padding
	innerH := p.height - 2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Render(p.name)

	// Centre the title in the available space
	padTop := (innerH - 1) / 2
	lines := make([]string, 0, innerH)
	for i := 0; i < padTop; i++ {
		lines = append(lines, "")
	}
	lines = append(lines, title)
	for len(lines) < innerH {
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")

	return p.style.
		Width(innerW).
		Height(innerH).
		Align(lipgloss.Center).
		Render(content)
}

func (p *Placeholder) SetSize(w, h int) {
	p.width = w
	p.height = h
}

// DefaultWidgets returns the full set of placeholder widgets matching
// the plan's default widget set.
func DefaultWidgets() map[string]Widget {
	_ = fmt.Sprintf // keep fmt import available
	return map[string]Widget{
		"packet_inspector": NewPlaceholder("Packet Inspector", "12"),
		"connections":      NewPlaceholder("Connections", "14"),
		"dns_queries":      NewPlaceholder("DNS Queries", "11"),
		"bandwidth":        NewPlaceholder("Bandwidth", "13"),
		"protocol_dist":    NewPlaceholder("Protocol Distribution", "9"),
		"remote_hosts":     NewPlaceholder("Remote Hosts", "10"),
		"tls_inspector":    NewPlaceholder("TLS Inspector", "208"),
	}
}
