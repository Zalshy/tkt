package side

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/tui/styles"
)

// feedEntry represents a single ticket transition from the audit log.
type feedEntry struct {
	ticketID    int64
	sessionName string
	toState     string
	createdAt   time.Time
	arrivedAt   time.Time // set when first detected as new on a poll cycle; zero for pre-existing
}

// loadFeed queries the most recent 15 ticket transitions across all tickets.
// Returns an empty slice (not an error) when db is nil.
func loadFeed(db *sql.DB) ([]feedEntry, error) {
	if db == nil {
		return nil, nil
	}

	rows, err := db.Query(`
		SELECT tl.ticket_id, tl.session_name, tl.to_state, tl.created_at
		FROM ticket_log tl
		WHERE tl.kind = 'transition' AND tl.deleted_at IS NULL
		ORDER BY tl.created_at DESC
		LIMIT 15
	`)
	if err != nil {
		return nil, fmt.Errorf("feed.loadFeed: query: %w", err)
	}
	defer rows.Close()

	var entries []feedEntry
	for rows.Next() {
		var e feedEntry
		var toState sql.NullString
		if err := rows.Scan(&e.ticketID, &e.sessionName, &toState, &e.createdAt); err != nil {
			return nil, fmt.Errorf("feed.loadFeed: scan: %w", err)
		}
		if toState.Valid {
			e.toState = toState.String
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("feed.loadFeed: rows: %w", err)
	}
	return entries, nil
}

// relAge formats a time as a human-readable relative age string.
//   - < 60s  → "Xs"
//   - < 3600s → "Xm"
//   - < 86400s → "Xh"
//   - else   → "Xd"
func relAge(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	secs := int(d.Seconds())
	switch {
	case secs < 60:
		return fmt.Sprintf("%ds", secs)
	case secs < 3600:
		return fmt.Sprintf("%dm", secs/60)
	case secs < 86400:
		return fmt.Sprintf("%dh", secs/3600)
	default:
		return fmt.Sprintf("%dd", secs/86400)
	}
}

// renderFeed renders the TICKET CHANGES section.
func renderFeed(entries []feedEntry, width int) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(styles.Primary).
		Bold(true)

	var sb strings.Builder
	sb.WriteString(headerStyle.Render("TICKET CHANGES"))
	sb.WriteString("\n")

	if len(entries) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(styles.Faint).Render("  (none)"))
		return sb.String()
	}

	highlightStyle := lipgloss.NewStyle().
		Background(styles.Warning).
		Foreground(styles.BgDeep)
	normalStyle := lipgloss.NewStyle().Foreground(styles.Secondary)

	for _, e := range entries {
		line := fmt.Sprintf("  %s · #%d → %s    %s",
			e.sessionName,
			e.ticketID,
			e.toState,
			relAge(e.createdAt),
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

	return sb.String()
}
