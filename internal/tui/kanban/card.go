package kanban

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/tui/styles"
)

// CardHeight is the number of rendered lines a card occupies.
const CardHeight = 2

func tierColor(tier string) lipgloss.Color {
	switch tier {
	case "critical":
		return styles.TierCritical
	case "low":
		return styles.TierLow
	default: // "standard" and anything else
		return styles.TierStandard
	}
}

// rainbowColors cycles through these vibrant hues for the IN PROGRESS tag.
var rainbowColors = []lipgloss.Color{
	"#22D3EE", // cyan
	"#F472B6", // pink
	"#FBBF24", // amber
	"#A78BFA", // violet
}

// renderRainbow renders each non-space rune in s with a cycling vibrant color
// (bold). Space characters are rendered with selBg when selected is true,
// otherwise as plain spaces.
func renderRainbow(s string, selected bool) string {
	var sb strings.Builder
	ci := 0
	for _, r := range s {
		if r == ' ' {
			if selected {
				sb.WriteString(lipgloss.NewStyle().Background(styles.Accent).Render(" "))
			} else {
				sb.WriteRune(' ')
			}
		} else {
			st := lipgloss.NewStyle().
				Foreground(rainbowColors[ci%len(rainbowColors)]).
				Bold(true)
			if selected {
				st = st.Background(styles.Accent)
			}
			sb.WriteString(st.Render(string(r)))
			ci++
		}
	}
	return sb.String()
}

// renderCard renders a single ticket card at the given width.
// Layout:
//
//	Line 1:  │  #ID [tier]          STATUS TAG   (status tag right-aligned, if space)
//	Line 2:  │  title (full inner width)
//
// selected: cursor is on this card. focused: the column owning this card is active.
func renderCard(t models.Ticket, width int, selected, focused bool) string {
	innerWidth := width - 2 // left border takes 1 col, right side has 1 col padding
	if innerWidth < 4 {
		innerWidth = 4
	}

	color := tierColor(t.Tier)

	// ── Status tag ──────────────────────────────────────────────────────────
	// IN_PROGRESS: rainbow bold text.  VERIFIED: emerald bold text.
	const inProgressTag = "IN PROGRESS"
	const verifiedTag = "VERIFIED"

	var tagText string
	var tagIsRainbow bool
	var tagColor lipgloss.Color
	switch t.Status {
	case models.StatusInProgress:
		tagText = inProgressTag
		tagIsRainbow = true
	case models.StatusVerified:
		tagText = verifiedTag
		tagColor = styles.StatusVerified
	}

	// ── Line 1 ──────────────────────────────────────────────────────────────
	leftStr := fmt.Sprintf("#%d [%s]", t.ID, t.Tier)
	leftRunes := len([]rune(leftStr))
	tagRunes := len([]rune(tagText))

	// Minimum gap between left text and tag so they don't run together.
	const minGap = 2

	var line1 string
	if tagText != "" && leftRunes+minGap+tagRunes <= innerWidth {
		gap := innerWidth - leftRunes - tagRunes

		leftSt := lipgloss.NewStyle().Foreground(color)
		if selected {
			leftSt = leftSt.Background(styles.Accent)
		}

		spacer := strings.Repeat(" ", gap)
		if selected {
			spacer = lipgloss.NewStyle().Background(styles.Accent).Render(spacer)
		}

		var tagRendered string
		if tagIsRainbow {
			tagRendered = renderRainbow(tagText, selected)
		} else {
			tagSt := lipgloss.NewStyle().Foreground(tagColor).Bold(true)
			if selected {
				tagSt = tagSt.Background(styles.Accent)
			}
			tagRendered = tagSt.Render(tagText)
		}

		line1 = leftSt.Render(leftStr) + spacer + tagRendered
	} else {
		// Tag doesn't fit (or no tag) — just show left part padded to fill the line.
		st := lipgloss.NewStyle().Foreground(color)
		if selected {
			st = st.Background(styles.Accent)
		}
		line1 = st.Render(padRight(truncate(leftStr, innerWidth), innerWidth))
	}

	// ── Line 2: title ───────────────────────────────────────────────────────
	title := padRight(truncate(t.Title, innerWidth), innerWidth)
	titleSt := lipgloss.NewStyle().Foreground(styles.Primary)
	if selected {
		titleSt = titleSt.Background(styles.Accent).Bold(true)
	}

	// ── Left border ─────────────────────────────────────────────────────────
	borderColor := color
	if !focused && !selected {
		borderColor = styles.Faint
	}
	border := lipgloss.NewStyle().Foreground(borderColor).Render("│")

	return border + line1 + "\n" + border + titleSt.Render(title)
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}

func padRight(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return string(runes[:width])
	}
	for len(runes) < width {
		runes = append(runes, ' ')
	}
	return string(runes)
}
