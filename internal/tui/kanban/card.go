package kanban

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/tui/styles"
)

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

// renderProgressBar renders a comet-tail scanning bar of width chars.
// Head advances left-to-right at tickN%width; tail decays behind it.
// dist = (head - i + width) % width: 0→█ 1→▓ 2→▒ 3→░ else space.
func renderProgressBar(width, tickN int, selected bool) string {
	if width <= 0 {
		return ""
	}
	head := tickN % width
	var sb strings.Builder
	for i := 0; i < width; i++ {
		dist := (head - i + width) % width
		var ch rune
		switch dist {
		case 0:
			ch = '█'
		case 1:
			ch = '▓'
		case 2:
			ch = '▒'
		case 3:
			ch = '░'
		default:
			ch = ' '
		}
		st := lipgloss.NewStyle().Foreground(styles.StatusInProg)
		if selected {
			st = st.Background(styles.Accent)
		}
		sb.WriteString(st.Render(string(ch)))
	}
	return sb.String()
}

// renderCard renders a single ticket card at the given width.
// Layout:
//
//	Line 1:  │  #ID [tier]          STATUS TAG   (status tag right-aligned, if space)
//	Line 2:  │  title (full inner width)
//	Line 3:  │  progress bar (IN_PROGRESS cards only)
//
// selected: cursor is on this card. focused: the column owning this card is active.
func renderCard(t models.Ticket, width int, selected, focused bool, tickN int) string {
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
	var tagColor lipgloss.Color
	switch t.Status {
	case models.StatusInProgress:
		tagText = inProgressTag
		tagColor = styles.StatusInProg
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

		tagSt := lipgloss.NewStyle().Foreground(tagColor).Bold(true)
		if selected {
			tagSt = tagSt.Background(styles.Accent)
		}
		tagRendered := tagSt.Render(tagText)

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

	result := border + line1 + "\n" + border + titleSt.Render(title)
	if t.Status == models.StatusInProgress {
		result += "\n" + border + renderProgressBar(innerWidth, tickN, selected)
	}
	return result
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
