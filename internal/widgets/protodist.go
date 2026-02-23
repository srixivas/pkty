package widgets

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/c0d343v3r/netdash/internal/events"
)

type protoSort struct {
	Name  string
	Count uint64
}

type ProtocolDistWidget struct {
	counts map[string]uint64
	sorted []protoSort
	total  uint64
	cursor int
	focused bool
	width  int
	height int

	titleStyle    lipgloss.Style
	labelStyle    lipgloss.Style
	selectedStyle lipgloss.Style
	borderStyle   lipgloss.Style
	focusBorder   lipgloss.Style
	barColors     map[string]lipgloss.Style
}

func NewProtocolDistWidget() *ProtocolDistWidget {
	return &ProtocolDistWidget{
		counts:        make(map[string]uint64),
		titleStyle:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9")),
		labelStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		selectedStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("25")),
		borderStyle:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("9")),
		focusBorder:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("226")),
		barColors: map[string]lipgloss.Style{
			"TCP":     lipgloss.NewStyle().Foreground(lipgloss.Color("111")),
			"UDP":     lipgloss.NewStyle().Foreground(lipgloss.Color("147")),
			"DNS":     lipgloss.NewStyle().Foreground(lipgloss.Color("114")),
			"HTTP":    lipgloss.NewStyle().Foreground(lipgloss.Color("215")),
			"TLS":     lipgloss.NewStyle().Foreground(lipgloss.Color("213")),
			"ARP":     lipgloss.NewStyle().Foreground(lipgloss.Color("222")),
			"ICMP":    lipgloss.NewStyle().Foreground(lipgloss.Color("117")),
			"Unknown": lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		},
	}
}

func (p *ProtocolDistWidget) Name() string     { return "Protocol Distribution" }
func (p *ProtocolDistWidget) Init() tea.Cmd     { return nil }
func (p *ProtocolDistWidget) SetSize(w, h int)  { p.width = w; p.height = h }
func (p *ProtocolDistWidget) SetFocused(f bool) { p.focused = f }
func (p *ProtocolDistWidget) Focused() bool     { return p.focused }

func (p *ProtocolDistWidget) SelectedFilter() *DisplayFilter {
	if len(p.sorted) == 0 || p.cursor < 0 || p.cursor >= len(p.sorted) {
		return nil
	}
	f := MakeFilter(FilterProtocol, p.sorted[p.cursor].Name)
	return &f
}

func (p *ProtocolDistWidget) Update(msg tea.Msg) (Widget, tea.Cmd) {
	switch msg := msg.(type) {
	case events.PacketEvent:
		proto := msg.Protocol
		if proto == "" {
			proto = "Unknown"
		}
		p.counts[proto]++
		p.total++
		p.rebuildSorted()

	case tea.KeyMsg:
		if !p.focused {
			return p, nil
		}
		switch msg.String() {
		case "up", "k":
			if p.cursor > 0 {
				p.cursor--
			}
		case "down", "j":
			if p.cursor < len(p.sorted)-1 {
				p.cursor++
			}
		case "enter":
			if f := p.SelectedFilter(); f != nil {
				return p, func() tea.Msg { return DisplayFilterToggleMsg{Filter: *f} }
			}
		}
	}
	return p, nil
}

func (p *ProtocolDistWidget) rebuildSorted() {
	p.sorted = make([]protoSort, 0, len(p.counts))
	for name, count := range p.counts {
		p.sorted = append(p.sorted, protoSort{name, count})
	}
	sort.Slice(p.sorted, func(i, j int) bool { return p.sorted[i].Count > p.sorted[j].Count })
	if p.cursor >= len(p.sorted) {
		p.cursor = len(p.sorted) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
}

func (p *ProtocolDistWidget) View() string {
	cw := p.width - 2
	ch := p.height - 2
	if cw < 5 || ch < 2 {
		return p.borderStyle.Width(cw).Render("Proto")
	}

	border := p.borderStyle
	if p.focused {
		border = p.focusBorder
	}

	title := p.titleStyle.Render(" Protocol Distribution")

	labelW := 8
	countW := 8
	barW := cw - labelW - countW - 4
	if barW < 5 {
		barW = 5
	}

	lines := []string{title}
	maxRows := ch - 1
	for i, ps := range p.sorted {
		if i >= maxRows {
			break
		}
		pct := float64(0)
		if p.total > 0 {
			pct = float64(ps.Count) / float64(p.total)
		}
		filled := int(pct * float64(barW))
		if filled < 0 {
			filled = 0
		}

		bar := strings.Repeat("█", filled) + strings.Repeat("░", barW-filled)
		style := p.labelStyle
		if s, ok := p.barColors[ps.Name]; ok {
			style = s
		}

		label := fmt.Sprintf("%-*s", labelW, truncStr(ps.Name, labelW))
		count := fmt.Sprintf("%*d", countW, ps.Count)
		pctStr := fmt.Sprintf("%5.1f%%", pct*100)
		line := label + " " + style.Render(bar) + " " + count + " " + pctStr

		if i == p.cursor && p.focused {
			lines = append(lines, p.selectedStyle.Width(cw).Render(truncStr(label+" "+bar+" "+count+" "+pctStr, cw)))
		} else {
			lines = append(lines, truncStr(line, cw))
		}
	}

	for len(lines) < ch {
		lines = append(lines, strings.Repeat(" ", cw))
	}
	return border.Width(cw).Render(strings.Join(lines, "\n"))
}
