package widgets

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/srixivas/pkty/internal/events"
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
			"TCP":      lipgloss.NewStyle().Foreground(lipgloss.Color("111")),
			"UDP":      lipgloss.NewStyle().Foreground(lipgloss.Color("147")),
			"HTTP":     lipgloss.NewStyle().Foreground(lipgloss.Color("215")),
			"HTTPS":    lipgloss.NewStyle().Foreground(lipgloss.Color("213")),
			"DNS":      lipgloss.NewStyle().Foreground(lipgloss.Color("114")),
			"SSH":      lipgloss.NewStyle().Foreground(lipgloss.Color("226")),
			"FTP":      lipgloss.NewStyle().Foreground(lipgloss.Color("208")),
			"SMTP":     lipgloss.NewStyle().Foreground(lipgloss.Color("180")),
			"SMTPS":    lipgloss.NewStyle().Foreground(lipgloss.Color("181")),
			"POP3":     lipgloss.NewStyle().Foreground(lipgloss.Color("179")),
			"IMAP":     lipgloss.NewStyle().Foreground(lipgloss.Color("178")),
			"IMAPS":    lipgloss.NewStyle().Foreground(lipgloss.Color("177")),
			"RDP":      lipgloss.NewStyle().Foreground(lipgloss.Color("204")),
			"VNC":      lipgloss.NewStyle().Foreground(lipgloss.Color("205")),
			"NTP":      lipgloss.NewStyle().Foreground(lipgloss.Color("116")),
			"mDNS":     lipgloss.NewStyle().Foreground(lipgloss.Color("115")),
			"MySQL":    lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
			"Postgres": lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
			"Redis":    lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
			"MongoDB":  lipgloss.NewStyle().Foreground(lipgloss.Color("34")),
			"DHCP":     lipgloss.NewStyle().Foreground(lipgloss.Color("220")),
			"TLS":      lipgloss.NewStyle().Foreground(lipgloss.Color("213")),
			"ARP":      lipgloss.NewStyle().Foreground(lipgloss.Color("222")),
			"ICMP":     lipgloss.NewStyle().Foreground(lipgloss.Color("117")),
			"Unknown":  lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		},
	}
}

func (p *ProtocolDistWidget) Name() string     { return "Protocol Distribution" }
func (p *ProtocolDistWidget) Init() tea.Cmd     { return nil }
func (p *ProtocolDistWidget) SetSize(w, h int)  { p.width = w; p.height = h }
func (p *ProtocolDistWidget) SetFocused(f bool) { p.focused = f }
func (p *ProtocolDistWidget) Focused() bool     { return p.focused }

// l7PrimaryPort maps port-inferred service names to their primary port string.
// Services here were inferred from ports in effectiveProto(), so we filter by
// port to get consistent packet-list results.
var l7PrimaryPort = map[string]string{
	"HTTPS":    "443",
	"SSH":      "22",
	"FTP":      "21",
	"SMTP":     "25",
	"SMTPS":    "465",
	"POP3":     "110",
	"IMAP":     "143",
	"IMAPS":    "993",
	"MySQL":    "3306",
	"Postgres": "5432",
	"Redis":    "6379",
	"MongoDB":  "27017",
	"mDNS":     "5353",
	"DHCP":     "67",
	"NTP":      "123",
	"RDP":      "3389",
	"VNC":      "5900",
	// HTTP and DNS are omitted: they may be parser-set (Protocol="HTTP"/"DNS"),
	// so FilterProtocol matches them correctly without a port mapping.
}

func (p *ProtocolDistWidget) SelectedFilter() *DisplayFilter {
	if len(p.sorted) == 0 || p.cursor < 0 || p.cursor >= len(p.sorted) {
		return nil
	}
	name := p.sorted[p.cursor].Name
	if port, ok := l7PrimaryPort[name]; ok {
		f := MakeFilter(FilterPort, port)
		return &f
	}
	f := MakeFilter(FilterProtocol, name)
	return &f
}

// effectiveProto returns the application-layer protocol name for a packet.
// For TCP/UDP, it checks well-known ports to return service names (HTTPS, SSH, etc.).
// For packets already tagged with a meaningful protocol by the parser, it returns as-is.
func effectiveProto(proto string, srcPort, dstPort uint16) string {
	if proto != "TCP" && proto != "UDP" {
		return proto // already meaningful: HTTP (parser-set), ICMP, ARP, etc.
	}
	// Check dest port first (outgoing service port)
	if name := servicePortName(dstPort); name != "" {
		return name
	}
	// Then src port (incoming / response)
	if name := servicePortName(srcPort); name != "" {
		return name
	}
	return proto
}

// servicePortName maps well-known port numbers to service labels.
// Returns "" for unrecognised ports. Mirrors protoLabel() in netgraph.go.
func servicePortName(port uint16) string {
	switch port {
	case 80, 8080:
		return "HTTP"
	case 443, 8443:
		return "HTTPS"
	case 22:
		return "SSH"
	case 53:
		return "DNS"
	case 21:
		return "FTP"
	case 25, 587:
		return "SMTP"
	case 465:
		return "SMTPS"
	case 110:
		return "POP3"
	case 143:
		return "IMAP"
	case 993:
		return "IMAPS"
	case 3306:
		return "MySQL"
	case 5432:
		return "Postgres"
	case 6379:
		return "Redis"
	case 27017:
		return "MongoDB"
	case 5353:
		return "mDNS"
	case 67, 68:
		return "DHCP"
	case 123:
		return "NTP"
	case 3389:
		return "RDP"
	case 5900:
		return "VNC"
	}
	return ""
}

func (p *ProtocolDistWidget) Update(msg tea.Msg) (Widget, tea.Cmd) {
	switch msg := msg.(type) {
	case events.PacketEvent:
		proto := effectiveProto(msg.Protocol, msg.SrcPort, msg.DstPort)
		if proto == "" {
			proto = "Unknown"
		}
		p.counts[proto]++
		p.total++
		p.rebuildSorted()

	case tea.MouseMsg:
		if !p.focused {
			return p, nil
		}
		switch msg.Type {
		case tea.MouseWheelUp:
			if p.cursor > 0 {
				p.cursor--
			}
		case tea.MouseWheelDown:
			if p.cursor < len(p.sorted)-1 {
				p.cursor++
			}
		}

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
