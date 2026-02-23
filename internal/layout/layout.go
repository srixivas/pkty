package layout

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/c0d343v3r/netdash/internal/config"
	"github.com/c0d343v3r/netdash/internal/events"
	"github.com/c0d343v3r/netdash/internal/widgets"
)

// Msg types that bridge EventBus channels into bubbletea.
type PacketMsg events.PacketEvent
type DNSMsg events.DNSEvent
type TLSMsg events.TLSEvent
type HTTPMsg events.HTTPEvent
type CaptureErrMsg struct{ Err error }

const (
	FocusCentre = 0
	FocusLeft   = 1
	FocusRight  = 2
	FocusBottom = 3
)

// bottomFocus indices for the three bottom widgets.
const (
	BottomProtoDist   = 0
	BottomRemoteHosts = 1
	BottomTLS         = 2
)

type Model struct {
	cfg    *config.Config
	width  int
	height int

	// Widgets
	inspector   *widgets.PacketInspector
	connections *widgets.ConnectionsWidget
	dns         *widgets.DNSWidget
	bandwidth   *widgets.BandwidthWidget
	protoDist   *widgets.ProtocolDistWidget
	remoteHosts *widgets.RemoteHostsWidget
	tlsInspect  *widgets.TLSInspectorWidget
	filterBar   *widgets.FilterBar

	displayFilters *widgets.DisplayFilterSet

	bus           *events.EventBus
	OnFilterApply func(expr string) error

	packetCount uint64
	captureErr  error
	focusTarget int
	bottomFocus int
	statusStyle    lipgloss.Style
	filterStyle    lipgloss.Style
	displayBarStyle lipgloss.Style
	displayBarActive lipgloss.Style
}

func New(cfg *config.Config, bus *events.EventBus) Model {
	ds := widgets.NewDisplayFilterSet()
	insp := widgets.NewPacketInspector()
	insp.List().SetDisplayFilter(ds)

	m := Model{
		cfg:            cfg,
		bus:            bus,
		inspector:      insp,
		connections:    widgets.NewConnectionsWidget(),
		dns:            widgets.NewDNSWidget(),
		bandwidth:      widgets.NewBandwidthWidget(),
		protoDist:      widgets.NewProtocolDistWidget(),
		remoteHosts:    widgets.NewRemoteHostsWidget(),
		tlsInspect:     widgets.NewTLSInspectorWidget(),
		filterBar:      widgets.NewFilterBar(),
		displayFilters: ds,
		focusTarget:    FocusCentre,
		bottomFocus:    BottomProtoDist,
		statusStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("236")).
			Padding(0, 1),
		filterStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("226")).
			Background(lipgloss.Color("236")).
			Padding(0, 1),
		displayBarStyle: lipgloss.NewStyle().
			Background(lipgloss.Color("234")).
			Padding(0, 1),
		displayBarActive: lipgloss.NewStyle().
			Background(lipgloss.Color("234")).
			Padding(0, 1),
	}
	m.updateFocus()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.listenPackets(),
		m.listenDNS(),
		m.listenTLS(),
		m.listenHTTP(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// BPF filter bar eats all input when active
		if m.filterBar.Active() {
			var cmd tea.Cmd
			m.filterBar, cmd = m.filterBar.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "/":
			m.filterBar.Activate()
			return m, nil

		case "D":
			// Clear all display filters
			m.displayFilters.Clear()
			m.inspector.List().RebuildFiltered()
			return m, nil

		case "1":
			m.focusTarget = FocusCentre
			m.updateFocus()
			return m, nil
		case "2":
			m.focusTarget = FocusLeft
			m.updateFocus()
			return m, nil
		case "3":
			m.focusTarget = FocusRight
			m.updateFocus()
			return m, nil
		case "4":
			m.focusTarget = FocusBottom
			m.updateFocus()
			return m, nil

		case "tab":
			if m.focusTarget == FocusBottom {
				m.bottomFocus = (m.bottomFocus + 1) % 3
				m.updateFocus()
				return m, nil
			}
			// Fall through to route to focused widget (inspector handles sub-pane cycling)
			fallthrough

		default:
			cmd := m.routeKey(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case widgets.DisplayFilterToggleMsg:
		m.displayFilters.Toggle(msg.Filter)
		m.inspector.List().RebuildFiltered()

	case widgets.FilterApplyMsg:
		if m.OnFilterApply != nil {
			if err := m.OnFilterApply(msg.Expression); err != nil {
				m.filterBar.SetError(err.Error())
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcSizes()

	case PacketMsg:
		m.packetCount++
		evt := events.PacketEvent(msg)

		w, cmd := m.inspector.Update(evt)
		m.inspector = w.(*widgets.PacketInspector)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

		m.connections.Update(evt)
		m.bandwidth.Update(evt)
		m.protoDist.Update(evt)
		m.remoteHosts.Update(evt)

		cmds = append(cmds, m.listenPackets())

	case DNSMsg:
		evt := events.DNSEvent(msg)
		m.dns.Update(evt)
		if evt.ResolvedIP != nil {
			m.displayFilters.AddDNSMapping(evt.QueryName, evt.ResolvedIP)
			m.remoteHosts.AddIPName(evt.ResolvedIP.String(), evt.QueryName)
		}
		cmds = append(cmds, m.listenDNS())

	case TLSMsg:
		evt := events.TLSEvent(msg)
		m.tlsInspect.Update(evt)
		if evt.SNI != "" {
			m.displayFilters.AddSNIMapping(evt.SNI, evt.DstIP)
		}
		cmds = append(cmds, m.listenTLS())

	case HTTPMsg:
		cmds = append(cmds, m.listenHTTP())

	case CaptureErrMsg:
		m.captureErr = msg.Err
	}

	return m, tea.Batch(cmds...)
}

// routeKey sends a keyboard message to the currently focused widget.
func (m *Model) routeKey(msg tea.KeyMsg) tea.Cmd {
	switch m.focusTarget {
	case FocusCentre:
		var cmd tea.Cmd
		var w widgets.Widget
		w, cmd = m.inspector.Update(msg)
		m.inspector = w.(*widgets.PacketInspector)
		return cmd

	case FocusLeft:
		var cmd tea.Cmd
		var w widgets.Widget
		w, cmd = m.connections.Update(msg)
		m.connections = w.(*widgets.ConnectionsWidget)
		return cmd

	case FocusRight:
		var cmd tea.Cmd
		var w widgets.Widget
		w, cmd = m.dns.Update(msg)
		m.dns = w.(*widgets.DNSWidget)
		return cmd

	case FocusBottom:
		switch m.bottomFocus {
		case BottomProtoDist:
			var cmd tea.Cmd
			var w widgets.Widget
			w, cmd = m.protoDist.Update(msg)
			m.protoDist = w.(*widgets.ProtocolDistWidget)
			return cmd
		case BottomRemoteHosts:
			var cmd tea.Cmd
			var w widgets.Widget
			w, cmd = m.remoteHosts.Update(msg)
			m.remoteHosts = w.(*widgets.RemoteHostsWidget)
			return cmd
		case BottomTLS:
			var cmd tea.Cmd
			var w widgets.Widget
			w, cmd = m.tlsInspect.Update(msg)
			m.tlsInspect = w.(*widgets.TLSInspectorWidget)
			return cmd
		}
	}
	return nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	if m.captureErr != nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).Bold(true).Padding(2, 4).
			Render(fmt.Sprintf("Capture Error:\n\n%s", m.captureErr))
	}

	// Status bar
	var statusBar string
	m.filterBar.SetWidth(m.width)
	if m.filterBar.Active() {
		statusBar = m.statusStyle.Width(m.width).Render(m.filterBar.View())
	} else {
		pane := m.inspector.PaneName()
		filterView := m.filterBar.View()

		focusHint := focusName(m.focusTarget, m.bottomFocus)
		statusText := fmt.Sprintf(" netdash  |  Packets: %d  |  [%s]  |  Focus: %s  |  1-4: panels  tab: sub  Enter: filter  D: clear  /: bpf  q: quit",
			m.packetCount, pane, focusHint)
		if filterView != "" {
			statusText += "  |  " + filterView
		}
		statusBar = m.statusStyle.Width(m.width).Render(statusText)
	}

	bottomH := m.cfg.Layout.BottomPanelHeight
	mainH := m.height - bottomH - 2 // -1 statusBar, -1 displayFilterBar
	if mainH < 5 {
		mainH = 5
	}

	leftW := m.cfg.Layout.LeftPanelWidth
	rightW := m.cfg.Layout.RightPanelWidth
	centreW := m.width - leftW - rightW
	if centreW < 20 {
		centreW = 20
	}

	// Left panel: connections
	m.connections.SetSize(leftW, mainH)
	leftView := m.connections.View()

	// Right panel: DNS (top half) + Bandwidth (bottom half)
	rightTopH := mainH / 2
	rightBotH := mainH - rightTopH
	m.dns.SetSize(rightW, rightTopH)
	m.bandwidth.SetSize(rightW, rightBotH)
	rightView := lipgloss.JoinVertical(lipgloss.Left,
		m.dns.View(),
		m.bandwidth.View(),
	)

	// Centre panel
	m.inspector.SetSize(centreW, mainH)
	centreView := m.inspector.View()

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, leftView, centreView, rightView)
	topRow = lipgloss.NewStyle().Height(mainH).MaxHeight(mainH).Render(topRow)

	// Bottom: protocol dist | remote hosts | TLS inspector
	botW := m.width / 3
	botWLast := m.width - botW*2
	m.protoDist.SetSize(botW, bottomH)
	m.remoteHosts.SetSize(botW, bottomH)
	m.tlsInspect.SetSize(botWLast, bottomH)
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top,
		m.protoDist.View(),
		m.remoteHosts.View(),
		m.tlsInspect.View(),
	)

	displayBar := m.renderDisplayFilterBar()
	return lipgloss.JoinVertical(lipgloss.Left, displayBar, topRow, bottomRow, statusBar)
}

func (m *Model) updateFocus() {
	m.inspector.SetFocused(m.focusTarget == FocusCentre)
	m.connections.SetFocused(m.focusTarget == FocusLeft)
	m.dns.SetFocused(m.focusTarget == FocusRight)
	m.protoDist.SetFocused(m.focusTarget == FocusBottom && m.bottomFocus == BottomProtoDist)
	m.remoteHosts.SetFocused(m.focusTarget == FocusBottom && m.bottomFocus == BottomRemoteHosts)
	m.tlsInspect.SetFocused(m.focusTarget == FocusBottom && m.bottomFocus == BottomTLS)
}

func focusName(target, bottom int) string {
	switch target {
	case FocusCentre:
		return "Centre"
	case FocusLeft:
		return "Connections"
	case FocusRight:
		return "DNS"
	case FocusBottom:
		switch bottom {
		case BottomProtoDist:
			return "Proto Dist"
		case BottomRemoteHosts:
			return "Remote Hosts"
		case BottomTLS:
			return "TLS"
		}
	}
	return "?"
}

func (m Model) listenPackets() tea.Cmd {
	bus := m.bus
	if bus == nil {
		return nil
	}
	return func() tea.Msg {
		evt, ok := <-bus.Packets
		if !ok {
			return CaptureErrMsg{Err: fmt.Errorf("packet channel closed")}
		}
		return PacketMsg(evt)
	}
}

func (m Model) listenDNS() tea.Cmd {
	bus := m.bus
	if bus == nil {
		return nil
	}
	return func() tea.Msg {
		evt, ok := <-bus.DNS
		if !ok {
			return nil
		}
		return DNSMsg(evt)
	}
}

func (m Model) listenTLS() tea.Cmd {
	bus := m.bus
	if bus == nil {
		return nil
	}
	return func() tea.Msg {
		evt, ok := <-bus.TLS
		if !ok {
			return nil
		}
		return TLSMsg(evt)
	}
}

func (m Model) listenHTTP() tea.Cmd {
	bus := m.bus
	if bus == nil {
		return nil
	}
	return func() tea.Msg {
		evt, ok := <-bus.HTTP
		if !ok {
			return nil
		}
		return HTTPMsg(evt)
	}
}

func (m *Model) recalcSizes() {
	// Sizes are set directly in View() now for precision
}

// displayFilterBar renders the top-level 1-line filter state bar.
func (m Model) renderDisplayFilterBar() string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33"))
	label := labelStyle.Render("Display: ")

	var content string
	if m.displayFilters.Active() {
		summaryStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("226"))
		hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		content = summaryStyle.Render(m.displayFilters.Summary()) +
			hintStyle.Render("  [D: clear]")
		return m.displayBarActive.Width(m.width).Render(label + content)
	}

	wildcardStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	content = wildcardStyle.Render("◆ all traffic")
	return m.displayBarStyle.Width(m.width).Render(label + content)
}
