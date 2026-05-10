package widgets

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/srixivas/pkty/internal/events"
)

type connKey struct {
	SrcIP   string
	DstIP   string
	SrcPort uint16
	DstPort uint16
	Proto   string
}

type connEntry struct {
	Key       connKey
	Bytes     uint64
	Packets   uint64
	LastFlags string
}

type ConnectionsWidget struct {
	conns   map[connKey]*connEntry
	sorted  []*connEntry
	cursor  int
	offset  int
	focused bool
	width   int
	height  int

	headerStyle   lipgloss.Style
	normalStyle   lipgloss.Style
	selectedStyle lipgloss.Style
	tcpStyle      lipgloss.Style
	udpStyle      lipgloss.Style
	borderStyle   lipgloss.Style
	focusBorder   lipgloss.Style
}

func NewConnectionsWidget() *ConnectionsWidget {
	return &ConnectionsWidget{
		conns:         make(map[connKey]*connEntry),
		headerStyle:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("236")),
		normalStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		selectedStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("25")),
		tcpStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("111")),
		udpStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("147")),
		borderStyle:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("14")),
		focusBorder:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("226")),
	}
}

func (c *ConnectionsWidget) Name() string    { return "Connections" }
func (c *ConnectionsWidget) Init() tea.Cmd   { return nil }
func (c *ConnectionsWidget) SetSize(w, h int) { c.width = w; c.height = h }
func (c *ConnectionsWidget) SetFocused(f bool) { c.focused = f }
func (c *ConnectionsWidget) Focused() bool     { return c.focused }

func (c *ConnectionsWidget) SelectedFilter() *DisplayFilter {
	if len(c.sorted) == 0 || c.cursor < 0 || c.cursor >= len(c.sorted) {
		return nil
	}
	e := c.sorted[c.cursor]
	f := MakeFilter(FilterIP, e.Key.DstIP)
	return &f
}

func (c *ConnectionsWidget) Update(msg tea.Msg) (Widget, tea.Cmd) {
	switch msg := msg.(type) {
	case events.PacketEvent:
		if msg.SrcIP == nil || msg.DstIP == nil {
			return c, nil
		}
		proto := msg.Protocol
		if proto != "TCP" && proto != "UDP" {
			return c, nil
		}
		key := connKey{
			SrcIP: msg.SrcIP.String(), DstIP: msg.DstIP.String(),
			SrcPort: msg.SrcPort, DstPort: msg.DstPort, Proto: proto,
		}
		entry, ok := c.conns[key]
		if !ok {
			entry = &connEntry{Key: key}
			c.conns[key] = entry
		}
		entry.Bytes += uint64(msg.Length)
		entry.Packets++
		if msg.Info != "" {
			if idx := strings.Index(msg.Info, "["); idx >= 0 {
				if end := strings.Index(msg.Info[idx:], "]"); end >= 0 {
					entry.LastFlags = msg.Info[idx+1 : idx+end]
				}
			}
		}
		c.rebuildSorted()

	case tea.MouseMsg:
		if !c.focused {
			return c, nil
		}
		switch msg.Type {
		case tea.MouseWheelUp:
			if c.cursor > 0 {
				c.cursor--
				c.ensureVisible()
			}
		case tea.MouseWheelDown:
			if c.cursor < len(c.sorted)-1 {
				c.cursor++
				c.ensureVisible()
			}
		}

	case tea.KeyMsg:
		if !c.focused {
			return c, nil
		}
		switch msg.String() {
		case "up", "k":
			if c.cursor > 0 {
				c.cursor--
				c.ensureVisible()
			}
		case "down", "j":
			if c.cursor < len(c.sorted)-1 {
				c.cursor++
				c.ensureVisible()
			}
		case "enter":
			if f := c.SelectedFilter(); f != nil {
				return c, func() tea.Msg { return DisplayFilterToggleMsg{Filter: *f} }
			}
		}
	}
	return c, nil
}

func (c *ConnectionsWidget) rebuildSorted() {
	c.sorted = make([]*connEntry, 0, len(c.conns))
	for _, e := range c.conns {
		c.sorted = append(c.sorted, e)
	}
	sort.Slice(c.sorted, func(i, j int) bool {
		return c.sorted[i].Bytes > c.sorted[j].Bytes
	})
	if c.cursor >= len(c.sorted) {
		c.cursor = len(c.sorted) - 1
	}
	if c.cursor < 0 {
		c.cursor = 0
	}
}

func (c *ConnectionsWidget) ensureVisible() {
	vis := c.visibleRows()
	if c.cursor < c.offset {
		c.offset = c.cursor
	}
	if c.cursor >= c.offset+vis {
		c.offset = c.cursor - vis + 1
	}
	if c.offset < 0 {
		c.offset = 0
	}
}

func (c *ConnectionsWidget) visibleRows() int {
	v := c.height - 4 // border(2) + title(1) + header(1)
	if v < 1 {
		return 1
	}
	return v
}

func (c *ConnectionsWidget) View() string {
	cw := c.width - 2
	ch := c.height - 2
	if cw < 5 || ch < 2 {
		return c.borderStyle.Width(cw).Render("Conn")
	}

	border := c.borderStyle
	if c.focused {
		border = c.focusBorder
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14")).
		Render(fmt.Sprintf(" Connections (%d)", len(c.conns)))

	header := c.headerStyle.Width(cw).Render(truncStr(
		fmt.Sprintf("%-*s %5s %7s", cw-14, "Remote", "Proto", "Bytes"), cw))

	lines := []string{title, header}
	vis := ch - 2
	for i := c.offset; i < c.offset+vis && i < len(c.sorted); i++ {
		e := c.sorted[i]
		remote := fmt.Sprintf("%s:%d", e.Key.DstIP, e.Key.DstPort)
		bytesStr := formatBytes(e.Bytes)
		line := fmt.Sprintf("%-*s %5s %7s",
			cw-14, truncStr(remote, cw-14), e.Key.Proto, bytesStr)

		if i == c.cursor && c.focused {
			lines = append(lines, c.selectedStyle.Width(cw).Render(truncStr(line, cw)))
		} else {
			style := c.normalStyle
			if e.Key.Proto == "TCP" {
				style = c.tcpStyle
			} else if e.Key.Proto == "UDP" {
				style = c.udpStyle
			}
			lines = append(lines, style.Width(cw).Render(truncStr(line, cw)))
		}
	}
	for len(lines) < ch {
		lines = append(lines, strings.Repeat(" ", cw))
	}
	return border.Width(cw).Render(strings.Join(lines, "\n"))
}

func formatBytes(b uint64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%dB", b)
	}
}
