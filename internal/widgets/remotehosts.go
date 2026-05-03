package widgets

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/c0d343v3r/pkty/internal/events"
)

type hostEntry struct {
	IP      string
	Bytes   uint64
	Packets uint64
}

type RemoteHostsWidget struct {
	hosts    map[string]*hostEntry
	sorted   []*hostEntry
	ipToName map[string]string // IP → domain name (populated from DNS events)
	maxByte  uint64
	cursor   int
	offset   int
	focused  bool
	width    int
	height   int

	titleStyle    lipgloss.Style
	headerStyle   lipgloss.Style
	normalStyle   lipgloss.Style
	selectedStyle lipgloss.Style
	barStyle      lipgloss.Style
	borderStyle   lipgloss.Style
	focusBorder   lipgloss.Style
}

func NewRemoteHostsWidget() *RemoteHostsWidget {
	return &RemoteHostsWidget{
		hosts:         make(map[string]*hostEntry),
		ipToName:      make(map[string]string),
		titleStyle:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10")),
		headerStyle:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("236")),
		normalStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		selectedStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("25")),
		barStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		borderStyle:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("10")),
		focusBorder:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("226")),
	}
}

func (r *RemoteHostsWidget) Name() string     { return "Remote Hosts" }
func (r *RemoteHostsWidget) Init() tea.Cmd     { return nil }
func (r *RemoteHostsWidget) SetSize(w, h int)  { r.width = w; r.height = h }
func (r *RemoteHostsWidget) SetFocused(f bool) { r.focused = f }
func (r *RemoteHostsWidget) Focused() bool     { return r.focused }

// AddIPName records that an IP address resolves to the given domain name.
func (r *RemoteHostsWidget) AddIPName(ip, name string) {
	if ip != "" && name != "" {
		r.ipToName[ip] = name
	}
}

// displayName returns the domain name for an IP if known, otherwise the IP itself.
func (r *RemoteHostsWidget) displayName(ip string) string {
	if name, ok := r.ipToName[ip]; ok {
		return name
	}
	return ip
}

func (r *RemoteHostsWidget) SelectedFilter() *DisplayFilter {
	if len(r.sorted) == 0 || r.cursor < 0 || r.cursor >= len(r.sorted) {
		return nil
	}
	ip := r.sorted[r.cursor].IP
	// If we know the domain name, filter by DNS so all IPs for that domain match.
	if name, ok := r.ipToName[ip]; ok {
		f := MakeFilter(FilterDNS, name)
		return &f
	}
	f := MakeFilter(FilterIP, ip)
	return &f
}

func (r *RemoteHostsWidget) Update(msg tea.Msg) (Widget, tea.Cmd) {
	switch msg := msg.(type) {
	case events.PacketEvent:
		if msg.DstIP != nil {
			ip := msg.DstIP.String()
			h, ok := r.hosts[ip]
			if !ok {
				h = &hostEntry{IP: ip}
				r.hosts[ip] = h
			}
			h.Bytes += uint64(msg.Length)
			h.Packets++
			if h.Bytes > r.maxByte {
				r.maxByte = h.Bytes
			}
		}
		if msg.SrcIP != nil {
			ip := msg.SrcIP.String()
			h, ok := r.hosts[ip]
			if !ok {
				h = &hostEntry{IP: ip}
				r.hosts[ip] = h
			}
			h.Bytes += uint64(msg.Length)
			h.Packets++
			if h.Bytes > r.maxByte {
				r.maxByte = h.Bytes
			}
		}
		r.rebuildSorted()

	case tea.MouseMsg:
		if !r.focused {
			return r, nil
		}
		switch msg.Type {
		case tea.MouseWheelUp:
			if r.cursor > 0 {
				r.cursor--
				r.ensureVisible()
			}
		case tea.MouseWheelDown:
			if r.cursor < len(r.sorted)-1 {
				r.cursor++
				r.ensureVisible()
			}
		}

	case tea.KeyMsg:
		if !r.focused {
			return r, nil
		}
		switch msg.String() {
		case "up", "k":
			if r.cursor > 0 {
				r.cursor--
				r.ensureVisible()
			}
		case "down", "j":
			if r.cursor < len(r.sorted)-1 {
				r.cursor++
				r.ensureVisible()
			}
		case "enter":
			if f := r.SelectedFilter(); f != nil {
				return r, func() tea.Msg { return DisplayFilterToggleMsg{Filter: *f} }
			}
		}
	}
	return r, nil
}

func (r *RemoteHostsWidget) rebuildSorted() {
	r.sorted = make([]*hostEntry, 0, len(r.hosts))
	for _, h := range r.hosts {
		r.sorted = append(r.sorted, h)
	}
	sort.Slice(r.sorted, func(i, j int) bool {
		return r.sorted[i].Bytes > r.sorted[j].Bytes
	})
	if r.cursor >= len(r.sorted) {
		r.cursor = len(r.sorted) - 1
	}
	if r.cursor < 0 {
		r.cursor = 0
	}
}

func (r *RemoteHostsWidget) ensureVisible() {
	vis := r.height - 4
	if vis < 1 {
		vis = 1
	}
	if r.cursor < r.offset {
		r.offset = r.cursor
	}
	if r.cursor >= r.offset+vis {
		r.offset = r.cursor - vis + 1
	}
	if r.offset < 0 {
		r.offset = 0
	}
}

func (r *RemoteHostsWidget) View() string {
	cw := r.width - 2
	ch := r.height - 2
	if cw < 5 || ch < 2 {
		return r.borderStyle.Width(cw).Render("Hosts")
	}

	border := r.borderStyle
	if r.focused {
		border = r.focusBorder
	}

	title := r.titleStyle.Render(fmt.Sprintf(" Remote Hosts (%d)", len(r.hosts)))

	hostW := 24 // wider to fit domain names
	bytesW := 8
	barW := cw - hostW - bytesW - 3
	if barW < 3 {
		barW = 3
	}

	header := fmt.Sprintf("%-*s %*s %-*s", hostW, "Host", bytesW, "Bytes", barW, "Traffic")
	header = r.headerStyle.Width(cw).Render(truncStr(header, cw))

	lines := []string{title, header}
	vis := ch - 2
	for i := r.offset; i < r.offset+vis && i < len(r.sorted); i++ {
		h := r.sorted[i]
		filled := 0
		if r.maxByte > 0 {
			filled = int(float64(h.Bytes) / float64(r.maxByte) * float64(barW))
		}
		if filled < 0 {
			filled = 0
		}
		bar := r.barStyle.Render(strings.Repeat("█", filled) + strings.Repeat("░", barW-filled))
		line := fmt.Sprintf("%-*s %*s %s",
			hostW, truncStr(r.displayName(h.IP), hostW),
			bytesW, formatBytes(h.Bytes),
			bar)

		if i == r.cursor && r.focused {
			lines = append(lines, r.selectedStyle.Width(cw).Render(truncStr(line, cw)))
		} else {
			lines = append(lines, r.normalStyle.Width(cw).Render(truncStr(line, cw)))
		}
	}
	for len(lines) < ch {
		lines = append(lines, strings.Repeat(" ", cw))
	}
	return border.Width(cw).Render(strings.Join(lines, "\n"))
}
