package widgets

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/c0d343v3r/netdash/internal/events"
)

type BandwidthWidget struct {
	totalBytes   uint64
	totalPackets uint64
	samples      []bwSample // per-second samples
	lastSecond   int64
	currentBytes uint64
	width        int
	height       int

	titleStyle  lipgloss.Style
	labelStyle  lipgloss.Style
	barStyle    lipgloss.Style
	valueStyle  lipgloss.Style
	borderStyle lipgloss.Style
}

type bwSample struct {
	Bytes   uint64
	Packets uint64
}

func NewBandwidthWidget() *BandwidthWidget {
	return &BandwidthWidget{
		titleStyle:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("13")),
		labelStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		barStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("13")),
		valueStyle:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),
		borderStyle: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("13")),
	}
}

func (b *BandwidthWidget) Name() string     { return "Bandwidth" }
func (b *BandwidthWidget) Init() tea.Cmd     { return nil }
func (b *BandwidthWidget) SetSize(w, h int)  { b.width = w; b.height = h }

func (b *BandwidthWidget) Update(msg tea.Msg) (Widget, tea.Cmd) {
	if evt, ok := msg.(events.PacketEvent); ok {
		b.totalBytes += uint64(evt.Length)
		b.totalPackets++

		sec := evt.Timestamp.Unix()
		if b.lastSecond == 0 {
			b.lastSecond = sec
		}
		if sec > b.lastSecond {
			b.samples = append(b.samples, bwSample{Bytes: b.currentBytes, Packets: 1})
			if len(b.samples) > 60 {
				b.samples = b.samples[len(b.samples)-60:]
			}
			b.currentBytes = 0
			b.lastSecond = sec
		}
		b.currentBytes += uint64(evt.Length)
	}
	return b, nil
}

func (b *BandwidthWidget) View() string {
	cw := b.width - 2
	ch := b.height - 2
	if cw < 5 || ch < 2 {
		return b.borderStyle.Width(cw).Render("BW")
	}

	title := b.titleStyle.Render(" Bandwidth")

	// Current rate
	rate := b.currentBytes
	if len(b.samples) > 0 {
		rate = b.samples[len(b.samples)-1].Bytes
	}
	rateStr := formatBytes(rate) + "/s"

	// Sparkline from samples
	spark := b.sparkline(cw - 2)

	// Total
	elapsed := time.Duration(len(b.samples)) * time.Second
	elStr := "0s"
	if elapsed > 0 {
		elStr = elapsed.String()
	}

	lines := []string{
		title,
		b.labelStyle.Render("Rate: ") + b.valueStyle.Render(rateStr),
		b.labelStyle.Render("Total: ") + b.valueStyle.Render(formatBytes(b.totalBytes)),
		b.labelStyle.Render("Pkts: ") + b.valueStyle.Render(fmt.Sprintf("%d", b.totalPackets)),
		b.labelStyle.Render("Time: ") + b.valueStyle.Render(elStr),
		"",
		b.barStyle.Render(spark),
	}

	for len(lines) < ch {
		lines = append(lines, strings.Repeat(" ", cw))
	}
	if len(lines) > ch {
		lines = lines[:ch]
	}

	return b.borderStyle.Width(cw).Render(strings.Join(lines, "\n"))
}

func (b *BandwidthWidget) sparkline(width int) string {
	if len(b.samples) == 0 || width < 2 {
		return ""
	}
	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	// Use last `width` samples
	start := 0
	if len(b.samples) > width {
		start = len(b.samples) - width
	}
	data := b.samples[start:]

	var maxVal uint64
	for _, s := range data {
		if s.Bytes > maxVal {
			maxVal = s.Bytes
		}
	}
	if maxVal == 0 {
		maxVal = 1
	}

	var sb strings.Builder
	for _, s := range data {
		idx := int(s.Bytes * uint64(len(blocks)-1) / maxVal)
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		sb.WriteRune(blocks[idx])
	}
	return sb.String()
}
