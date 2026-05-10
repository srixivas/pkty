package widgets

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/srixivas/pkty/internal/events"
)

type dnsRow struct {
	QueryName  string
	RecordType string
	ResolvedIP string
	TTL        string
	IsNX       bool
	IsResponse bool
}

type DNSWidget struct {
	rows    []dnsRow
	maxRows int
	cursor  int
	offset  int
	focused bool
	width   int
	height  int

	headerStyle   lipgloss.Style
	normalStyle   lipgloss.Style
	selectedStyle lipgloss.Style
	nxStyle       lipgloss.Style
	borderStyle   lipgloss.Style
	focusBorder   lipgloss.Style
}

func NewDNSWidget() *DNSWidget {
	return &DNSWidget{
		maxRows:       500,
		headerStyle:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("236")),
		normalStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("114")),
		selectedStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("25")),
		nxStyle:       lipgloss.NewStyle().Foreground(lipgloss.Color("222")),
		borderStyle:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("11")),
		focusBorder:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("226")),
	}
}

func (d *DNSWidget) Name() string     { return "DNS Queries" }
func (d *DNSWidget) Init() tea.Cmd    { return nil }
func (d *DNSWidget) SetSize(w, h int) { d.width = w; d.height = h }
func (d *DNSWidget) SetFocused(f bool) { d.focused = f }
func (d *DNSWidget) Focused() bool     { return d.focused }

func (d *DNSWidget) SelectedFilter() *DisplayFilter {
	if len(d.rows) == 0 || d.cursor < 0 || d.cursor >= len(d.rows) {
		return nil
	}
	f := MakeFilter(FilterDNS, d.rows[d.cursor].QueryName)
	return &f
}

func (d *DNSWidget) Update(msg tea.Msg) (Widget, tea.Cmd) {
	switch msg := msg.(type) {
	case events.DNSEvent:
		ip := ""
		if msg.ResolvedIP != nil {
			ip = msg.ResolvedIP.String()
		}
		ttl := ""
		if msg.IsResponse {
			ttl = fmt.Sprintf("%d", msg.TTL)
		}
		d.rows = append(d.rows, dnsRow{
			QueryName:  msg.QueryName,
			RecordType: msg.RecordType,
			ResolvedIP: ip,
			TTL:        ttl,
			IsNX:       msg.IsNXDomain,
			IsResponse: msg.IsResponse,
		})
		if len(d.rows) > d.maxRows {
			d.rows = d.rows[len(d.rows)-d.maxRows:]
		}
		// Auto-scroll only when not focused
		if !d.focused {
			vis := d.height - 4
			if vis < 1 {
				vis = 1
			}
			if len(d.rows) > vis {
				d.offset = len(d.rows) - vis
			}
			d.cursor = len(d.rows) - 1
		}

	case tea.MouseMsg:
		if !d.focused {
			return d, nil
		}
		switch msg.Type {
		case tea.MouseWheelUp:
			if d.cursor > 0 {
				d.cursor--
				d.ensureVisible()
			}
		case tea.MouseWheelDown:
			if d.cursor < len(d.rows)-1 {
				d.cursor++
				d.ensureVisible()
			}
		}

	case tea.KeyMsg:
		if !d.focused {
			return d, nil
		}
		switch msg.String() {
		case "up", "k":
			if d.cursor > 0 {
				d.cursor--
				d.ensureVisible()
			}
		case "down", "j":
			if d.cursor < len(d.rows)-1 {
				d.cursor++
				d.ensureVisible()
			}
		case "enter":
			if f := d.SelectedFilter(); f != nil {
				return d, func() tea.Msg { return DisplayFilterToggleMsg{Filter: *f} }
			}
		}
	}
	return d, nil
}

func (d *DNSWidget) ensureVisible() {
	vis := d.height - 4
	if vis < 1 {
		vis = 1
	}
	if d.cursor < d.offset {
		d.offset = d.cursor
	}
	if d.cursor >= d.offset+vis {
		d.offset = d.cursor - vis + 1
	}
	if d.offset < 0 {
		d.offset = 0
	}
}

func (d *DNSWidget) View() string {
	cw := d.width - 2
	ch := d.height - 2
	if cw < 5 || ch < 2 {
		return d.borderStyle.Width(cw).Render("DNS")
	}

	border := d.borderStyle
	if d.focused {
		border = d.focusBorder
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render(" DNS Queries")
	colType := 6
	colIP := 16
	colName := cw - colType - colIP - 3
	if colName < 8 {
		colName = 8
	}

	header := fmt.Sprintf("%-*s %-*s %-*s", colName, "Name", colType, "Type", colIP, "Resolved")
	header = d.headerStyle.Width(cw).Render(truncStr(header, cw))

	lines := []string{title, header}
	vis := ch - 2
	for i := d.offset; i < d.offset+vis && i < len(d.rows); i++ {
		r := d.rows[i]
		ip := r.ResolvedIP
		if r.IsNX {
			ip = "NXDOMAIN"
		}
		line := fmt.Sprintf("%-*s %-*s %-*s",
			colName, truncStr(r.QueryName, colName),
			colType, r.RecordType,
			colIP, truncStr(ip, colIP))

		if i == d.cursor && d.focused {
			lines = append(lines, d.selectedStyle.Width(cw).Render(truncStr(line, cw)))
		} else {
			style := d.normalStyle
			if r.IsNX {
				style = d.nxStyle
			}
			lines = append(lines, style.Width(cw).Render(truncStr(line, cw)))
		}
	}
	for len(lines) < ch {
		lines = append(lines, strings.Repeat(" ", cw))
	}
	return border.Width(cw).Render(strings.Join(lines, "\n"))
}
