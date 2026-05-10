package widgets

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// owlFrames holds 4 animation frames of the ASCII owl (6 rows each, 15 chars wide).
//
// Row 0 — ear tufts:  /\_/\  (static)
// Row 1 — eyes:       animated — open (@v@) → open (@v@) → half (-v-) → closed (.v.)
// Row 2 — chest:      ():::()  (static)
// Row 3 — belly:       (   )   (static)
// Row 4 — wings:      animated — neutral /|\ → raised //|\\ → neutral /|\ → lowered \|/
// Row 5 — talons:      v   v   (static)
var owlFrames = [4][6]string{
	{ // Frame 0: eyes open, wings neutral
		"     /\\_/\\     ",
		"    ((@v@))    ",
		"    ():::()    ",
		"     (   )     ",
		"    /|   |\\    ",
		"     v   v     ",
	},
	{ // Frame 1: eyes open (glow peak), wings raised
		"     /\\_/\\     ",
		"    ((@v@))    ",
		"    ():::()    ",
		"     (   )     ",
		"   //|   |\\\\   ",
		"     v   v     ",
	},
	{ // Frame 2: eyes half-blink, wings neutral
		"     /\\_/\\     ",
		"    ((-v-))    ",
		"    ():::()    ",
		"     (   )     ",
		"    /|   |\\    ",
		"     v   v     ",
	},
	{ // Frame 3: eyes closed, wings lowered
		"     /\\_/\\     ",
		"    ((.v.))    ",
		"    ():::()    ",
		"     (   )     ",
		"    \\|   |/    ",
		"     v   v     ",
	},
}

// eyeColors maps each frame's eye row to a lipgloss color.
// Frames 0+1 glow bright yellow; frame 2 dims; frame 3 closes to dark amber.
var eyeColors = [4]string{"226", "226", "178", "136"}

// OwlWidget renders an animated ASCII owl with a capture-state status line.
// It does not implement the full Widget interface — layout uses it directly.
type OwlWidget struct {
	frame      int
	capturing  bool
	hasBackend bool
	width      int
}

// NewOwlWidget creates an OwlWidget with default state (no backend, not capturing).
func NewOwlWidget() *OwlWidget {
	return &OwlWidget{}
}

// Height returns the fixed height of the owl widget (6 art rows + 1 status row).
func (o *OwlWidget) Height() int { return 7 }

// SetWidth sets the render width used for centring the art.
func (o *OwlWidget) SetWidth(w int) { o.width = w }

// SetCapturing updates whether capture is active (controls animation and status text).
func (o *OwlWidget) SetCapturing(c bool) { o.capturing = c }

// SetHasBackend records whether a capture backend is wired up.
func (o *OwlWidget) SetHasBackend(h bool) { o.hasBackend = h }

// Advance steps to the next animation frame (called on each tick when capturing).
func (o *OwlWidget) Advance() { o.frame = (o.frame + 1) % 4 }

// View renders the owl widget as a 7-row string at the configured width.
func (o *OwlWidget) View() string {
	frame := owlFrames[o.frame]

	artW := 0
	for _, row := range frame {
		if len(row) > artW {
			artW = len(row)
		}
	}
	pad := 0
	if o.width > artW {
		pad = (o.width - artW) / 2
	}
	prefix := strings.Repeat(" ", pad)

	bodyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	eyeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(eyeColors[o.frame])).
		Bold(o.frame <= 1)

	var lines []string
	for i, row := range frame {
		style := bodyStyle
		if i == 1 {
			style = eyeStyle
		}
		lines = append(lines, style.Render(prefix+row))
	}

	var statusLine string
	switch {
	case !o.hasBackend:
		statusLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(centreText("○ NO SOURCE", o.width))
	case o.capturing:
		statusLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")).
			Bold(true).
			Render(centreText("● CAPTURING", o.width))
	default:
		statusLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226")).
			Bold(true).
			Render(centreText("■ PAUSED", o.width))
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
