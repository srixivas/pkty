package widgets

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/srixivas/pkty/internal/events"
)

type TreeNode struct {
	Label      string
	IsLayer    bool
	Expanded   bool
	LayerIndex int
	FieldIndex int
	Start      int
	End        int
}

type DetailTree struct {
	nodes   []TreeNode
	cursor  int
	offset  int
	width   int
	height  int
	focused bool
	packet  *events.PacketEvent

	OnSelect func(start, end int)

	layerStyle    lipgloss.Style
	fieldStyle    lipgloss.Style
	selectedStyle lipgloss.Style
	borderStyle   lipgloss.Style
}

func NewDetailTree() *DetailTree {
	return &DetailTree{
		layerStyle:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")),
		fieldStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		selectedStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("25")),
		borderStyle:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")),
	}
}

func (d *DetailTree) Name() string      { return "Packet Detail" }
func (d *DetailTree) Init() tea.Cmd     { return nil }
func (d *DetailTree) SetFocused(f bool) { d.focused = f }
func (d *DetailTree) Focused() bool     { return d.focused }
func (d *DetailTree) SetSize(w, h int)  { d.width = w; d.height = h }

func (d *DetailTree) SetPacket(evt *events.PacketEvent) {
	d.packet = evt
	d.nodes = nil
	d.cursor = 0
	d.offset = 0
	if evt == nil {
		return
	}

	d.nodes = append(d.nodes, TreeNode{
		Label: fmt.Sprintf("Frame %d: %d bytes on wire", evt.Number, evt.Length),
		IsLayer: true, Expanded: false, Start: 0, End: len(evt.RawBytes),
	})

	for i, layer := range evt.LayerData {
		d.nodes = append(d.nodes, TreeNode{
			Label: layer.Name, IsLayer: true, Expanded: true,
			LayerIndex: i, Start: layer.Start, End: layer.End,
		})
		for j, field := range layer.Fields {
			d.nodes = append(d.nodes, TreeNode{
				Label: fmt.Sprintf("%s: %s", field.Key, field.Value),
				LayerIndex: i, FieldIndex: j, Start: layer.Start, End: layer.End,
			})
		}
	}
}

func (d *DetailTree) SelectedRange() (int, int) {
	if d.cursor >= 0 && d.cursor < len(d.nodes) {
		return d.nodes[d.cursor].Start, d.nodes[d.cursor].End
	}
	return -1, -1
}

func (d *DetailTree) Update(msg tea.Msg) (Widget, tea.Cmd) {
	if msg, ok := msg.(tea.MouseMsg); ok && d.focused {
		switch msg.Type {
		case tea.MouseWheelUp:
			d.moveCursor(-1)
		case tea.MouseWheelDown:
			d.moveCursor(1)
		}
		return d, nil
	}
	if msg, ok := msg.(tea.KeyMsg); ok && d.focused {
		switch msg.String() {
		case "up", "k":
			d.moveCursor(-1)
		case "down", "j":
			d.moveCursor(1)
		case "enter", " ":
			d.toggleExpand()
		case "pgup":
			d.moveCursor(-d.visibleRows())
		case "pgdown":
			d.moveCursor(d.visibleRows())
		}
	}
	return d, nil
}

func (d *DetailTree) moveCursor(delta int) {
	vis := d.getVisibleNodes()
	d.cursor += delta
	if d.cursor < 0 {
		d.cursor = 0
	}
	if d.cursor >= len(vis) {
		d.cursor = len(vis) - 1
	}
	if d.cursor < 0 {
		d.cursor = 0
	}
	d.ensureVisible()
	d.notifySelect()
}

func (d *DetailTree) toggleExpand() {
	vis := d.getVisibleNodes()
	if d.cursor < 0 || d.cursor >= len(vis) {
		return
	}
	idx := vis[d.cursor]
	if d.nodes[idx].IsLayer {
		d.nodes[idx].Expanded = !d.nodes[idx].Expanded
	}
}

func (d *DetailTree) View() string {
	cw := d.width - 2
	ch := d.height - 2
	if cw < 5 || ch < 1 {
		return d.borderStyle.Width(cw).Render("...")
	}

	if d.packet == nil {
		content := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Width(cw).Render("Select a packet to view details")
		pad := make([]string, ch)
		pad[ch/2] = content
		for i := range pad {
			if pad[i] == "" {
				pad[i] = strings.Repeat(" ", cw)
			}
		}
		return d.borderStyle.Width(cw).Render(strings.Join(pad, "\n"))
	}

	vis := d.getVisibleNodes()
	var lines []string
	for vi := d.offset; vi < d.offset+ch && vi < len(vis); vi++ {
		node := d.nodes[vis[vi]]
		var line string
		if node.IsLayer {
			p := "▶ "
			if node.Expanded {
				p = "▼ "
			}
			line = p + node.Label
		} else {
			line = "    " + node.Label
		}
		line = truncStr(line, cw)

		if vi == d.cursor && d.focused {
			lines = append(lines, d.selectedStyle.Width(cw).Render(line))
		} else if node.IsLayer {
			lines = append(lines, d.layerStyle.Width(cw).Render(line))
		} else {
			lines = append(lines, d.fieldStyle.Width(cw).Render(line))
		}
	}
	for len(lines) < ch {
		lines = append(lines, strings.Repeat(" ", cw))
	}
	return d.borderStyle.Width(cw).Render(strings.Join(lines, "\n"))
}

func (d *DetailTree) getVisibleNodes() []int {
	var vis []int
	expanded := false
	for i, n := range d.nodes {
		if n.IsLayer {
			vis = append(vis, i)
			expanded = n.Expanded
		} else if expanded {
			vis = append(vis, i)
		}
	}
	return vis
}

func (d *DetailTree) visibleRows() int {
	v := d.height - 3
	if v < 1 {
		return 1
	}
	return v
}

func (d *DetailTree) ensureVisible() {
	vis := d.visibleRows()
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

func (d *DetailTree) notifySelect() {
	if d.OnSelect == nil {
		return
	}
	vis := d.getVisibleNodes()
	if d.cursor >= 0 && d.cursor < len(vis) {
		n := d.nodes[vis[d.cursor]]
		d.OnSelect(n.Start, n.End)
	}
}
