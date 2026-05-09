package side

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/session"
	"github.com/zalshy/tkt/internal/tui/styles"
)

// sessionEvent represents an active session entry.
type sessionEvent struct {
	name      string
	role      string
	startedAt time.Time
	arrivedAt time.Time // set when first detected as new on a poll cycle; zero for pre-existing
}

// sessionCounts holds the count of active sessions by base role.
type sessionCounts struct {
	arch int
	impl int
}

// loadSessions queries recent active sessions (excluding monitor) and returns
// the list along with arch/impl counts. Returns empty results (not an error)
// when db is nil.
func loadSessions(db *sql.DB) ([]sessionEvent, sessionCounts, error) {
	if db == nil {
		return nil, sessionCounts{}, nil
	}

	rows, err := db.Query(`
		SELECT s.name, r.base_role, s.created_at
		FROM sessions s
		JOIN roles r ON r.name = s.role
		WHERE s.expired_at IS NULL
		  AND r.base_role != 'monitor'
		ORDER BY s.created_at DESC
		LIMIT 5
	`)
	if err != nil {
		return nil, sessionCounts{}, fmt.Errorf("sessions.loadSessions: query: %w", err)
	}
	defer rows.Close()

	var events []sessionEvent
	for rows.Next() {
		var e sessionEvent
		if err := rows.Scan(&e.name, &e.role, &e.startedAt); err != nil {
			return nil, sessionCounts{}, fmt.Errorf("sessions.loadSessions: scan: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, sessionCounts{}, fmt.Errorf("sessions.loadSessions: rows: %w", err)
	}

	countMap, err := session.CountActive(db)
	if err != nil {
		// Non-fatal: return the events with zero counts.
		return events, sessionCounts{}, nil
	}

	counts := sessionCounts{
		arch: countMap[models.RoleArchitect],
		impl: countMap[models.RoleImplementer],
	}

	return events, counts, nil
}

// renderSessions renders the SESSIONS section.
func renderSessions(events []sessionEvent, counts sessionCounts, width int) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(styles.Primary).
		Bold(true)

	var sb strings.Builder
	sb.WriteString(headerStyle.Render("SESSIONS"))
	sb.WriteString("\n")

	if len(events) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(styles.Faint).Render("  (none)"))
		sb.WriteString("\n")
	} else {
		highlightStyle := lipgloss.NewStyle().
			Background(styles.Warning).
			Foreground(styles.BgDeep)
		normalStyle := lipgloss.NewStyle().Foreground(styles.Secondary)

		for _, e := range events {
			line := fmt.Sprintf("  %s    started   %s",
				e.name,
				e.startedAt.Format("15:04"),
			)
			// Truncate to width if needed.
			if width > 0 && len(line) > width {
				line = line[:width]
			}
			isNew := !e.arrivedAt.IsZero() && time.Since(e.arrivedAt) < 1500*time.Millisecond
			if isNew {
				sb.WriteString(highlightStyle.Render(line))
			} else {
				sb.WriteString(normalStyle.Render(line))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	countsLine := fmt.Sprintf("  arch: %d   impl: %d", counts.arch, counts.impl)
	sb.WriteString(lipgloss.NewStyle().Foreground(styles.Muted).Render(countsLine))

	return sb.String()
}
