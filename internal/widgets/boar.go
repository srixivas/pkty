package widgets

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// boarFrames holds 4 animation frames of the ASCII boar (6 rows each, 15 chars wide).
//
// Stable rows (identical every frame):
//
//	row 1 — eyes      /(o  o)\
//	row 2 — snout     \| (;;)  |/  (tusks)
//	row 3 — chin      \___/
//
// Animated rows:
//
//	row 0 — ears      /\  /\  (relaxed)  vs  |\  /|  (pricked, odd frames)
//	row 4 — legs      forward → straight → back → lifting
//	row 5 — hooves    wide-out → straight → wide-in → straight
var boarFrames = [4][6]string{
	{ // Frame 0: ears relaxed, legs forward
		"      /\\  /\\   ",
		"     /(o  o)\\  ",
		"   \\| (;;)  |/ ",
		"      \\___/    ",
		"      //  \\\\   ",
		"     //    \\\\  ",
	},
	{ // Frame 1: ears pricked, legs straight
		"      |\\  /|   ",
		"     /(o  o)\\  ",
		"   \\| (;;)  |/ ",
		"      \\___/    ",
		"      ||  ||   ",
		"      |    |   ",
	},
	{ // Frame 2: ears relaxed, legs back
		"      /\\  /\\   ",
		"     /(o  o)\\  ",
		"   \\| (;;)  |/ ",
		"      \\___/    ",
		"      \\\\  //   ",
		"      \\\\    // ",
	},
	{ // Frame 3: ears pricked, legs lifting
		"      |\\  /|   ",
		"     /(o  o)\\  ",
		"   \\| (;;)  |/ ",
		"      \\___/    ",
		"      /|  |\\   ",
		"      |    |   ",
	},
}

// BoarWidget renders an animated ASCII boar with a capture-state status line.
// It does not implement the full Widget interface — layout uses it directly.
type BoarWidget struct {
	frame      int
	capturing  bool
	hasBackend bool
	width      int
}

// NewBoarWidget creates a BoarWidget with default state (no backend, not capturing).
func NewBoarWidget() *BoarWidget {
	return &BoarWidget{}
}

// Height returns the fixed height of the boar widget (6 art rows + 1 status row).
func (b *BoarWidget) Height() int { return 7 }

// SetWidth sets the render width used for centring the art.
func (b *BoarWidget) SetWidth(w int) { b.width = w }

// SetCapturing updates whether capture is active (controls animation and status text).
func (b *BoarWidget) SetCapturing(c bool) { b.capturing = c }

// SetHasBackend records whether a capture backend is wired up.
func (b *BoarWidget) SetHasBackend(h bool) { b.hasBackend = h }

// Advance steps to the next animation frame (called on each tick when capturing).
func (b *BoarWidget) Advance() { b.frame = (b.frame + 1) % 4 }

// View renders the boar widget as a 7-row string at the configured width.
func (b *BoarWidget) View() string {
	frame := boarFrames[b.frame]

	// Find the max art width so we can centre the block.
	artW := 0
	for _, row := range frame {
		if len(row) > artW {
			artW = len(row)
		}
	}

	pad := 0
	if b.width > artW {
		pad = (b.width - artW) / 2
	}
	prefix := strings.Repeat(" ", pad)

	artStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	var lines []string
	for _, row := range frame {
		lines = append(lines, artStyle.Render(prefix+row))
	}

	// Status line
	var statusLine string
	switch {
	case !b.hasBackend:
		statusLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(centreText("○ NO SOURCE", b.width))
	case b.capturing:
		statusLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")).
			Bold(true).
			Render(centreText("● CAPTURING", b.width))
	default:
		statusLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226")).
			Bold(true).
			Render(centreText("■ PAUSED", b.width))
	}
	lines = append(lines, statusLine)

	return strings.Join(lines, "\n")
}

// centreText centres s within width using space padding.
func centreText(s string, width int) string {
	if width <= len(s) {
		return s
	}
	pad := (width - len(s)) / 2
	return strings.Repeat(" ", pad) + s
}
