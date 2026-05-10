package widgets

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/srixivas/pkty/internal/events"
)

type tlsRow struct {
	SNI     string
	Version string
	Cipher  string
}

type TLSInspectorWidget struct {
	rows          []tlsRow
	maxRows       int
	cursor        int
	offset        int
	focused       bool
	versionCounts map[string]uint64
	totalHS       uint64
	width         int
	height        int

	titleStyle    lipgloss.Style
	headerStyle   lipgloss.Style
	selectedStyle lipgloss.Style
	tls13Style    lipgloss.Style
	tls12Style    lipgloss.Style
	tls10Style    lipgloss.Style
	normalStyle   lipgloss.Style
	borderStyle   lipgloss.Style
	focusBorder   lipgloss.Style
}

func NewTLSInspectorWidget() *TLSInspectorWidget {
	return &TLSInspectorWidget{
		maxRows:       300,
		versionCounts: make(map[string]uint64),
		titleStyle:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("208")),
		headerStyle:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("236")),
		selectedStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("25")),
		tls13Style:    lipgloss.NewStyle().Foreground(lipgloss.Color("114")),
		tls12Style:    lipgloss.NewStyle().Foreground(lipgloss.Color("222")),
		tls10Style:    lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		normalStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		borderStyle:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("208")),
		focusBorder:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("226")),
	}
}

func (t *TLSInspectorWidget) Name() string     { return "TLS Inspector" }
func (t *TLSInspectorWidget) Init() tea.Cmd     { return nil }
func (t *TLSInspectorWidget) SetSize(w, h int)  { t.width = w; t.height = h }
func (t *TLSInspectorWidget) SetFocused(f bool) { t.focused = f }
func (t *TLSInspectorWidget) Focused() bool     { return t.focused }

func (t *TLSInspectorWidget) SelectedFilter() *DisplayFilter {
	if len(t.rows) == 0 || t.cursor < 0 || t.cursor >= len(t.rows) {
		return nil
	}
	sni := t.rows[t.cursor].SNI
	if sni == "" {
		return nil
	}
	f := MakeFilter(FilterTLSSNI, sni)
	return &f
}

func (t *TLSInspectorWidget) Update(msg tea.Msg) (Widget, tea.Cmd) {
	switch msg := msg.(type) {
	case events.TLSEvent:
		t.rows = append(t.rows, tlsRow{
			SNI: msg.SNI, Version: msg.Version, Cipher: msg.CipherSuite,
		})
		if len(t.rows) > t.maxRows {
			t.rows = t.rows[len(t.rows)-t.maxRows:]
		}
		t.versionCounts[msg.Version]++
		t.totalHS++

		// Auto-scroll only when not focused
		if !t.focused {
			vis := t.height - 6
			if vis < 1 {
				vis = 1
			}
			if len(t.rows) > vis {
				t.offset = len(t.rows) - vis
			}
			t.cursor = len(t.rows) - 1
		}

	case tea.MouseMsg:
		if !t.focused {
			return t, nil
		}
		switch msg.Type {
		case tea.MouseWheelUp:
			if t.cursor > 0 {
				t.cursor--
				t.ensureVisible()
			}
		case tea.MouseWheelDown:
			if t.cursor < len(t.rows)-1 {
				t.cursor++
				t.ensureVisible()
			}
		}

	case tea.KeyMsg:
		if !t.focused {
			return t, nil
		}
		switch msg.String() {
		case "up", "k":
			if t.cursor > 0 {
				t.cursor--
				t.ensureVisible()
			}
		case "down", "j":
			if t.cursor < len(t.rows)-1 {
				t.cursor++
				t.ensureVisible()
			}
		case "enter":
			if f := t.SelectedFilter(); f != nil {
				return t, func() tea.Msg { return DisplayFilterToggleMsg{Filter: *f} }
			}
		}
	}
	return t, nil
}

func (t *TLSInspectorWidget) ensureVisible() {
	vis := t.height - 6
	if vis < 1 {
		vis = 1
	}
	if t.cursor < t.offset {
		t.offset = t.cursor
	}
	if t.cursor >= t.offset+vis {
		t.offset = t.cursor - vis + 1
	}
	if t.offset < 0 {
		t.offset = 0
	}
}

func (t *TLSInspectorWidget) View() string {
	cw := t.width - 2
	ch := t.height - 2
	if cw < 5 || ch < 2 {
		return t.borderStyle.Width(cw).Render("TLS")
	}

	border := t.borderStyle
	if t.focused {
		border = t.focusBorder
	}

	title := t.titleStyle.Render(fmt.Sprintf(" TLS Inspector (%d)", t.totalHS))

	versionBar := t.versionBar(cw)

	colSNI := cw - 18
	if colSNI < 10 {
		colSNI = 10
	}
	header := fmt.Sprintf("%-*s %-8s %-8s", colSNI, "SNI", "Version", "Cipher")
	header = t.headerStyle.Width(cw).Render(truncStr(header, cw))

	lines := []string{title, versionBar, header}
	vis := ch - 3
	for i := t.offset; i < t.offset+vis && i < len(t.rows); i++ {
		r := t.rows[i]
		line := fmt.Sprintf("%-*s %-8s %-8s",
			colSNI, truncStr(r.SNI, colSNI),
			truncStr(r.Version, 8),
			truncStr(r.Cipher, 8))

		if i == t.cursor && t.focused {
			lines = append(lines, t.selectedStyle.Width(cw).Render(truncStr(line, cw)))
		} else {
			lines = append(lines, t.versionStyle(r.Version).Width(cw).Render(truncStr(line, cw)))
		}
	}
	for len(lines) < ch {
		lines = append(lines, strings.Repeat(" ", cw))
	}
	return border.Width(cw).Render(strings.Join(lines, "\n"))
}

func (t *TLSInspectorWidget) versionBar(width int) string {
	if t.totalHS == 0 {
		return strings.Repeat("░", width)
	}
	versions := []string{"TLS 1.3", "TLS 1.2", "TLS 1.1", "TLS 1.0", "SSL 3.0"}
	var parts []string
	remaining := width
	for _, v := range versions {
		count := t.versionCounts[v]
		if count == 0 {
			continue
		}
		w := int(float64(count) / float64(t.totalHS) * float64(width))
		if w < 1 {
			w = 1
		}
		if w > remaining {
			w = remaining
		}
		style := t.versionStyle(v)
		parts = append(parts, style.Render(strings.Repeat("█", w)))
		remaining -= w
	}
	if remaining > 0 {
		parts = append(parts, strings.Repeat("░", remaining))
	}
	return strings.Join(parts, "")
}

func (t *TLSInspectorWidget) versionStyle(v string) lipgloss.Style {
	switch v {
	case "TLS 1.3":
		return t.tls13Style
	case "TLS 1.2":
		return t.tls12Style
	case "TLS 1.0", "TLS 1.1", "SSL 3.0":
		return t.tls10Style
	default:
		return t.normalStyle
	}
}
