package side

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/tui/styles"
)

// sessionEvent represents an active session entry.
type sessionEvent struct {
	name      string
	role      string
	startedAt time.Time
	arrivedAt time.Time // set when first detected as new on a poll cycle; zero for pre-existing
}

// loadSessions queries the 5 most recent active sessions (excluding monitor).
// Returns empty results (not an error) when db is nil.
func loadSessions(db *sql.DB) ([]sessionEvent, error) {
	if db == nil {
		return nil, nil
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
		return nil, fmt.Errorf("sessions.loadSessions: query: %w", err)
	}
	defer rows.Close()

	var events []sessionEvent
	for rows.Next() {
		var e sessionEvent
		if err := rows.Scan(&e.name, &e.role, &e.startedAt); err != nil {
			return nil, fmt.Errorf("sessions.loadSessions: scan: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sessions.loadSessions: rows: %w", err)
	}
	return events, nil
}

// renderSessions renders the SESSIONS section.
//
// maxVisible controls how many session rows are shown:
//   - 0  → compact mode: title + role counts only (no divider, no rows)
//   - >0 → full mode: title + counts + divider + up to maxVisible rows
//
// Counts always reflect ALL events regardless of maxVisible, so the numbers
// stay accurate even when rows are hidden or capped.
//
// Layout — full mode:
//
//	       SESSIONS          ← centered title
//	  architect   N          ← counts from ALL events
//	  implementer N
//	  ─────────────────      ← divider
//	  alice-arch  arch  14:28
//	  bob         impl  09:55
//
// Layout — compact mode (maxVisible == 0):
//
//	       SESSIONS
//	  architect   N
//	  implementer N
func renderSessions(events []sessionEvent, width, maxVisible int) string {
	compact := maxVisible <= 0

	// — Counts: always from ALL events, before any display cap —
	var archC, implC int
	for _, e := range events {
		switch e.role {
		case "architect":
			archC++
		case "implementer":
			implC++
		}
	}

	// Cap the display slice for full mode.
	displayEvents := events
	if !compact && len(displayEvents) > maxVisible {
		displayEvents = displayEvents[:maxVisible]
	}

	var sb strings.Builder

	// — Section header — centered —
	sb.WriteString(lipgloss.NewStyle().
		Foreground(styles.Primary).
		Bold(true).
		Width(width).
		Align(lipgloss.Center).
		Render("SESSIONS"))
	sb.WriteString("\n")

	archBadge := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#C678DD")).
		Bold(true).
		Render("architect")
	archCount := lipgloss.NewStyle().
		Foreground(styles.Primary).
		Bold(true).
		Render(fmt.Sprintf("  %d", archC))

	implBadge := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#56B6C2")).
		Bold(true).
		Render("implementer")
	implCount := lipgloss.NewStyle().
		Foreground(styles.Primary).
		Bold(true).
		Render(fmt.Sprintf("  %d", implC))

	sb.WriteString(archBadge + archCount)
	sb.WriteString("\n")
	sb.WriteString(implBadge + implCount)
	sb.WriteString("\n")

	// Compact mode: counts only — stop here.
	if compact {
		return sb.String()
	}

	// — Divider —
	divW := max(width, 1)
	sb.WriteString(lipgloss.NewStyle().
		Foreground(styles.Faint).
		Render(strings.Repeat("─", divW)))
	sb.WriteString("\n")

	// — Session rows — dynamic nameW so each row fills the available width —
	if len(displayEvents) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(styles.Faint).Render("(none)"))
		return sb.String()
	}

	// Layout: name(nameW) + role(5) + time(5) = width
	const roleColW = 5 // "arch " or "impl " (4 + 1 space)
	const timeColW = 5 // "15:04"
	nameW := width - roleColW - timeColW
	if nameW < 8 {
		nameW = 8
	}

	highlightStyle := lipgloss.NewStyle().
		Background(styles.Warning).
		Foreground(styles.BgDeep)

	for _, e := range displayEvents {
		isNew := !e.arrivedAt.IsZero() && time.Since(e.arrivedAt) < 1500*time.Millisecond

		name := e.name
		if len(name) > nameW {
			name = name[:nameW-1] + "…"
		}

		if isNew {
			line := fmt.Sprintf("%-*s %-4s %s",
				nameW, name, roleAbbrev(e.role), e.startedAt.Format("15:04"))
			sb.WriteString(highlightStyle.Render(line))
		} else {
			roleLabel, roleColor := roleStyle(e.role)
			sb.WriteString(lipgloss.NewStyle().
				Foreground(sessionColor(e.name)).
				Render(fmt.Sprintf("%-*s", nameW, name)))
			sb.WriteString(lipgloss.NewStyle().
				Foreground(roleColor).Bold(true).
				Render(fmt.Sprintf("%-*s", roleColW, roleLabel)))
			sb.WriteString(lipgloss.NewStyle().
				Foreground(styles.Secondary).
				Render(e.startedAt.Format("15:04")))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// roleAbbrev returns the short label for a base role.
func roleAbbrev(role string) string {
	switch role {
	case "architect":
		return "arch"
	case "implementer":
		return "impl"
	default:
		return role
	}
}

// roleStyle returns the short label and brand colour for a base role.
func roleStyle(role string) (string, lipgloss.Color) {
	switch role {
	case "architect":
		return "arch", lipgloss.Color("#C678DD")
	case "implementer":
		return "impl", lipgloss.Color("#56B6C2")
	default:
		return role, styles.Muted
	}
}
