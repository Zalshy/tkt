package side

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderCometBar renders one content row for the comet box.
//
// The comet shape replicates renderProgressBar from the kanban IN_PROGRESS
// card exactly: a █ head with ▓▒░ trailing chars, everything else is space.
//
// Unlike the card version (single colour, wraps), this one:
//   - uses the ping-pong position (pos ∈ [0..1])
//   - colours every character at its own x-position via statusPaletteColor,
//     so the comet reads as a reflection of the header bar gradient
//   - uses dirBlend ∈ [-1..+1] to smoothly shift the tail between the left
//     and right sides during a direction reversal, instead of snapping
//
// Tail character brightness at distance d from head:
//
//	d=0 → 1.0 (█)   d=1 → 0.75 (▓)   d=2 → 0.50 (▒)   d=3 → 0.25 (░)
//
// Each side's tail is scaled by its weight:
//
//	leftWeight  = (dirBlend+1)/2  — 1 going right, 0 going left
//	rightWeight = (1-dirBlend)/2  — 0 going right, 1 going left
func renderCometBar(pos float64, dirBlend float64, width int) string {
	if width <= 0 {
		return ""
	}

	denom := float64(width - 1)
	if denom == 0 {
		denom = 1
	}

	headCol := int(math.Round(pos * denom))
	leftWeight := (dirBlend + 1.0) / 2.0
	rightWeight := (1.0 - dirBlend) / 2.0

	tailBrightness := func(dist int) float64 {
		switch dist {
		case 1:
			return 0.75
		case 2:
			return 0.50
		case 3:
			return 0.25
		}
		return 0.0
	}

	charForBrightness := func(b float64) rune {
		switch {
		case b >= 1.0:
			return '█'
		case b >= 0.60:
			return '▓'
		case b >= 0.35:
			return '▒'
		case b >= 0.10:
			return '░'
		default:
			return ' '
		}
	}

	var sb strings.Builder
	for col := 0; col < width; col++ {
		var brightness float64
		if col == headCol {
			brightness = 1.0
		} else if col < headCol {
			brightness = tailBrightness(headCol-col) * leftWeight
		} else {
			brightness = tailBrightness(col-headCol) * rightWeight
		}

		ch := charForBrightness(brightness)
		if ch == ' ' {
			sb.WriteByte(' ')
		} else {
			colColor := statusPaletteColor(float64(col) / denom)
			sb.WriteString(lipgloss.NewStyle().Foreground(colColor).Render(string(ch)))
		}
	}
	return sb.String()
}
