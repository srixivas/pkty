package widgets

// TODO: Annotate edges with HTTP content-type from HTTPEvent when the
// connection is identified (e.g. "video/mp4" → streaming, "application/json"
// → API call). HTTPEvent already carries ContentType and Method/URL; wiring it
// here would allow edge labels like [HTTPS · video/mp4].

import (
	"fmt"
	"net"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/srixivas/pkty/internal/events"
)

// privateBlocks is initialised once at startup.
var privateBlocks []*net.IPNet

func init() {
	for _, cidr := range []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"::1/128",
		"fe80::/10",
		"fc00::/7",
	} {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			privateBlocks = append(privateBlocks, block)
		}
	}
}

func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, b := range privateBlocks {
		if b.Contains(ip) {
			return true
		}
	}
	return false
}

// protoLabel returns a human-readable name for well-known port numbers.
func protoLabel(proto string, port uint16) string {
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
	if proto != "" && port > 0 {
		return fmt.Sprintf("%s:%d", proto, port)
	}
	if port > 0 {
		return fmt.Sprintf("???:%d", port)
	}
	return "???"
}

// --- data model -------------------------------------------------------------

type ngEdgeKey struct {
	Proto string
	Port  uint16
}

type ngEdge struct {
	Proto   string
	Port    uint16
	TXBytes uint64 // outgoing: local → remote
	RXBytes uint64 // incoming: remote → local
}

type ngHost struct {
	IP      string
	Edges   map[ngEdgeKey]*ngEdge
	TXTotal uint64
	RXTotal uint64
	Tag     string // "local" or "remote"
}

// hostRows returns the number of display rows a host group occupies:
// 1 header + N edges + 1 separator line.
func hostRows(h *ngHost) int { return 1 + len(h.Edges) + 1 }

// --- widget -----------------------------------------------------------------

type NetGraphWidget struct {
	hosts        map[string]*ngHost
	sorted       []*ngHost
	visible      []*ngHost          // filtered subset of sorted; == sorted when no filter active
	displayFilter *DisplayFilterSet
	ipToName     map[string]string
	maxEdgeBytes uint64 // global max of any single edge TX or RX — used for bar scaling
	sourceName   string
	cursor       int // host index
	offset       int // first visible pre-rendered row index
	focused      bool
	width        int
	height       int

	sourceStyle    lipgloss.Style
	hostNormStyle  lipgloss.Style
	hostSelStyle   lipgloss.Style
	localTagStyle  lipgloss.Style
	remoteTagStyle lipgloss.Style
	treeStyle      lipgloss.Style
	txStyle        lipgloss.Style
	rxStyle        lipgloss.Style
	labelStyle     lipgloss.Style
	totalStyle     lipgloss.Style
	hintStyle      lipgloss.Style
	emptyStyle     lipgloss.Style
	borderStyle    lipgloss.Style
	focusBorder    lipgloss.Style
}

func NewNetGraphWidget() *NetGraphWidget {
	return &NetGraphWidget{
		hosts:          make(map[string]*ngHost),
		ipToName:       make(map[string]string),
		sourceName:     "source",
		sourceStyle:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("226")),
		hostNormStyle:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252")),
		hostSelStyle:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("25")),
		localTagStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		remoteTagStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("33")),
		treeStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		txStyle:        lipgloss.NewStyle().Foreground(lipgloss.Color("10")),  // green  ──▶
		rxStyle:        lipgloss.NewStyle().Foreground(lipgloss.Color("39")),  // cyan   ◀──
		labelStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		totalStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		hintStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		emptyStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true),
		borderStyle:    lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("33")),
		focusBorder:    lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("226")),
	}
}

func (g *NetGraphWidget) Name() string          { return "Network Diagram" }
func (g *NetGraphWidget) PaneName() string      { return "Network Diagram" }
func (g *NetGraphWidget) Init() tea.Cmd         { return nil }
func (g *NetGraphWidget) SetSize(w, h int)      { g.width = w; g.height = h }
func (g *NetGraphWidget) SetFocused(f bool)     { g.focused = f }
func (g *NetGraphWidget) Focused() bool         { return g.focused }
func (g *NetGraphWidget) SetSourceName(n string) {
	if n != "" {
		g.sourceName = n
	}
}

func (g *NetGraphWidget) SetDisplayFilter(ds *DisplayFilterSet) {
	g.displayFilter = ds
}

// RebuildVisible recomputes g.visible from g.sorted applying the active display filter.
// Called after rebuildSorted() and after filter changes from layout.
func (g *NetGraphWidget) RebuildVisible() {
	if g.displayFilter == nil || !g.displayFilter.Active() {
		g.visible = g.sorted
	} else {
		vis := make([]*ngHost, 0, len(g.sorted))
		for _, h := range g.sorted {
			var ports []uint16
			var protos []string
			for ek := range h.Edges {
				ports = append(ports, ek.Port)
				protos = append(protos, ek.Proto)
			}
			if g.displayFilter.MatchHost(h.IP, ports, protos) {
				vis = append(vis, h)
			}
		}
		g.visible = vis
	}
	if g.cursor >= len(g.visible) {
		g.cursor = len(g.visible) - 1
	}
	if g.cursor < 0 {
		g.cursor = 0
	}
}

func (g *NetGraphWidget) AddIPName(ip, name string) {
	if ip != "" && name != "" {
		g.ipToName[ip] = name
	}
}

func (g *NetGraphWidget) displayName(ip string) string {
	if name, ok := g.ipToName[ip]; ok {
		return name
	}
	return ip
}

func (g *NetGraphWidget) SelectedFilter() *DisplayFilter {
	if len(g.visible) == 0 || g.cursor < 0 || g.cursor >= len(g.visible) {
		return nil
	}
	ip := g.visible[g.cursor].IP
	if name, ok := g.ipToName[ip]; ok {
		f := MakeFilter(FilterDNS, name)
		return &f
	}
	f := MakeFilter(FilterIP, ip)
	return &f
}

func (g *NetGraphWidget) Update(msg tea.Msg) (Widget, tea.Cmd) {
	switch msg := msg.(type) {
	case events.PacketEvent:
		if msg.SrcIP == nil || msg.DstIP == nil {
			return g, nil
		}

		srcPriv := isPrivateIP(msg.SrcIP)
		dstPriv := isPrivateIP(msg.DstIP)

		// Determine the remote host and traffic direction.
		var remoteIP net.IP
		var isTX bool    // true = outgoing (local→remote)
		var svcPort uint16

		switch {
		case srcPriv && !dstPriv:
			// Outgoing: local → internet
			remoteIP, isTX, svcPort = msg.DstIP, true, msg.DstPort
		case !srcPriv && dstPriv:
			// Incoming: internet → local
			remoteIP, isTX, svcPort = msg.SrcIP, false, msg.SrcPort
		case srcPriv && dstPriv:
			// LAN (local → local)
			remoteIP, isTX, svcPort = msg.DstIP, true, msg.DstPort
		default:
			// Both public (relay/passthrough)
			remoteIP, isTX, svcPort = msg.DstIP, true, msg.DstPort
		}

		ip := remoteIP.String()
		host, ok := g.hosts[ip]
		if !ok {
			tag := "remote"
			if isPrivateIP(remoteIP) {
				tag = "local"
			}
			host = &ngHost{IP: ip, Edges: make(map[ngEdgeKey]*ngEdge), Tag: tag}
			g.hosts[ip] = host
		}

		ek := ngEdgeKey{Proto: msg.Protocol, Port: svcPort}
		edge, ok := host.Edges[ek]
		if !ok {
			edge = &ngEdge{Proto: msg.Protocol, Port: svcPort}
			host.Edges[ek] = edge
		}

		bytes := uint64(msg.Length)
		if isTX {
			edge.TXBytes += bytes
			host.TXTotal += bytes
		} else {
			edge.RXBytes += bytes
			host.RXTotal += bytes
		}
		if edge.TXBytes > g.maxEdgeBytes {
			g.maxEdgeBytes = edge.TXBytes
		}
		if edge.RXBytes > g.maxEdgeBytes {
			g.maxEdgeBytes = edge.RXBytes
		}

		g.rebuildSorted()

	case tea.MouseMsg:
		if !g.focused {
			return g, nil
		}
		switch msg.Type {
		case tea.MouseWheelUp:
			if g.cursor > 0 {
				g.cursor--
				g.ensureVisible()
			}
		case tea.MouseWheelDown:
			if g.cursor < len(g.visible)-1 {
				g.cursor++
				g.ensureVisible()
			}
		}

	case tea.KeyMsg:
		if !g.focused {
			return g, nil
		}
		switch msg.String() {
		case "up", "k":
			if g.cursor > 0 {
				g.cursor--
				g.ensureVisible()
			}
		case "down", "j":
			if g.cursor < len(g.visible)-1 {
				g.cursor++
				g.ensureVisible()
			}
		case "enter":
			if f := g.SelectedFilter(); f != nil {
				return g, func() tea.Msg { return DisplayFilterToggleMsg{Filter: *f} }
			}
		}
	}
	return g, nil
}

func (g *NetGraphWidget) rebuildSorted() {
	g.sorted = make([]*ngHost, 0, len(g.hosts))
	for _, h := range g.hosts {
		g.sorted = append(g.sorted, h)
	}
	sort.Slice(g.sorted, func(i, j int) bool {
		ti := g.sorted[i].TXTotal + g.sorted[i].RXTotal
		tj := g.sorted[j].TXTotal + g.sorted[j].RXTotal
		return ti > tj
	})
	g.RebuildVisible()
}

// hostFirstRowVisible counts pre-rendered rows for hosts in g.visible up to hostIdx.
// Rows 0 and 1 are always the source node and its tree connector.
func (g *NetGraphWidget) hostFirstRowVisible(hostIdx int) int {
	row := 2
	for i := 0; i < hostIdx; i++ {
		row += hostRows(g.visible[i])
	}
	return row
}

func (g *NetGraphWidget) ensureVisible() {
	// avail = inner height − source(1) − connector(1) − hint(1)
	avail := g.height - 2 - 3
	if avail < 1 {
		avail = 1
	}
	if len(g.visible) == 0 {
		return
	}
	first := g.hostFirstRowVisible(g.cursor)
	last := first + hostRows(g.visible[g.cursor]) - 1

	if first < g.offset {
		g.offset = first
	}
	if last >= g.offset+avail {
		g.offset = last - avail + 1
	}
	if g.offset < 0 {
		g.offset = 0
	}
}

// ngBarFill computes how many bar cells to fill for a given value/max/width.
func ngBarFill(val, max uint64, width int) int {
	if max == 0 || width <= 0 {
		return 0
	}
	n := int(float64(val) / float64(max) * float64(width))
	if n < 0 {
		return 0
	}
	if n > width {
		return width
	}
	return n
}

func (g *NetGraphWidget) View() string {
	cw := g.width - 2  // inner width  (border eats 2)
	ch := g.height - 2 // inner height (border eats 2)
	if cw < 20 || ch < 4 {
		bs := g.borderStyle
		if g.focused {
			bs = g.focusBorder
		}
		return bs.Width(cw).Render("Network Diagram")
	}

	border := g.borderStyle
	if g.focused {
		border = g.focusBorder
	}

	// Bar width for edge rows.
	// Fixed visible chars per edge row (approximately):
	//   "  │  └──[LABEL  ]──▶  1.2 MB  ◀──  800 KB " = ~46 chars
	// Split the remainder equally between TX and RX bars.
	const edgeFixed = 46
	barW := (cw - edgeFixed) / 2
	if barW < 3 {
		barW = 3
	}

	// ── Pre-render all rows ──────────────────────────────────────────────────
	var rows []string

	// Row 0: source node
	rows = append(rows, g.sourceStyle.Render(fmt.Sprintf("  [%s · source]", g.sourceName)))

	if len(g.sorted) == 0 {
		rows = append(rows, g.treeStyle.Render("  │"))
		rows = append(rows, g.emptyStyle.Render("  └── (waiting for traffic...)"))
	} else {
		rows = append(rows, g.treeStyle.Render("  │"))

		// Filter indicator: show when some hosts are hidden by an active filter.
		if g.displayFilter != nil && g.displayFilter.Active() && len(g.visible) < len(g.sorted) {
			hidden := len(g.sorted) - len(g.visible)
			rows = append(rows, g.treeStyle.Render(
				fmt.Sprintf("  │  (%d of %d hosts hidden by filter)", hidden, len(g.sorted)),
			))
		}

		if len(g.visible) == 0 {
			rows = append(rows, g.emptyStyle.Render("  └── (no hosts match filter)"))
		} else {
			for i, h := range g.visible {
				isLastHost := i == len(g.visible)-1
				isCursor := i == g.cursor

				hostConn := "  ├─▶"
				vertLine := "  │"
				if isLastHost {
					hostConn = "  └─▶"
					vertLine = "   "
				}

				// Host header ────────────────────────────────────────────────────
				name := g.displayName(h.IP)
				if name != h.IP {
					// Show "domain.com (1.2.3.4)"
					name = fmt.Sprintf("%s (%s)", truncStr(name, 28), h.IP)
				}
				name = truncStr(name, 40)

				total := h.TXTotal + h.RXTotal
				tagTxt := "[remote]"
				tagSty := g.remoteTagStyle
				if h.Tag == "local" {
					tagTxt = "[local]"
					tagSty = g.localTagStyle
				}

				if isCursor && g.focused {
					raw := fmt.Sprintf("%s %s %s  %s",
						hostConn, name, tagTxt, formatBytes(total))
					rows = append(rows, g.hostSelStyle.Width(cw).Render(truncStr(raw, cw)))
				} else {
					rows = append(rows,
						g.treeStyle.Render(hostConn)+" "+
							g.hostNormStyle.Render(name)+" "+
							tagSty.Render(tagTxt)+"  "+
							g.totalStyle.Render(formatBytes(total)),
					)
				}

				// Edge rows ──────────────────────────────────────────────────────
				sortedEdges := make([]*ngEdge, 0, len(h.Edges))
				for _, e := range h.Edges {
					sortedEdges = append(sortedEdges, e)
				}
				sort.Slice(sortedEdges, func(a, b int) bool {
					ta := sortedEdges[a].TXBytes + sortedEdges[a].RXBytes
					tb := sortedEdges[b].TXBytes + sortedEdges[b].RXBytes
					return ta > tb
				})

				for ei, e := range sortedEdges {
					edgeConn := "  ├──"
					if ei == len(sortedEdges)-1 {
						edgeConn = "  └──"
					}

					label := fmt.Sprintf("%-7s", truncStr(protoLabel(e.Proto, e.Port), 7))

					txFill := ngBarFill(e.TXBytes, g.maxEdgeBytes, barW)
					rxFill := ngBarFill(e.RXBytes, g.maxEdgeBytes, barW)
					txBar := g.txStyle.Render(strings.Repeat("█", txFill) + strings.Repeat("░", barW-txFill))
					rxBar := g.rxStyle.Render(strings.Repeat("█", rxFill) + strings.Repeat("░", barW-rxFill))

					row := g.treeStyle.Render(vertLine+edgeConn) +
						"[" + g.labelStyle.Render(label) + "]" +
						g.txStyle.Render("──▶") + " " +
						g.txStyle.Render(fmt.Sprintf("%7s", formatBytes(e.TXBytes))) + " " +
						txBar + "  " +
						g.rxStyle.Render("◀──") + " " +
						g.rxStyle.Render(fmt.Sprintf("%7s", formatBytes(e.RXBytes))) + " " +
						rxBar

					rows = append(rows, row)
				}

				// Separator between host groups
				if !isLastHost {
					rows = append(rows, g.treeStyle.Render(vertLine))
				}
			}
		}
	}

	// ── Windowed rendering ───────────────────────────────────────────────────
	avail := ch - 1 // last row reserved for hint
	if avail < 1 {
		avail = 1
	}

	start := g.offset
	if start >= len(rows) {
		start = len(rows)
	}
	end := start + avail
	if end > len(rows) {
		end = len(rows)
	}

	var lines []string
	lines = append(lines, rows[start:end]...)

	// Pad to fill the viewport
	for len(lines) < avail {
		lines = append(lines, "")
	}

	// Hint (always pinned to last row)
	hint := g.hintStyle.Render(" j/k: scroll   Enter: filter   n: toggle view")
	lines = append(lines, hint)

	return border.Width(cw).Render(strings.Join(lines, "\n"))
}
