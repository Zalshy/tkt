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
// All maps are nil-safe (nil == no tickets yet).
type statsData struct {
	byStatus    map[string]int // key: status string
	byAttention map[string]int // key: band label ("unset"|"low"|"medium"|"high"|"critical")
	byMainType  map[string]int // key: main_type string ("" maps to "(none)")
}

// attentionBand converts a numeric attention level to a band label.
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

// loadStats runs a single lightweight query and returns pre-grouped counts.
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
		var attentionLevel int
		if err := rows.Scan(&status, &mainType, &attentionLevel); err != nil {
			return statsData{}, fmt.Errorf("stats.loadStats: scan: %w", err)
		}

		s.byStatus[status]++
		s.byAttention[attentionBand(attentionLevel)]++

		key := mainType
		if key == "" {
			key = "(none)"
		}
		s.byMainType[key]++
	}

	return s, rows.Err()
}

// colRow is a single label+count line for a stats column.
type colRow struct {
	label string // plain text label
	color lipgloss.Color
	count int
}

// renderStatCol renders one vertical stats column (header + rows) into a
// string of exactly colWidth columns. Each row shows a colored label and a
// right-aligned count. A mini bar (█/░) fills the remaining space.
func renderStatCol(header string, rows []colRow, colWidth int) string {
	if colWidth < 4 {
		colWidth = 4
	}

	headerLine := lipgloss.NewStyle().
		Foreground(styles.Secondary).
		Bold(false).
		Width(colWidth).
		Render(header)

	maxCount := 0
	for _, r := range rows {
		if r.count > maxCount {
			maxCount = r.count
		}
	}

	// label(8) + " " + bar + " " + count(3) = colWidth
	const labelW = 8
	const countW = 3
	const fixed = labelW + 1 + 1 + countW // label + space + space + count
	barW := colWidth - fixed
	if barW < 1 {
		barW = 1
	}

	var lines []string
	lines = append(lines, headerLine)
	for _, r := range rows {
		label := r.label
		if len(label) > labelW {
			label = label[:labelW-1] + "…"
		}

		filled := 0
		if maxCount > 0 {
			filled = int(float64(barW) * float64(r.count) / float64(maxCount))
		}
		empty := barW - filled

		labelStr := lipgloss.NewStyle().Foreground(r.color).Render(fmt.Sprintf("%-*s", labelW, label))
		barStr := lipgloss.NewStyle().Foreground(r.color).Render(strings.Repeat("█", filled)) +
			lipgloss.NewStyle().Foreground(styles.Faint).Render(strings.Repeat("░", empty))
		countStr := lipgloss.NewStyle().Foreground(styles.Muted).Render(fmt.Sprintf("%*d", countW, r.count))

		line := labelStr + " " + barStr + " " + countStr
		lines = append(lines, line)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderStats renders the STATS section as three equal vertical columns:
// by status | by attention level | by main_type.
// The border is applied in model.go via sectionStyle.
func renderStats(s statsData, width int) string {
	// Zero-data guard.
	if s.byStatus == nil && s.byAttention == nil && s.byMainType == nil {
		header := lipgloss.NewStyle().
			Foreground(styles.Primary).
			Bold(true).
			Render("STATS")
		loading := lipgloss.NewStyle().
			Foreground(styles.Muted).
			Render("  loading…")
		return lipgloss.JoinVertical(lipgloss.Left, header, loading)
	}

	// Section header.
	sectionHeader := lipgloss.NewStyle().
		Foreground(styles.Primary).
		Bold(true).
		Render("STATS")

	// Divide width into 3 equal columns, with 1-char separator gaps.
	// total = 3*colW + 2*1 → colW = (width - 2) / 3
	colWidth := (width - 2) / 3
	if colWidth < 8 {
		colWidth = 8
	}

	sep := lipgloss.NewStyle().Foreground(styles.Faint).Render("│")

	// — Column 1: by status —
	statusOrder := []string{"TODO", "PLANNING", "IN_PROGRESS", "DONE", "VERIFIED"}
	statusColors := map[string]lipgloss.Color{
		"TODO":        styles.StatusTodo,
		"PLANNING":    styles.StatusPlanning,
		"IN_PROGRESS": styles.StatusInProg,
		"DONE":        styles.StatusDone,
		"VERIFIED":    styles.StatusVerified,
	}
	var statusRows []colRow
	for _, st := range statusOrder {
		statusRows = append(statusRows, colRow{
			label: st,
			color: statusColors[st],
			count: s.byStatus[st],
		})
	}
	col1 := renderStatCol("by status", statusRows, colWidth)

	// — Column 2: by attention level —
	attentionOrder := []string{"critical", "high", "medium", "low", "unset"}
	attentionColors := map[string]lipgloss.Color{
		"critical": styles.AttentionE,
		"high":     styles.AttentionD,
		"medium":   styles.AttentionC,
		"low":      styles.AttentionA,
		"unset":    styles.Muted,
	}
	var attnRows []colRow
	for _, band := range attentionOrder {
		attnRows = append(attnRows, colRow{
			label: band,
			color: attentionColors[band],
			count: s.byAttention[band],
		})
	}
	col2 := renderStatCol("by attention", attnRows, colWidth)

	// — Column 3: by main_type — top 5, sorted by count desc then alpha.
	type nameCount struct {
		name  string
		count int
	}
	var types []nameCount
	for k, v := range s.byMainType {
		types = append(types, nameCount{k, v})
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
	var typeRows []colRow
	for _, tc := range types {
		typeRows = append(typeRows, colRow{
			label: tc.name,
			color: styles.Secondary,
			count: tc.count,
		})
	}
	col3 := renderStatCol("by type", typeRows, colWidth)

	// Join the three columns side-by-side, row by row.
	col1Lines := strings.Split(col1, "\n")
	col2Lines := strings.Split(col2, "\n")
	col3Lines := strings.Split(col3, "\n")

	maxLines := len(col1Lines)
	if len(col2Lines) > maxLines {
		maxLines = len(col2Lines)
	}
	if len(col3Lines) > maxLines {
		maxLines = len(col3Lines)
	}

	empty1 := lipgloss.NewStyle().Width(colWidth).Render("")
	empty2 := lipgloss.NewStyle().Width(colWidth).Render("")
	empty3 := lipgloss.NewStyle().Width(colWidth).Render("")

	var sb strings.Builder
	sb.WriteString(sectionHeader)
	sb.WriteString("\n")
	for i := 0; i < maxLines; i++ {
		l1, l2, l3 := empty1, empty2, empty3
		if i < len(col1Lines) {
			l1 = col1Lines[i]
		}
		if i < len(col2Lines) {
			l2 = col2Lines[i]
		}
		if i < len(col3Lines) {
			l3 = col3Lines[i]
		}
		sb.WriteString(l1 + sep + l2 + sep + l3)
		if i < maxLines-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
