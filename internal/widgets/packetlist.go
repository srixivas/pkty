package widgets

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/srixivas/pkty/internal/events"
)

// PacketRow holds the display data for one packet in the list.
type PacketRow struct {
	Number    uint64
	Timestamp time.Time
	SrcAddr   string
	DstAddr   string
	Protocol  string
	Length    int
	Info      string
	Event     events.PacketEvent
}

// PacketList is the scrollable packet table widget.
type PacketList struct {
	rows         []PacketRow
	filteredRows []PacketRow
	displayFilter *DisplayFilterSet
	cursor       int
	offset       int
	width        int
	height       int
	focused      bool
	maxRows      int
	startTime    time.Time

	OnSelect func(PacketRow)

	headerStyle    lipgloss.Style
	normalStyle    lipgloss.Style
	selectedStyle  lipgloss.Style
	borderStyle    lipgloss.Style
	protocolStyles map[string]lipgloss.Style
}

func NewPacketList() *PacketList {
	return &PacketList{
		maxRows: 50000,
		focused: true,
		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("236")),
		normalStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),
		selectedStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("25")),
		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")),
		protocolStyles: map[string]lipgloss.Style{
			"TCP":  lipgloss.NewStyle().Foreground(lipgloss.Color("111")),
			"UDP":  lipgloss.NewStyle().Foreground(lipgloss.Color("147")),
			"DNS":  lipgloss.NewStyle().Foreground(lipgloss.Color("114")),
			"HTTP": lipgloss.NewStyle().Foreground(lipgloss.Color("215")),
			"TLS":  lipgloss.NewStyle().Foreground(lipgloss.Color("213")),
			"ARP":  lipgloss.NewStyle().Foreground(lipgloss.Color("222")),
			"ICMP": lipgloss.NewStyle().Foreground(lipgloss.Color("117")),
		},
	}
}

func (p *PacketList) Name() string      { return "Packet List" }
func (p *PacketList) Init() tea.Cmd     { return nil }
func (p *PacketList) SetFocused(f bool) { p.focused = f }
func (p *PacketList) Focused() bool     { return p.focused }
func (p *PacketList) SetSize(w, h int)  { p.width = w; p.height = h }

// SetDisplayFilter wires a DisplayFilterSet to this list and rebuilds the view.
func (p *PacketList) SetDisplayFilter(ds *DisplayFilterSet) {
	p.displayFilter = ds
	p.RebuildFiltered()
}

// RebuildFiltered recomputes filteredRows from all rows using the current filter.
// Call this whenever filters change.
func (p *PacketList) RebuildFiltered() {
	p.filteredRows = p.filteredRows[:0]
	active := p.displayFilter != nil && p.displayFilter.Active()
	for _, row := range p.rows {
		if !active || p.displayFilter.Match(row) {
			p.filteredRows = append(p.filteredRows, row)
		}
	}
	// Jump to bottom of filtered view
	if len(p.filteredRows) > 0 {
		p.cursor = len(p.filteredRows) - 1
	} else {
		p.cursor = 0
	}
	p.offset = 0
	p.ensureVisible()
	p.notifySelect()
}

// activeRows returns the row slice to use for display and navigation.
func (p *PacketList) activeRows() []PacketRow {
	if p.displayFilter != nil && p.displayFilter.Active() {
		return p.filteredRows
	}
	return p.rows
}

func (p *PacketList) SelectedPacket() *PacketRow {
	rows := p.activeRows()
	if p.cursor >= 0 && p.cursor < len(rows) {
		return &rows[p.cursor]
	}
	return nil
}

func (p *PacketList) Update(msg tea.Msg) (Widget, tea.Cmd) {
	switch msg := msg.(type) {
	case events.PacketEvent:
		p.addPacket(msg)
	case tea.MouseMsg:
		if !p.focused {
			return p, nil
		}
		rows := p.activeRows()
		switch msg.Type {
		case tea.MouseWheelUp:
			if p.cursor > 0 {
				p.cursor--
				p.ensureVisible()
				p.notifySelect()
			}
		case tea.MouseWheelDown:
			if p.cursor < len(rows)-1 {
				p.cursor++
				p.ensureVisible()
				p.notifySelect()
			}
		}

	case tea.KeyMsg:
		if !p.focused {
			return p, nil
		}
		rows := p.activeRows()
		switch msg.String() {
		case "up", "k":
			if p.cursor > 0 {
				p.cursor--
				p.ensureVisible()
				p.notifySelect()
			}
		case "down", "j":
			if p.cursor < len(rows)-1 {
				p.cursor++
				p.ensureVisible()
				p.notifySelect()
			}
		case "pgup":
			p.cursor -= p.visibleRows()
			if p.cursor < 0 {
				p.cursor = 0
			}
			p.ensureVisible()
			p.notifySelect()
		case "pgdown":
			p.cursor += p.visibleRows()
			if p.cursor >= len(rows) {
				p.cursor = len(rows) - 1
			}
			if p.cursor < 0 {
				p.cursor = 0
			}
			p.ensureVisible()
			p.notifySelect()
		case "home", "g":
			p.cursor = 0
			p.ensureVisible()
			p.notifySelect()
		case "end", "G":
			if len(rows) > 0 {
				p.cursor = len(rows) - 1
			}
			p.ensureVisible()
			p.notifySelect()
		}
	}
	return p, nil
}

func (p *PacketList) addPacket(evt events.PacketEvent) {
	firstPacket := len(p.rows) == 0
	if p.startTime.IsZero() {
		p.startTime = evt.Timestamp
	}

	src, dst := "", ""
	if evt.SrcIP != nil {
		src = evt.SrcIP.String()
		if evt.SrcPort > 0 {
			src = fmt.Sprintf("%s:%d", src, evt.SrcPort)
		}
	}
	if evt.DstIP != nil {
		dst = evt.DstIP.String()
		if evt.DstPort > 0 {
			dst = fmt.Sprintf("%s:%d", dst, evt.DstPort)
		}
	}

	row := PacketRow{
		Number: evt.Number, Timestamp: evt.Timestamp,
		SrcAddr: src, DstAddr: dst, Protocol: evt.Protocol,
		Length: evt.Length, Info: evt.Info, Event: evt,
	}

	p.rows = append(p.rows, row)

	if len(p.rows) > p.maxRows {
		trim := len(p.rows) - p.maxRows
		p.rows = p.rows[trim:]
		filterActive := p.displayFilter != nil && p.displayFilter.Active()
		if !filterActive {
			p.cursor -= trim
			if p.cursor < 0 {
				p.cursor = 0
			}
			p.offset -= trim
			if p.offset < 0 {
				p.offset = 0
			}
		}
	}

	filterActive := p.displayFilter != nil && p.displayFilter.Active()
	if filterActive {
		if p.displayFilter.Match(row) {
			p.filteredRows = append(p.filteredRows, row)
			if len(p.filteredRows) > p.maxRows {
				trim := len(p.filteredRows) - p.maxRows
				p.filteredRows = p.filteredRows[trim:]
				p.cursor -= trim
				if p.cursor < 0 {
					p.cursor = 0
				}
				p.offset -= trim
				if p.offset < 0 {
					p.offset = 0
				}
			}
			if p.cursor >= len(p.filteredRows)-2 {
				p.cursor = len(p.filteredRows) - 1
				p.ensureVisible()
			}
		}
	} else {
		if p.cursor >= len(p.rows)-2 {
			p.cursor = len(p.rows) - 1
			p.ensureVisible()
		}
	}

	if firstPacket {
		p.notifySelect()
	}
}

func (p *PacketList) View() string {
	// contentW/contentH = space inside the border
	contentW := p.width - 2
	contentH := p.height - 2
	if contentW < 10 || contentH < 2 {
		return p.borderStyle.Width(contentW).Render("...")
	}

	colNo := 7
	colTime := 10
	colProto := 6
	colLen := 6
	colAddr := (contentW - colNo - colTime - colProto - colLen - 6) / 3
	if colAddr < 12 {
		colAddr = 12
	}
	colInfo := contentW - colNo - colTime - colAddr*2 - colProto - colLen - 6
	if colInfo < 5 {
		colInfo = 5
	}

	header := fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s %*s %-*s",
		colNo, "No.", colTime, "Time", colAddr, "Source", colAddr, "Dest",
		colProto, "Proto", colLen, "Len", colInfo, "Info")
	header = p.headerStyle.Width(contentW).Render(truncStr(header, contentW))

	rows := p.activeRows()
	visible := contentH - 1
	lines := []string{header}

	for i := p.offset; i < p.offset+visible && i < len(rows); i++ {
		row := rows[i]
		elapsed := row.Timestamp.Sub(p.startTime).Seconds()
		timeStr := fmt.Sprintf("%.4f", elapsed)

		line := fmt.Sprintf("%-*d %-*s %-*s %-*s %-*s %*d %-*s",
			colNo, row.Number, colTime, truncStr(timeStr, colTime),
			colAddr, truncStr(row.SrcAddr, colAddr), colAddr, truncStr(row.DstAddr, colAddr),
			colProto, truncStr(row.Protocol, colProto), colLen, row.Length,
			colInfo, truncStr(row.Info, colInfo))

		if i == p.cursor && p.focused {
			lines = append(lines, p.selectedStyle.Width(contentW).Render(truncStr(line, contentW)))
		} else {
			style := p.normalStyle
			if ps, ok := p.protocolStyles[row.Protocol]; ok {
				style = ps
			}
			lines = append(lines, style.Width(contentW).Render(truncStr(line, contentW)))
		}
	}

	for len(lines) < contentH {
		lines = append(lines, strings.Repeat(" ", contentW))
	}

	return p.borderStyle.Width(contentW).Render(strings.Join(lines, "\n"))
}

func (p *PacketList) visibleRows() int {
	v := p.height - 3
	if v < 1 {
		return 1
	}
	return v
}

func (p *PacketList) ensureVisible() {
	rows := p.activeRows()
	visible := p.visibleRows()
	if p.cursor < p.offset {
		p.offset = p.cursor
	}
	if p.cursor >= p.offset+visible {
		p.offset = p.cursor - visible + 1
	}
	if p.offset < 0 {
		p.offset = 0
	}
	_ = rows
}

func (p *PacketList) notifySelect() {
	rows := p.activeRows()
	if p.OnSelect != nil && p.cursor >= 0 && p.cursor < len(rows) {
		p.OnSelect(rows[p.cursor])
	}
}

// AllRows returns all packet rows (unfiltered) for export purposes.
func (p *PacketList) AllRows() []PacketRow {
	return p.rows
}

func truncStr(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "~"
}
