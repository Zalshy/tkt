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
	isCreate    bool      // true for ticket-created events sourced from tickets table
	isForced    bool      // true when the transition was a forced op
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
		SELECT ticket_id, session_name, to_state, 0 AS is_create, forced, created_at
		FROM ticket_log
		WHERE kind = 'transition' AND deleted_at IS NULL

		UNION ALL

		SELECT id, created_by, 'TODO', 1 AS is_create, 0 AS forced, created_at
		FROM tickets
		WHERE deleted_at IS NULL

		ORDER BY created_at DESC
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
		var isCreate, forcedVal int
		if err := rows.Scan(&e.ticketID, &e.sessionName, &toState, &isCreate, &forcedVal, &e.createdAt); err != nil {
			return nil, fmt.Errorf("feed.loadFeed: scan: %w", err)
		}
		if toState.Valid {
			e.toState = toState.String
		}
		e.isCreate = isCreate != 0
		e.isForced = forcedVal != 0
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("feed.loadFeed: rows: %w", err)
	}
	return entries, nil
}

// relAge formats a time as a human-readable relative age string.
//   - < 60s   → "just now"
//   - < 3600s  → "Xm ago"
//   - < 86400s → "Xh ago"
//   - else     → "Xd ago"
func relAge(t time.Time) string {
	d := max(time.Since(t), 0)
	secs := int(d.Seconds())
	switch {
	case secs < 60:
		return "just now"
	case secs < 3600:
		return fmt.Sprintf("%dm ago", secs/60)
	case secs < 86400:
		return fmt.Sprintf("%dh ago", secs/3600)
	default:
		return fmt.Sprintf("%dd ago", secs/86400)
	}
}

// renderFeed renders the TICKET ACTIVITY section.
// maxEntries caps how many rows are shown so the section fits the available height.
// Pass 0 or negative to show all entries.
func renderFeed(entries []feedEntry, width int, maxEntries int) string {
	var sb strings.Builder

	// — Section header — centered —
	sb.WriteString(lipgloss.NewStyle().
		Foreground(styles.Primary).
		Bold(true).
		Width(width).
		Align(lipgloss.Center).
		Render("TICKET ACTIVITY"))
	sb.WriteString("\n")

	if len(entries) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(styles.Faint).Render("  (none)"))
		return sb.String()
	}

	// Cap entries to maxEntries if set.
	if maxEntries > 0 && len(entries) > maxEntries {
		entries = entries[:maxEntries]
	}

	highlightStyle := lipgloss.NewStyle().
		Background(styles.Warning).
		Foreground(styles.BgDeep)
	restStyle := lipgloss.NewStyle().Foreground(styles.Secondary)
	warnStyle := lipgloss.NewStyle().Foreground(styles.Warning)

	// Column widths — sessionW is computed so every row fills `width` exactly.
	// Layout: session(sessionW) + " · "(3) + ticket(6) + " → "(3) + state(12) + " "(1) + age(8)
	// ageW=8 fits "just now" (8 chars) and "999d ago" (7 chars) without truncation.
	const stateW = 12
	const ageW = 8
	const fixedW = 3 + 6 + 3 + stateW + 1 + ageW // = 33
	sessionW := max(width-fixedW, 8)

	for _, e := range entries {
		isNew := !e.arrivedAt.IsZero() && time.Since(e.arrivedAt) < 1500*time.Millisecond

		session := e.sessionName
		if len(session) > sessionW {
			session = session[:sessionW-1] + "…"
		}

		ticket := fmt.Sprintf("#%d", e.ticketID)

		var state string
		if e.isCreate {
			state = "created"
		} else {
			state = strings.ToLower(e.toState)
		}
		if len(state) > stateW {
			state = state[:stateW]
		}

		age := relAge(e.createdAt)

		rest := fmt.Sprintf(" · %-6s → %-*s %*s", ticket, stateW, state, ageW, age)

		if isNew {
			plain := fmt.Sprintf("%-*s%s", sessionW, session, rest)
			sb.WriteString(highlightStyle.Render(plain))
		} else if e.isForced {
			// Session name in its normal per-session colour; everything after in warning.
			// rest starts with " " — replace that leading space with "⚠" so the symbol
			// sits flush after the session name without changing total row width.
			sb.WriteString(lipgloss.NewStyle().
				Foreground(sessionColor(e.sessionName)).
				Render(fmt.Sprintf("%-*s", sessionW, session)))
			sb.WriteString(warnStyle.Render("⚠" + rest[1:]))
		} else {
			sb.WriteString(lipgloss.NewStyle().
				Foreground(sessionColor(e.sessionName)).
				Render(fmt.Sprintf("%-*s", sessionW, session)))
			sb.WriteString(restStyle.Render(rest))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
