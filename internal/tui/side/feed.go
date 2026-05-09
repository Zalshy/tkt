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
		Render("TICKET CHANGES"))
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
	markerStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
	restStyle := lipgloss.NewStyle().Foreground(styles.Secondary)

	// Column widths — sessionW is computed so every row fills `width` exactly.
	// Layout: marker(2) + session(sessionW) + " · "(3) + ticket(6) + " → "(3) + state(12) + " "(1) + age(4)
	const stateW = 12
	const ageW = 4
	const fixedW = 2 + 3 + 6 + 3 + stateW + 1 + ageW // = 31
	sessionW := width - fixedW
	if sessionW < 8 {
		sessionW = 8
	}

	for i, e := range entries {
		isFirst := i == 0
		isNew := !e.arrivedAt.IsZero() && time.Since(e.arrivedAt) < 1500*time.Millisecond

		session := e.sessionName
		if len(session) > sessionW {
			session = session[:sessionW-1] + "…"
		}

		ticket := fmt.Sprintf("#%d", e.ticketID)

		state := e.toState
		if len(state) > stateW {
			state = state[:stateW]
		}

		age := relAge(e.createdAt)

		rest := fmt.Sprintf(" · %-6s → %-*s %*s", ticket, stateW, state, ageW, age)

		if isNew {
			marker := "  "
			if isFirst {
				marker = "▶ "
			}
			plain := fmt.Sprintf("%s%-*s%s", marker, sessionW, session, rest)
			sb.WriteString(highlightStyle.Render(plain))
		} else {
			if isFirst {
				sb.WriteString(markerStyle.Render("▶ "))
			} else {
				sb.WriteString("  ")
			}
			sb.WriteString(lipgloss.NewStyle().
				Foreground(sessionColor(e.sessionName)).
				Render(fmt.Sprintf("%-*s", sessionW, session)))
			sb.WriteString(restStyle.Render(rest))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
