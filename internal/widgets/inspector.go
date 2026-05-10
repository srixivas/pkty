package widgets

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/srixivas/pkty/internal/events"
)

// Pane identifiers for focus cycling within the inspector.
const (
	PaneList   = 0
	PaneDetail = 1
	PaneHex    = 2
)

// PacketInspector is the centre panel composite widget containing
// the packet list, detail tree, and hex dump stacked vertically.
type PacketInspector struct {
	list      *PacketList
	detail    *DetailTree
	hexdump   *HexDump
	activePane int
	width     int
	height    int
	focused   bool

	titleStyle lipgloss.Style
}

// NewPacketInspector creates the composite centre panel.
func NewPacketInspector() *PacketInspector {
	pi := &PacketInspector{
		list:       NewPacketList(),
		detail:     NewDetailTree(),
		hexdump:    NewHexDump(),
		activePane: PaneList,
		focused:    true,
		titleStyle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")),
	}

	// Wire callbacks: selecting a packet updates detail + hex
	pi.list.OnSelect = func(row PacketRow) {
		pi.detail.SetPacket(&row.Event)
		pi.hexdump.SetData(row.Event.RawBytes)
		pi.hexdump.SetHighlight(-1, -1)
	}

	// Wire callbacks: selecting a tree node highlights hex bytes
	pi.detail.OnSelect = func(start, end int) {
		pi.hexdump.SetHighlight(start, end)
	}

	pi.updateFocus()
	return pi
}

func (pi *PacketInspector) Name() string { return "Packet Inspector" }
func (pi *PacketInspector) Init() tea.Cmd { return nil }

func (pi *PacketInspector) SetFocused(f bool) {
	pi.focused = f
	pi.updateFocus()
}

func (pi *PacketInspector) Update(msg tea.Msg) (Widget, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !pi.focused {
			return pi, nil
		}
		switch msg.String() {
		case "tab":
			pi.activePane = (pi.activePane + 1) % 3
			pi.updateFocus()
			return pi, nil
		case "shift+tab":
			pi.activePane = (pi.activePane + 2) % 3
			pi.updateFocus()
			return pi, nil
		}

		// Forward key to active pane
		switch pi.activePane {
		case PaneList:
			var cmd tea.Cmd
			w, cmd := pi.list.Update(msg)
			pi.list = w.(*PacketList)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case PaneDetail:
			var cmd tea.Cmd
			w, cmd := pi.detail.Update(msg)
			pi.detail = w.(*DetailTree)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case PaneHex:
			var cmd tea.Cmd
			w, cmd := pi.hexdump.Update(msg)
			pi.hexdump = w.(*HexDump)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	case tea.MouseMsg:
		if !pi.focused {
			return pi, nil
		}
		switch pi.activePane {
		case PaneList:
			w, cmd := pi.list.Update(msg)
			pi.list = w.(*PacketList)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case PaneDetail:
			w, cmd := pi.detail.Update(msg)
			pi.detail = w.(*DetailTree)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case PaneHex:
			w, cmd := pi.hexdump.Update(msg)
			pi.hexdump = w.(*HexDump)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	default:
		// PacketMsg events go to the list
		if _, ok := msg.(events.PacketEvent); ok {
			w, cmd := pi.list.Update(msg)
			pi.list = w.(*PacketList)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	return pi, tea.Batch(cmds...)
}

func (pi *PacketInspector) View() string {
	if pi.width < 10 || pi.height < 10 {
		return "..."
	}

	// Allocate space: list gets 45%, detail 30%, hex 25%
	listH := pi.height * 45 / 100
	detailH := pi.height * 30 / 100
	hexH := pi.height - listH - detailH

	if listH < 5 {
		listH = 5
	}
	if detailH < 4 {
		detailH = 4
	}
	if hexH < 4 {
		hexH = 4
	}

	pi.list.SetSize(pi.width, listH)
	pi.detail.SetSize(pi.width, detailH)
	pi.hexdump.SetSize(pi.width, hexH)

	parts := []string{
		pi.list.View(),
		pi.detail.View(),
		pi.hexdump.View(),
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (pi *PacketInspector) SetSize(w, h int) {
	pi.width = w
	pi.height = h
}

func (pi *PacketInspector) updateFocus() {
	pi.list.SetFocused(pi.focused && pi.activePane == PaneList)
	pi.detail.SetFocused(pi.focused && pi.activePane == PaneDetail)
	pi.hexdump.SetFocused(pi.focused && pi.activePane == PaneHex)
}

// List returns the underlying PacketList so callers can set display filters.
func (pi *PacketInspector) List() *PacketList {
	return pi.list
}

// PaneName returns the name of the currently active sub-pane.
func (pi *PacketInspector) PaneName() string {
	names := []string{"Packet List", "Packet Detail", "Hex Dump"}
	_ = strings.Join // keep import
	return names[pi.activePane]
}
