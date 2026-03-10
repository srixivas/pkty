package layout

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/gopacket/layers"
	"github.com/c0d343v3r/netdash/internal/config"
	"github.com/c0d343v3r/netdash/internal/events"
	"github.com/c0d343v3r/netdash/internal/resolve"
	"github.com/c0d343v3r/netdash/internal/session"
	"github.com/c0d343v3r/netdash/internal/store"
	"github.com/c0d343v3r/netdash/internal/widgets"
)

// AppName is the canonical display name of the application.
// Change this once here when the name is finalised.
const AppName = "netdash"

type animTickMsg time.Time

func animTickCmd() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
		return animTickMsg(t)
	})
}

// Msg types that bridge EventBus channels into bubbletea.
type PacketMsg events.PacketEvent
type DNSMsg events.DNSEvent
type TLSMsg events.TLSEvent
type HTTPMsg events.HTTPEvent
type CaptureErrMsg struct{ Err error }

// SaveStatusMsg carries the result of an S-key pcap save.
type SaveStatusMsg struct{ Text string }

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
	boar        *widgets.BoarWidget
	inspector   *widgets.PacketInspector
	netGraph    *widgets.NetGraphWidget
	connections *widgets.ConnectionsWidget
	dns         *widgets.DNSWidget
	bandwidth   *widgets.BandwidthWidget
	protoDist   *widgets.ProtocolDistWidget
	remoteHosts *widgets.RemoteHostsWidget
	tlsInspect  *widgets.TLSInspectorWidget
	filterBar   *widgets.FilterBar
	searchBar   *widgets.SearchBar

	displayFilters *widgets.DisplayFilterSet

	bus           *events.EventBus
	OnFilterApply func(expr string) error

	// Persistence
	linkType    layers.LinkType
	saveDir     string
	sqliteStore *store.SQLiteStore
	saveStatus  string

	// Reverse DNS
	resolver *resolve.Resolver

	// Capture state
	capturing  bool
	hasBackend bool

	packetCount uint64
	captureErr  error
	focusTarget int
	bottomFocus int
	centreMode  int // 0 = PacketInspector, 1 = NetGraph
	statusStyle     lipgloss.Style
	filterStyle     lipgloss.Style
	displayBarStyle  lipgloss.Style
	displayBarActive lipgloss.Style
}

// SetLinkType sets the pcap link-layer type used when saving captures.
func (m *Model) SetLinkType(lt layers.LinkType) { m.linkType = lt }

// SetSQLiteStore wires an optional SQLite store for continuous event logging.
func (m *Model) SetSQLiteStore(s *store.SQLiteStore) { m.sqliteStore = s }

// SetCapturing sets the initial capture state (true = active, false = paused).
// It also marks that a backend is present.
func (m *Model) SetCapturing(active bool) {
	m.capturing = active
	m.hasBackend = true
	m.boar.SetCapturing(active)
	m.boar.SetHasBackend(true)
}

func New(cfg *config.Config, bus *events.EventBus) Model {
	ds := widgets.NewDisplayFilterSet()
	insp := widgets.NewPacketInspector()
	insp.List().SetDisplayFilter(ds)

	m := Model{
		cfg:            cfg,
		bus:            bus,
		saveDir:        session.DefaultSaveDir(),
		resolver:       resolve.New(),
		boar:           widgets.NewBoarWidget(),
		inspector:      insp,
		netGraph:       func() *widgets.NetGraphWidget {
			ng := widgets.NewNetGraphWidget()
			ng.SetSourceName(cfg.Capture.Interface)
			ng.SetDisplayFilter(ds)
			return ng
		}(),
		connections:    widgets.NewConnectionsWidget(),
		dns:            widgets.NewDNSWidget(),
		bandwidth:      widgets.NewBandwidthWidget(),
		protoDist:      widgets.NewProtocolDistWidget(),
		remoteHosts:    widgets.NewRemoteHostsWidget(),
		tlsInspect:     widgets.NewTLSInspectorWidget(),
		filterBar:      widgets.NewFilterBar(),
		searchBar:      widgets.NewSearchBar(),
		displayFilters: ds,
		focusTarget:    FocusCentre,
		bottomFocus:    BottomProtoDist,
		centreMode:     0,
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
		animTickCmd(),
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

		// Search bar eats all input when active
		if m.searchBar.Active() {
			var cmd tea.Cmd
			m.searchBar, cmd = m.searchBar.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "S":
			cmds = append(cmds, m.savePcapCmd())
			return m, tea.Batch(cmds...)

		case "/":
			m.filterBar.Activate()
			return m, nil

		case "?":
			m.searchBar.Activate()
			return m, nil

		case "D":
			// Clear all display filters
			m.displayFilters.Clear()
			m.inspector.List().RebuildFiltered()
			m.netGraph.RebuildVisible()
			return m, nil

		case " ":
			if m.hasBackend {
				m.capturing = !m.capturing
				m.boar.SetCapturing(m.capturing)
			}
			return m, tea.Batch(cmds...)

		case "n":
			m.centreMode = (m.centreMode + 1) % 2
			m.updateFocus()
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

	case tea.MouseMsg:
		if msg.Type == tea.MouseWheelUp || msg.Type == tea.MouseWheelDown {
			if cmd := m.routeKey(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case animTickMsg:
		if m.capturing {
			m.boar.Advance()
		}
		cmds = append(cmds, animTickCmd())

	case SaveStatusMsg:
		m.saveStatus = msg.Text

	case widgets.DisplayFilterToggleMsg:
		m.displayFilters.Toggle(msg.Filter)
		m.inspector.List().RebuildFiltered()
		m.netGraph.RebuildVisible()

	case widgets.SearchApplyMsg:
		if f, ok := widgets.ParseSearchPattern(msg.Pattern); ok {
			m.displayFilters.Toggle(f)
			m.inspector.List().RebuildFiltered()
			m.netGraph.RebuildVisible()
		}

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
		evt := events.PacketEvent(msg)
		cmds = append(cmds, m.listenPackets()) // always drain to prevent channel blocking

		if !m.capturing {
			break // discard packet; widgets stay frozen
		}

		m.packetCount++

		if m.sqliteStore != nil {
			m.sqliteStore.WritePacket(evt)
		}

		// Fire reverse DNS lookups for IPs we haven't resolved yet.
		if evt.SrcIP != nil {
			if cmd := m.resolver.Resolve(evt.SrcIP.String()); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if evt.DstIP != nil {
			if cmd := m.resolver.Resolve(evt.DstIP.String()); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

		w, cmd := m.inspector.Update(evt)
		m.inspector = w.(*widgets.PacketInspector)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

		m.connections.Update(evt)
		m.bandwidth.Update(evt)
		m.protoDist.Update(evt)
		m.remoteHosts.Update(evt)
		m.netGraph.Update(evt)

	case DNSMsg:
		evt := events.DNSEvent(msg)
		if m.sqliteStore != nil {
			m.sqliteStore.WriteDNS(evt)
		}
		m.dns.Update(evt)
		if evt.ResolvedIP != nil {
			ip := evt.ResolvedIP.String()
			// Seed resolver so we don't fire a redundant PTR lookup for this IP.
			m.resolver.Seed(ip, evt.QueryName)
			m.displayFilters.AddDNSMapping(evt.QueryName, evt.ResolvedIP)
			m.remoteHosts.AddIPName(ip, evt.QueryName)
			m.netGraph.AddIPName(ip, evt.QueryName)
		}
		cmds = append(cmds, m.listenDNS())

	case resolve.Msg:
		if msg.Name != "" {
			m.remoteHosts.AddIPName(msg.IP, msg.Name)
			m.netGraph.AddIPName(msg.IP, msg.Name)
		}

	case TLSMsg:
		evt := events.TLSEvent(msg)
		if m.sqliteStore != nil {
			m.sqliteStore.WriteTLS(evt)
		}
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

// routeKey sends a keyboard or mouse message to the currently focused widget.
func (m *Model) routeKey(msg tea.Msg) tea.Cmd {
	switch m.focusTarget {
	case FocusCentre:
		if m.centreMode == 1 {
			var cmd tea.Cmd
			var w widgets.Widget
			w, cmd = m.netGraph.Update(msg)
			m.netGraph = w.(*widgets.NetGraphWidget)
			return cmd
		}
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
	m.filterBar.SetWidth(m.width)
	statusBar := m.renderStatusBar()

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

	// Left panel: boar (top) + connections (remainder)
	boarH := m.boar.Height()
	m.boar.SetWidth(leftW)
	m.connections.SetSize(leftW, mainH-boarH)
	leftView := lipgloss.JoinVertical(lipgloss.Left,
		m.boar.View(),
		m.connections.View(),
	)

	// Right panel: DNS (top half) + Bandwidth (bottom half)
	rightTopH := mainH / 2
	rightBotH := mainH - rightTopH
	m.dns.SetSize(rightW, rightTopH)
	m.bandwidth.SetSize(rightW, rightBotH)
	rightView := lipgloss.JoinVertical(lipgloss.Left,
		m.dns.View(),
		m.bandwidth.View(),
	)

	// Centre panel — tab bar (1 row) + widget (mainH-1 rows)
	tabBar := m.renderCentreTabBar(centreW)
	var centreContent string
	if m.centreMode == 1 {
		m.netGraph.SetSize(centreW, mainH-1)
		centreContent = m.netGraph.View()
	} else {
		m.inspector.SetSize(centreW, mainH-1)
		centreContent = m.inspector.View()
	}
	centreView := lipgloss.JoinVertical(lipgloss.Left, tabBar, centreContent)

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
	if m.searchBar.Active() {
		m.searchBar.SetWidth(m.width)
		displayBar = m.displayBarStyle.Width(m.width).Render(m.searchBar.View())
	}
	return lipgloss.JoinVertical(lipgloss.Left, displayBar, topRow, bottomRow, statusBar)
}

func (m *Model) updateFocus() {
	isCentre := m.focusTarget == FocusCentre
	m.inspector.SetFocused(isCentre && m.centreMode == 0)
	m.netGraph.SetFocused(isCentre && m.centreMode == 1)
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

// savePcapCmd snapshots all current packet rows and writes them to a pcap file
// in a background goroutine so the UI remains responsive.
func (m *Model) savePcapCmd() tea.Cmd {
	rows := m.inspector.List().AllRows()
	// Copy event slice synchronously on the UI goroutine to avoid data races.
	pkts := make([]events.PacketEvent, len(rows))
	for i, r := range rows {
		pkts[i] = r.Event
	}
	linkType := m.linkType
	saveDir := m.saveDir
	return func() tea.Msg {
		path, count, err := session.SavePcap(saveDir, linkType, pkts)
		if err != nil {
			return SaveStatusMsg{Text: fmt.Sprintf("Save failed: %v", err)}
		}
		return SaveStatusMsg{Text: fmt.Sprintf("Saved %d pkts → %s", count, path)}
	}
}

// renderCentreTabBar renders a 1-row tab strip above the centre widget.
func (m Model) renderCentreTabBar(width int) string {
	type tab struct {
		label string
		mode  int
	}
	tabs := []tab{
		{"Packet Inspector", 0},
		{"Network Diagram", 1},
	}

	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("226")).
		Padding(0, 1)
	inactiveStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Padding(0, 1)
	barStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("234")).
		Width(width)

	var parts []string
	for _, t := range tabs {
		if t.mode == m.centreMode {
			parts = append(parts, activeStyle.Render(t.label))
		} else {
			parts = append(parts, inactiveStyle.Render(t.label))
		}
	}

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("238")).
		Background(lipgloss.Color("234"))
	hint := hintStyle.Render("  n: cycle")

	return barStyle.Render(" " + strings.Join(parts, " ") + hint)
}

// renderStatusBar builds the bottom status bar. When the BPF filter bar is
// active it shows the filter input instead. Key bindings are rendered with a
// bright key label and a muted description so they are easy to scan.
func (m Model) renderStatusBar() string {
	const bg = "236" // must match statusStyle background

	if m.filterBar.Active() {
		return m.statusStyle.Width(m.width).Render(m.filterBar.View())
	}

	pane := m.inspector.PaneName()
	if m.centreMode == 1 {
		pane = m.netGraph.PaneName()
	}

	spaceDesc := "pause"
	if !m.capturing && m.hasBackend {
		spaceDesc = "resume"
	}

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).Bold(true).
		Background(lipgloss.Color(bg))
	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("246")).
		Background(lipgloss.Color(bg))
	sepStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("238")).
		Background(lipgloss.Color(bg))
	metaStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Background(lipgloss.Color(bg))
	nameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).Bold(true).
		Background(lipgloss.Color(bg))

	hint := func(key, desc string) string {
		return keyStyle.Render(key) + descStyle.Render(":"+desc)
	}
	gap := sepStyle.Render("  ")
	pipe := sepStyle.Render("  │  ")

	hints := strings.Join([]string{
		hint("1-4", "focus"),
		hint("tab", "sub"),
		hint("n", "view"),
		hint("/", "bpf"),
		hint("?", "search"),
		hint("Enter", "filter"),
		hint("D", "clear"),
		hint("S", "save"),
		hint("spc", spaceDesc),
		hint("q", "quit"),
	}, gap)

	bar := nameStyle.Render(" "+AppName) +
		pipe + metaStyle.Render(fmt.Sprintf("pkts: %d", m.packetCount)) +
		pipe + metaStyle.Render("["+pane+"]") +
		pipe + metaStyle.Render(focusName(m.focusTarget, m.bottomFocus)) +
		pipe + hints

	if fv := m.filterBar.View(); fv != "" {
		bar += pipe + metaStyle.Render(fv)
	}
	if m.saveStatus != "" {
		bar += pipe + metaStyle.Render(m.saveStatus)
	}

	return m.statusStyle.Width(m.width).Render(bar)
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
