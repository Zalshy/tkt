package side

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/tui/styles"
)

// forcedOpsEntry represents a single forced transition from ticket_log.
type forcedOpsEntry struct {
	ticketID    int64
	sessionName string
	fromState   string
	toState     string
	createdAt   time.Time
}

// loadForcedOps queries the 8 most recent forced transitions.
func loadForcedOps(db *sql.DB) ([]forcedOpsEntry, error) {
	if db == nil {
		return nil, nil
	}

	rows, err := db.Query(`
		SELECT ticket_id, session_name, from_state, to_state, created_at
		FROM ticket_log
		WHERE kind = 'transition' AND forced = 1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT 8
	`)
	if err != nil {
		return nil, fmt.Errorf("forcedops.loadForcedOps: query: %w", err)
	}
	defer rows.Close()

	var entries []forcedOpsEntry
	for rows.Next() {
		var e forcedOpsEntry
		var fromState, toState sql.NullString
		if err := rows.Scan(&e.ticketID, &e.sessionName, &fromState, &toState, &e.createdAt); err != nil {
			return nil, fmt.Errorf("forcedops.loadForcedOps: scan: %w", err)
		}
		if fromState.Valid {
			e.fromState = fromState.String
		}
		if toState.Valid {
			e.toState = toState.String
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("forcedops.loadForcedOps: rows: %w", err)
	}
	return entries, nil
}

// renderForcedOps renders the FORCED OPS section.
// Each row: session · #ticket FROM→TO  age
func renderForcedOps(entries []forcedOpsEntry, width int) string {
	var sb strings.Builder

	// — Section header — centered —
	sb.WriteString(lipgloss.NewStyle().
		Foreground(styles.Primary).
		Bold(true).
		Width(width).
		Align(lipgloss.Center).
		Render("FORCED OPS"))
	sb.WriteString("\n")

	if len(entries) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(styles.Faint).Render("  (none)"))
		return sb.String()
	}

	warnStyle := lipgloss.NewStyle().Foreground(styles.Warning)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	for _, e := range entries {
		// Compact layout: session(clipped) · #N FROM→TO  age
		ticket := fmt.Sprintf("#%d", e.ticketID)
		from := strings.ToLower(e.fromState)
		to := strings.ToLower(e.toState)
		age := relAge(e.createdAt)
		arrow := fmt.Sprintf(" · %s %s→%s  %s", ticket, from, to, age)

		sessionW := width - lipgloss.Width(arrow)
		if sessionW < 1 {
			sessionW = 1
		}
		session := e.sessionName
		if len(session) > sessionW {
			session = session[:sessionW-1] + "…"
		}

		sb.WriteString(warnStyle.Render(fmt.Sprintf("%-*s", sessionW, session)))
		sb.WriteString(mutedStyle.Render(arrow))
		sb.WriteString("\n")
	}

	return sb.String()
}
