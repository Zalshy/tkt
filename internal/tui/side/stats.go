package side

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/tui/styles"
)

// statsData holds pre-computed breakdowns for all three sections.
type statsData struct {
	byStatus    map[string]int
	byAttention map[string]int
	byMainType  map[string]int
}

func attentionBand(level int) string {
	switch {
	case level == 0:
		return "unset"
	case level <= 20:
		return "low"
	case level <= 40:
		return "medium"
	case level <= 60:
		return "high"
	default:
		return "critical"
	}
}

func loadStats(db *sql.DB) (statsData, error) {
	if db == nil {
		return statsData{}, nil
	}
	rows, err := db.Query(`
		SELECT status, main_type, attention_level
		FROM tickets
		WHERE status NOT IN ('CANCELED','ARCHIVED')
		  AND deleted_at IS NULL
	`)
	if err != nil {
		return statsData{}, fmt.Errorf("stats.loadStats: query: %w", err)
	}
	defer rows.Close()

	s := statsData{
		byStatus:    make(map[string]int),
		byAttention: make(map[string]int),
		byMainType:  make(map[string]int),
	}
	for rows.Next() {
		var status, mainType string
		var attn int
		if err := rows.Scan(&status, &mainType, &attn); err != nil {
			return statsData{}, fmt.Errorf("stats.loadStats: scan: %w", err)
		}
		s.byStatus[status]++
		s.byAttention[attentionBand(attn)]++
		key := mainType
		if key == "" {
			key = "(none)"
		}
		s.byMainType[key]++
	}
	return s, rows.Err()
}

// statRow is a single labeled count entry for a stat box.
type statRow struct {
	label string
	color lipgloss.Color
	count int
}

// renderStatBox renders a single bordered stat box with a bold centred title
// and a list of label+bar+count rows. Width includes the border.
func renderStatBox(title string, rows []statRow, width int) string {
	innerW := width - 2 // subtract border columns
	if innerW < 4 {
		innerW = 4
	}

	titleStr := lipgloss.NewStyle().
		Foreground(styles.Primary).
		Bold(true).
		Width(innerW).
		Align(lipgloss.Center).
		Render(title)

	// bar width: innerW - labelW(8) - space(1) - space(1) - countW(3)
	const labelW = 8
	const countW = 3
	barW := innerW - labelW - 1 - 1 - countW
	if barW < 1 {
		barW = 1
	}

	var lines []string
	lines = append(lines, titleStr)
	lines = append(lines, "") // blank line under title

	for _, r := range rows {
		label := r.label
		if len(label) > labelW {
			label = label[:labelW-1] + "…"
		}

		// Each "ticket" = one ▌ (left-half-block) in the row colour.
		// The transparent right half of each character cell acts as a very
		// thin natural separator — no explicit gap character needed.
		// filledCells = exact count; cap at barW to avoid overflow.
		const cellW = 1       // 1 char per ticket
		const ellipsisW = 3   // "..."

		overflow := r.count > barW
		showCells := r.count
		if overflow {
			showCells = barW - ellipsisW
			if showCells < 0 {
				showCells = 0
			}
		}

		labelStr := lipgloss.NewStyle().Foreground(r.color).
			Render(fmt.Sprintf("%-*s", labelW, label))

		var barSb strings.Builder
		cellStyle := lipgloss.NewStyle().Foreground(r.color)
		for i := 0; i < showCells; i++ {
			barSb.WriteString(cellStyle.Render("▌"))
		}
		if overflow {
			barSb.WriteString(lipgloss.NewStyle().Foreground(r.color).Bold(true).Render("..."))
			if rem := barW - showCells*cellW - ellipsisW; rem > 0 {
				barSb.WriteString(strings.Repeat(" ", rem))
			}
		} else {
			if rem := barW - showCells*cellW; rem > 0 {
				barSb.WriteString(strings.Repeat(" ", rem))
			}
		}

		countStr := lipgloss.NewStyle().Foreground(r.color).Bold(true).
			Render(fmt.Sprintf("%*d", countW, r.count))

		lines = append(lines, labelStr+" "+barSb.String()+" "+countStr)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	return styles.PanelInactive.
		Width(innerW).
		Render(content)
}

// renderStatsRow renders the three stat boxes side by side as one string.
// totalWidth is the full available width (outer border included in each box).
func renderStatsRow(s statsData, totalWidth int) string {
	if s.byStatus == nil && s.byAttention == nil && s.byMainType == nil {
		return lipgloss.NewStyle().
			Foreground(styles.Muted).
			Render("  loading…")
	}

	// Divide into 3 equal boxes.
	boxW := totalWidth / 3
	if boxW < 10 {
		boxW = 10
	}

	// — By Status —
	statusOrder := []string{"TODO", "PLANNING", "IN_PROGRESS", "DONE", "VERIFIED"}
	statusColors := map[string]lipgloss.Color{
		"TODO":        styles.StatusTodo,
		"PLANNING":    styles.StatusPlanning,
		"IN_PROGRESS": styles.StatusInProg,
		"DONE":        styles.StatusDone,
		"VERIFIED":    styles.StatusVerified,
	}
	var statusRows []statRow
	for _, st := range statusOrder {
		statusRows = append(statusRows, statRow{strings.ToLower(st), statusColors[st], s.byStatus[st]})
	}

	// — By Attention —
	attnOrder := []string{"critical", "high", "medium", "low", "unset"}
	attnColors := map[string]lipgloss.Color{
		"critical": styles.AttentionE,
		"high":     styles.AttentionD,
		"medium":   styles.AttentionC,
		"low":      styles.AttentionA,
		"unset":    styles.Muted,
	}
	var attnRows []statRow
	for _, band := range attnOrder {
		attnRows = append(attnRows, statRow{band, attnColors[band], s.byAttention[band]})
	}

	// — By Type — top 5
	type nc struct{ name string; count int }
	var types []nc
	for k, v := range s.byMainType {
		types = append(types, nc{k, v})
	}
	sort.Slice(types, func(i, j int) bool {
		if types[i].name == "(none)" {
			return false
		}
		if types[j].name == "(none)" {
			return true
		}
		if types[i].count != types[j].count {
			return types[i].count > types[j].count
		}
		return types[i].name < types[j].name
	})
	if len(types) > 5 {
		types = types[:5]
	}
	// Colors assigned by position — works for any type name.
	typePalette := []lipgloss.Color{
		lipgloss.Color("#38BDF8"), // sky blue
		lipgloss.Color("#F87171"), // coral red
		lipgloss.Color("#FBBF24"), // amber
		lipgloss.Color("#2DD4BF"), // teal
		lipgloss.Color("#F472B6"), // pink
	}
	var typeRows []statRow
	for i, tc := range types {
		col := typePalette[i%len(typePalette)]
		typeRows = append(typeRows, statRow{tc.name, col, tc.count})
	}

	box1 := renderStatBox("By Status", statusRows, boxW)
	box2 := renderStatBox("By Attention", attnRows, boxW)
	box3 := renderStatBox("By Type", typeRows, boxW)

	return lipgloss.JoinHorizontal(lipgloss.Top, box1, box2, box3)
}
