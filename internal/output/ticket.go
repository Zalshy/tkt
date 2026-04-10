package output

import (
	"fmt"
	"strings"
	"time"

	"github.com/zalshy/tkt/internal/models"
)

// separator is 45 U+2500 BOX DRAWINGS LIGHT HORIZONTAL characters.
const separator = "─────────────────────────────────────────────"

// formatRelativeTime returns a human-readable relative time string.
func formatRelativeTime(t time.Time) string {
	delta := time.Since(t)
	if delta >= time.Hour {
		return fmt.Sprintf("%dh ago", int(delta.Hours()))
	}
	if delta >= time.Minute {
		return fmt.Sprintf("%dm ago", int(delta.Minutes()))
	}
	return "just now"
}

// RenderDependencies renders the dependencies section for tkt show.
// Returns empty string if deps is empty (caller omits the section entirely).
func RenderDependencies(deps []models.Ticket) string {
	if len(deps) == 0 {
		return ""
	}

	var b strings.Builder

	b.WriteString(separator + "\n")
	b.WriteString("Dependencies:\n")

	blocked := 0
	for _, d := range deps {
		if d.Status == models.StatusVerified {
			b.WriteString(fmt.Sprintf("  %s #%-4d %s\n", Colorize("✓", Green), d.ID, ColorStatus(d.Status)))
		} else {
			b.WriteString(fmt.Sprintf("  %s #%-4d %s    ← blocking\n", Colorize("○", Gray), d.ID, ColorStatus(d.Status)))
			blocked++
		}
	}

	b.WriteString("\n")
	if blocked == 0 {
		b.WriteString("All dependencies resolved.\n")
	} else {
		b.WriteString(fmt.Sprintf("Blocked by %d unresolved dependencies.\n", blocked))
	}
	b.WriteString(separator + "\n")

	return b.String()
}

// RenderTicket renders a single ticket in chat-view format.
func RenderTicket(t models.Ticket, entries []models.LogEntry) string {
	var b strings.Builder

	// Phase 1 — Column alignment pre-pass
	allIDs := []string{t.CreatedBy}
	for _, e := range entries {
		allIDs = append(allIDs, e.SessionID)
	}
	maxWidth := 0
	for _, id := range allIDs {
		if len(id) > maxWidth {
			maxWidth = len(id)
		}
	}
	indent := strings.Repeat(" ", maxWidth+4)

	padID := func(id string) string {
		return id + strings.Repeat(" ", maxWidth-len(id))
	}

	// Phase 2 — Header
	b.WriteString(separator + "\n")
	b.WriteString(fmt.Sprintf("#%d  ·  %s\n", t.ID, ColorStatus(t.Status)))
	b.WriteString(t.Title + "\n")
	if t.Tier != "" && t.Tier != "standard" {
		b.WriteString(fmt.Sprintf("Tier:   %s\n", t.Tier))
	}
	b.WriteString(separator + "\n")
	b.WriteString("\n")

	// Phase 3 — Synthetic "created" entry
	b.WriteString(padID(t.CreatedBy) + "    ○ created\n")
	if t.Description != "" {
		for _, line := range strings.Split(t.Description, "\n") {
			b.WriteString(indent + line + "\n")
		}
	}
	b.WriteString("\n")

	// Phase 3 continued — entries loop
	for _, e := range entries {
		switch e.Kind {
		case "transition":
			fromStr := "?"
			if e.FromState != nil {
				fromStr = ColorStatus(*e.FromState)
			}
			toStr := "?"
			if e.ToState != nil {
				toStr = ColorStatus(*e.ToState)
			}
			b.WriteString(padID(e.SessionID) + "    ↳ " + fromStr + " → " + toStr + "\n")
			if e.Body != "" {
				for _, line := range strings.Split(e.Body, "\n") {
					b.WriteString(indent + line + "\n")
				}
			}
		case "plan":
			b.WriteString(padID(e.SessionID) + "    [plan]\n")
			if e.Body != "" {
				for _, line := range strings.Split(e.Body, "\n") {
					b.WriteString(indent + line + "\n")
				}
			}
		default:
			// "message" and any other kind
			lines := strings.Split(e.Body, "\n")
			b.WriteString(padID(e.SessionID) + "    " + lines[0] + "\n")
			for _, line := range lines[1:] {
				b.WriteString(indent + line + "\n")
			}
		}
		b.WriteString("\n")
	}

	// Phase 4 — Footer
	b.WriteString(separator + "\n")

	// Distinct session count
	seen := map[string]struct{}{}
	if t.CreatedBy != "" {
		seen[t.CreatedBy] = struct{}{}
	}
	for _, e := range entries {
		seen[e.SessionID] = struct{}{}
	}

	// Last activity time
	var lastTime time.Time
	if len(entries) > 0 {
		lastTime = entries[len(entries)-1].CreatedAt
	} else {
		lastTime = t.CreatedAt
	}

	b.WriteString(fmt.Sprintf("%d sessions · %d entries · last activity %s\n",
		len(seen), len(entries), formatRelativeTime(lastTime)))

	return b.String()
}
