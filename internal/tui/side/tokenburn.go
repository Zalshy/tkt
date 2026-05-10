package side

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/tui/styles"
)

// tokenBurnData holds aggregated token totals filtered to active tickets.
type tokenBurnData struct {
	total int64
	arch  int64
	impl  int64
}

// loadTokenBurn sums tokens from ticket_usage, joining to tickets so that
// CANCELED and ARCHIVED tickets are excluded.
func loadTokenBurn(db *sql.DB) (tokenBurnData, error) {
	if db == nil {
		return tokenBurnData{}, nil
	}

	row := db.QueryRow(`
		SELECT
			COALESCE(SUM(tu.tokens), 0),
			COALESCE(SUM(CASE WHEN tu.agent LIKE '%architect%' THEN tu.tokens ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN tu.agent LIKE '%implementer%' THEN tu.tokens ELSE 0 END), 0)
		FROM ticket_usage tu
		JOIN tickets t ON t.id = tu.ticket_id
		WHERE tu.deleted_at IS NULL
		  AND t.status NOT IN ('CANCELED', 'ARCHIVED')
		  AND t.deleted_at IS NULL
	`)

	var d tokenBurnData
	if err := row.Scan(&d.total, &d.arch, &d.impl); err != nil {
		return tokenBurnData{}, fmt.Errorf("tokenburn.loadTokenBurn: %w", err)
	}
	return d, nil
}

// fmtTokens formats a token count as a compact human-readable string.
// Returns "n/a" when n is zero (no data recorded yet).
func fmtTokens(n int64) string {
	switch {
	case n == 0:
		return "n/a"
	case n >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// renderTokenBurn renders the TOKEN BURN section.
//
// compact=true shows only the title and total line (for constrained heights).
// compact=false shows the full breakdown: total, arch, impl.
//
// Layout — full:
//
//	     TOKEN BURN          ← centered title
//	  total       1.84M
//	  architect   1.21M
//	  implementer 0.63M
//
// Layout — compact:
//
//	     TOKEN BURN
//	  total       1.84M
func renderTokenBurn(d tokenBurnData, width int, compact bool) string {
	var sb strings.Builder

	// — Section header — centered —
	sb.WriteString(lipgloss.NewStyle().
		Foreground(styles.Primary).
		Bold(true).
		Width(width).
		Align(lipgloss.Center).
		Render("TOKEN BURN"))
	sb.WriteString("\n")

	// renderRow writes "label <pad> value\n" with the value right-aligned.
	renderRow := func(label, value string, color lipgloss.Color) {
		labelStr := lipgloss.NewStyle().Foreground(color).Bold(true).Render(label)
		valStr := lipgloss.NewStyle().Foreground(color).Bold(true).Render(value)
		pad := width - lipgloss.Width(labelStr) - lipgloss.Width(valStr)
		if pad < 1 {
			pad = 1
		}
		sb.WriteString(labelStr)
		sb.WriteString(strings.Repeat(" ", pad))
		sb.WriteString(valStr)
		sb.WriteString("\n")
	}

	renderRow("total", fmtTokens(d.total), styles.Primary)
	if !compact {
		renderRow("arch", fmtTokens(d.arch), lipgloss.Color("#C678DD"))
		renderRow("impl", fmtTokens(d.impl), lipgloss.Color("#56B6C2"))
	}

	return sb.String()
}
