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

// renderStats renders the STATS section as a plain string (no border).
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

	var sb strings.Builder

	// Section header.
	sb.WriteString(lipgloss.NewStyle().
		Foreground(styles.Primary).
		Bold(true).
		Render("STATS"))
	sb.WriteString("\n\n")

	// Sub-header: by status.
	sb.WriteString(lipgloss.NewStyle().
		Foreground(styles.Secondary).
		Render("by status"))
	sb.WriteString("\n")

	// Status bar chart.
	statusOrder := []string{"TODO", "PLANNING", "IN_PROGRESS", "DONE", "VERIFIED"}
	statusColors := map[string]lipgloss.Color{
		"TODO":        styles.StatusTodo,
		"PLANNING":    styles.StatusPlanning,
		"IN_PROGRESS": styles.StatusInProg,
		"DONE":        styles.StatusDone,
		"VERIFIED":    styles.StatusVerified,
	}

	const labelWidth = 12
	const countWidth = 4 // space + 3 digit count
	const padding = 1

	// barWidth = width - labelWidth - countWidth - padding, clamped to min 4.
	barWidth := width - labelWidth - countWidth - padding
	if barWidth < 4 {
		barWidth = 4
	}

	// Find max count for normalisation.
	maxCount := 0
	for _, st := range statusOrder {
		if n := s.byStatus[st]; n > maxCount {
			maxCount = n
		}
	}

	for _, st := range statusOrder {
		count := s.byStatus[st]
		color := statusColors[st]

		// Label: left-aligned, padded to labelWidth.
		label := fmt.Sprintf("  %-*s", labelWidth-2, st)

		// Bar: filled + empty chars, max-normalised.
		filled := 0
		if maxCount > 0 {
			filled = int(float64(barWidth) * float64(count) / float64(maxCount))
		}
		empty := barWidth - filled

		filledStr := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled))
		emptyStr := lipgloss.NewStyle().Foreground(styles.Faint).Render(strings.Repeat("░", empty))

		countStr := fmt.Sprintf("%3d", count)

		sb.WriteString(label + filledStr + emptyStr + " " + countStr + "\n")
	}

	sb.WriteString("\n")

	// Two-column section: attention level + main_type.
	colWidth := (width - 3) / 2
	if colWidth < 4 {
		colWidth = 4
	}

	// Sub-headers.
	attnHeader := lipgloss.NewStyle().Foreground(styles.Secondary).Render("by attention level")
	typeHeader := lipgloss.NewStyle().Foreground(styles.Secondary).Render("by main_type")
	sep := lipgloss.NewStyle().Foreground(styles.Faint).Render("│")

	attnHeaderPad := lipgloss.NewStyle().Width(colWidth).Render(attnHeader)
	typeHeaderPad := lipgloss.NewStyle().Width(colWidth).Render(typeHeader)
	sb.WriteString(attnHeaderPad + sep + typeHeaderPad + "\n")

	// Attention column rows.
	attentionOrder := []string{"critical", "high", "medium", "low", "unset"}
	attentionColors := map[string]lipgloss.Color{
		"critical": styles.AttentionE,
		"high":     styles.AttentionD,
		"medium":   styles.AttentionC,
		"low":      styles.AttentionA,
		"unset":    styles.Muted,
	}

	var attnRows []string
	if len(s.byAttention) == 0 {
		attnRows = []string{
			lipgloss.NewStyle().Foreground(styles.Muted).Render("  (none)"),
		}
	} else {
		for _, band := range attentionOrder {
			n := s.byAttention[band]
			color := attentionColors[band]
			label := lipgloss.NewStyle().Foreground(color).Render(band)
			// Right-align count within available space.
			labelRaw := fmt.Sprintf("  %-8s %3d", band, n)
			_ = labelRaw
			line := fmt.Sprintf("  %s %3d", label, n)
			attnRows = append(attnRows, line)
		}
	}

	// Main type column rows: sorted by count desc, then alpha; "(none)" last.
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
	// Top 5 only.
	if len(types) > 5 {
		types = types[:5]
	}

	var typeRows []string
	if len(types) == 0 {
		typeRows = []string{
			lipgloss.NewStyle().Foreground(styles.Muted).Render("  (none)"),
		}
	} else {
		for _, tc := range types {
			line := fmt.Sprintf("  %-8s %3d", tc.name, tc.count)
			typeRows = append(typeRows, line)
		}
	}

	// Render rows side by side, padding to colWidth with separator.
	maxRows := len(attnRows)
	if len(typeRows) > maxRows {
		maxRows = len(typeRows)
	}
	for i := 0; i < maxRows; i++ {
		var left, right string
		if i < len(attnRows) {
			left = attnRows[i]
		}
		if i < len(typeRows) {
			right = typeRows[i]
		}
		leftPad := lipgloss.NewStyle().Width(colWidth).Render(left)
		rightPad := lipgloss.NewStyle().Width(colWidth).Render(right)
		sb.WriteString(leftPad + sep + rightPad + "\n")
	}

	// Trim final newline.
	result := sb.String()
	result = strings.TrimRight(result, "\n")
	return result
}
