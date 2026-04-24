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

// attentionColor returns the foreground color for a given attention level.
func attentionColor(level int) lipgloss.Color {
	if level <= 0 {
		return styles.Muted
	} else if level <= 20 {
		return styles.AttentionA
	} else if level <= 40 {
		return styles.AttentionB
	} else if level <= 60 {
		return styles.AttentionC
	} else if level <= 80 {
		return styles.AttentionD
	} else {
		return styles.AttentionE
	}
}

// tierBadge returns the bracket-wrapped tier label.
func tierBadge(tier string) string {
	switch tier {
	case "low":
		return "[low]"
	case "critical":
		return "[critical]"
	default:
		return "[standard]"
	}
}

// attentionTierLabel returns the badge text and color for a ticket's attention
// level, falling back to the tier badge when level == 0.
func attentionTierLabel(level int, fallbackTier string) (string, lipgloss.Color) {
	if level == 0 {
		return tierBadge(fallbackTier), tierColor(fallbackTier)
	} else if level <= 20 {
		return "[low]", styles.AttentionA
	} else if level <= 33 {
		return "[low]", styles.AttentionB
	} else if level <= 66 {
		return "[standard]", styles.AttentionC
	} else if level <= 80 {
		return "[critical]", styles.AttentionD
	} else {
		return "[critical]", styles.AttentionE
	}
}

// attentionDisplay returns the attention level display string for line 3.
func attentionDisplay(level int) string {
	if level >= 1 {
		return fmt.Sprintf("👁 %d", level)
	}
	return "👁 --"
}

// statusLabel returns the display text and color for a ticket status.
func statusLabel(s models.Status) (string, lipgloss.Color) {
	switch s {
	case models.StatusInProgress:
		return "IN PROGRESS", styles.StatusInProg
	case models.StatusVerified:
		return "VERIFIED", styles.StatusVerified
	case models.StatusDone:
		return "DONE", styles.StatusDone
	case models.StatusTodo:
		return "TODO", styles.StatusTodo
	case models.StatusPlanning:
		return "PLANNING", styles.StatusPlanning
	case models.StatusCanceled:
		return "CANCELED", styles.Danger
	case models.StatusArchived:
		return "ARCHIVED", styles.Muted
	default:
		return "UNKNOWN", styles.Muted
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
//	Line 1:  #ID     [tier/attention badge]     main_type
//	Line 2:  Title (truncated)
//	Line 3:  STATUS                              👁 N
//	Line 4:  ████▓▒░  (IN_PROGRESS only)
//
// selected: cursor is on this card. focused: the column owning this card is active.
func renderCard(t models.Ticket, width int, selected, focused bool, tickN int) string {
	innerWidth := width - 2 // left border takes 1 col, right side has 1 col padding
	if innerWidth < 4 {
		innerWidth = 4
	}

	color := tierColor(t.Tier)

	// ── Line 1 ──────────────────────────────────────────────────────────────
	// 3 segments: left=#ID, center=badge, right=MainType
	leftStr := fmt.Sprintf("#%d", t.ID)
	leftRunes := len([]rune(leftStr))

	badgeText, badgeColor := attentionTierLabel(t.AttentionLevel, t.Tier)
	badgeRunes := len([]rune(badgeText))

	rightMax := max(0, innerWidth/4)
	rightStr := truncate(t.MainType, rightMax)
	rightRunes := len([]rune(rightStr))

	centerAvail := innerWidth - leftRunes - rightRunes
	// shrink rightStr if badge won't fit
	if badgeRunes > centerAvail-2 {
		rightRunes = max(0, innerWidth-leftRunes-badgeRunes-2)
		rightStr = truncate(t.MainType, rightRunes)
		rightRunes = len([]rune(rightStr))
		centerAvail = innerWidth - leftRunes - rightRunes
	}

	var gap1, gap2 int
	// clamp for extreme narrow cards
	if centerAvail < badgeRunes {
		gap1 = 0
		gap2 = 0
		badgeText = truncate(badgeText, centerAvail)
	} else {
		gap1 = (centerAvail - badgeRunes) / 2
		gap2 = centerAvail - badgeRunes - gap1
	}

	leftSt := lipgloss.NewStyle().Foreground(color)
	if selected {
		leftSt = leftSt.Background(styles.Accent)
	}
	badgeSt := lipgloss.NewStyle().Foreground(badgeColor)
	if selected {
		badgeSt = badgeSt.Background(styles.Accent)
	}
	rightSt := lipgloss.NewStyle().Foreground(styles.Secondary)
	if selected {
		rightSt = rightSt.Background(styles.Accent)
	}

	spacer1 := strings.Repeat(" ", gap1)
	spacer2 := strings.Repeat(" ", gap2)
	if selected {
		spacerSt := lipgloss.NewStyle().Background(styles.Accent)
		spacer1 = spacerSt.Render(spacer1)
		spacer2 = spacerSt.Render(spacer2)
	}

	line1 := leftSt.Render(leftStr) + spacer1 + badgeSt.Render(badgeText) + spacer2 + rightSt.Render(rightStr)

	// ── Line 2: title ───────────────────────────────────────────────────────
	title := padRight(truncate(t.Title, innerWidth), innerWidth)
	titleSt := lipgloss.NewStyle().Foreground(styles.Primary)
	if selected {
		titleSt = titleSt.Background(styles.Accent).Bold(true)
	}

	// ── Line 3: status left, attention right ────────────────────────────────
	statusText, statusColor := statusLabel(t.Status)
	dispText := attentionDisplay(t.AttentionLevel)
	dispColor := attentionColor(t.AttentionLevel)

	// Use lipgloss.Width() — 👁 is 2 terminal columns
	statusW := lipgloss.Width(statusText)
	dispW := lipgloss.Width(dispText)
	gap3 := max(0, innerWidth-statusW-dispW)

	statusSt := lipgloss.NewStyle().Foreground(statusColor)
	if selected {
		statusSt = statusSt.Background(styles.Accent)
	}
	dispSt := lipgloss.NewStyle().Foreground(dispColor)
	if selected {
		dispSt = dispSt.Background(styles.Accent)
	}
	spacer3 := strings.Repeat(" ", gap3)
	if selected {
		spacer3 = lipgloss.NewStyle().Background(styles.Accent).Render(spacer3)
	}

	line3 := statusSt.Render(statusText) + spacer3 + dispSt.Render(dispText)

	// ── Left border ─────────────────────────────────────────────────────────
	borderColor := color
	if !focused && !selected {
		borderColor = styles.Faint
	}
	border := lipgloss.NewStyle().Foreground(borderColor).Render("│")

	result := border + line1 + "\n" + border + titleSt.Render(title) + "\n" + border + line3
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
