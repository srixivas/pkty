package widgets

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const bytesPerLine = 16

type HexDump struct {
	data           []byte
	highlightStart int
	highlightEnd   int
	offset         int
	width          int
	height         int
	focused        bool

	normalStyle    lipgloss.Style
	highlightStyle lipgloss.Style
	asciiStyle     lipgloss.Style
	offsetStyle    lipgloss.Style
	borderStyle    lipgloss.Style
}

func NewHexDump() *HexDump {
	return &HexDump{
		highlightStart: -1,
		highlightEnd:   -1,
		normalStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		highlightStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("53")),
		asciiStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("114")),
		offsetStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("67")),
		borderStyle:    lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")),
	}
}

func (h *HexDump) Name() string      { return "Hex Dump" }
func (h *HexDump) Init() tea.Cmd     { return nil }
func (h *HexDump) SetFocused(f bool) { h.focused = f }
func (h *HexDump) Focused() bool     { return h.focused }
func (h *HexDump) SetSize(w, ht int) { h.width = w; h.height = ht }

func (h *HexDump) SetData(data []byte)         { h.data = data; h.offset = 0 }
func (h *HexDump) SetHighlight(start, end int) {
	h.highlightStart = start
	h.highlightEnd = end
	if start >= 0 {
		target := start / bytesPerLine
		vis := h.visibleLines()
		if target < h.offset || target >= h.offset+vis {
			h.offset = target - vis/2
			if h.offset < 0 {
				h.offset = 0
			}
		}
	}
}

func (h *HexDump) Update(msg tea.Msg) (Widget, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok && h.focused {
		maxOff := h.totalLines() - h.visibleLines()
		if maxOff < 0 {
			maxOff = 0
		}
		switch msg.String() {
		case "up", "k":
			if h.offset > 0 {
				h.offset--
			}
		case "down", "j":
			if h.offset < maxOff {
				h.offset++
			}
		case "pgup":
			h.offset -= h.visibleLines()
			if h.offset < 0 {
				h.offset = 0
			}
		case "pgdown":
			h.offset += h.visibleLines()
			if h.offset > maxOff {
				h.offset = maxOff
			}
		}
	}
	return h, nil
}

func (h *HexDump) View() string {
	cw := h.width - 2
	ch := h.height - 2
	if cw < 10 || ch < 1 {
		return h.borderStyle.Width(cw).Render("...")
	}

	if len(h.data) == 0 {
		pad := make([]string, ch)
		for i := range pad {
			pad[i] = strings.Repeat(" ", cw)
		}
		pad[ch/2] = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Width(cw).Render("No packet data")
		return h.borderStyle.Width(cw).Render(strings.Join(pad, "\n"))
	}

	total := h.totalLines()
	var lines []string
	for li := h.offset; li < h.offset+ch && li < total; li++ {
		bs := li * bytesPerLine
		be := bs + bytesPerLine
		if be > len(h.data) {
			be = len(h.data)
		}

		off := h.offsetStyle.Render(fmt.Sprintf("%04x  ", bs))

		var hex []string
		for i := bs; i < bs+bytesPerLine; i++ {
			if i < be {
				s := fmt.Sprintf("%02x", h.data[i])
				if h.isHL(i) {
					hex = append(hex, h.highlightStyle.Render(s))
				} else {
					hex = append(hex, h.normalStyle.Render(s))
				}
			} else {
				hex = append(hex, "  ")
			}
			if (i-bs) == 7 {
				hex = append(hex, " ")
			}
		}

		var ascii []string
		for i := bs; i < bs+bytesPerLine; i++ {
			if i < be {
				c := "."
				if h.data[i] >= 32 && h.data[i] < 127 {
					c = string(h.data[i])
				}
				if h.isHL(i) {
					ascii = append(ascii, h.highlightStyle.Render(c))
				} else {
					ascii = append(ascii, h.asciiStyle.Render(c))
				}
			} else {
				ascii = append(ascii, " ")
			}
		}

		lines = append(lines, off+strings.Join(hex, " ")+"  "+strings.Join(ascii, ""))
	}

	for len(lines) < ch {
		lines = append(lines, strings.Repeat(" ", cw))
	}

	return h.borderStyle.Width(cw).Render(strings.Join(lines, "\n"))
}

func (h *HexDump) isHL(i int) bool {
	return h.highlightStart >= 0 && i >= h.highlightStart && i < h.highlightEnd
}
func (h *HexDump) totalLines() int {
	n := len(h.data) / bytesPerLine
	if len(h.data)%bytesPerLine != 0 {
		n++
	}
	return n
}
func (h *HexDump) visibleLines() int {
	v := h.height - 3
	if v < 1 {
		return 1
	}
	return v
}
