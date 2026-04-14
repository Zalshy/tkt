package output

import (
	"encoding/json"
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

// usageEntry holds a parsed usage log entry for display.
type usageEntry struct {
	SessionID  string
	Tokens     int
	Tools      int
	DurationMs int
	Agent      string
	Label      string
	CreatedAt  time.Time
}

// parseUsageBody parses the JSON body of a usage log entry.
// Returns zero-value usageEntry on parse failure (graceful degradation).
func parseUsageBody(e models.LogEntry) usageEntry {
	u := usageEntry{
		SessionID: e.SessionID,
		CreatedAt: e.CreatedAt,
	}
	var raw struct {
		Tokens     int    `json:"tokens"`
		Tools      int    `json:"tools"`
		DurationMs int    `json:"duration_ms"`
		Agent      string `json:"agent"`
		Label      string `json:"label"`
	}
	if err := json.Unmarshal([]byte(e.Body), &raw); err == nil {
		u.Tokens = raw.Tokens
		u.Tools = raw.Tools
		u.DurationMs = raw.DurationMs
		u.Agent = raw.Agent
		u.Label = raw.Label
	}
	return u
}

// formatIntComma formats an integer with thousands separators (e.g. 78155 → "78,155").
func formatIntComma(n int) string {
	s := fmt.Sprintf("%d", n)
	neg := n < 0
	if neg {
		s = s[1:]
	}
	result := []byte{}
	for i, c := range []byte(s) {
		pos := len(s) - i
		if i > 0 && pos%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, c)
	}
	if neg {
		return "-" + string(result)
	}
	return string(result)
}

// RenderTicket renders a single ticket in chat-view format.
func RenderTicket(t models.Ticket, entries []models.LogEntry) string {
	var b strings.Builder

	// Phase 1 — Extract usage entries BEFORE the main loop.
	// Usage entries must not fall through to the default branch (which renders raw JSON as message rows).
	var usageEntries []usageEntry
	var nonUsageEntries []models.LogEntry
	for _, e := range entries {
		if e.Kind == "usage" {
			usageEntries = append(usageEntries, parseUsageBody(e))
		} else {
			nonUsageEntries = append(nonUsageEntries, e)
		}
	}

	// Phase 2 — Column alignment pre-pass (uses non-usage entries only).
	allIDs := []string{t.CreatedBy}
	for _, e := range nonUsageEntries {
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

	// Phase 3 — Header
	b.WriteString(separator + "\n")
	b.WriteString(fmt.Sprintf("#%d  ·  %s\n", t.ID, ColorStatus(t.Status)))
	b.WriteString(t.Title + "\n")
	if t.Tier != "" && t.Tier != "standard" {
		b.WriteString(fmt.Sprintf("Tier:   %s\n", t.Tier))
	}
	b.WriteString(separator + "\n")
	b.WriteString("\n")

	// Phase 4 — Synthetic "created" entry
	b.WriteString(padID(t.CreatedBy) + "    ○ created\n")
	if t.Description != "" {
		for _, line := range strings.Split(t.Description, "\n") {
			b.WriteString(indent + line + "\n")
		}
	}
	b.WriteString("\n")

	// Phase 4 continued — entries loop (non-usage only)
	for _, e := range nonUsageEntries {
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
			// "message" and any other non-usage kind
			lines := strings.Split(e.Body, "\n")
			b.WriteString(padID(e.SessionID) + "    " + lines[0] + "\n")
			for _, line := range lines[1:] {
				b.WriteString(indent + line + "\n")
			}
		}
		b.WriteString("\n")
	}

	// Phase 5 — Token usage section (only if usage entries exist)
	if len(usageEntries) > 0 {
		b.WriteString(separator + "\n")
		b.WriteString("Token usage:\n")

		// Column widths for alignment.
		maxSessW := 0
		maxAgentW := 0
		for _, u := range usageEntries {
			if len(u.SessionID) > maxSessW {
				maxSessW = len(u.SessionID)
			}
			if len(u.Agent) > maxAgentW {
				maxAgentW = len(u.Agent)
			}
		}

		totalTokens := 0
		for _, u := range usageEntries {
			totalTokens += u.Tokens

			sessCol := u.SessionID + strings.Repeat(" ", maxSessW-len(u.SessionID))
			agentCol := u.Agent + strings.Repeat(" ", maxAgentW-len(u.Agent))

			row := fmt.Sprintf("  %s    %s    %s tokens", sessCol, agentCol, formatIntComma(u.Tokens))
			if u.Tools > 0 {
				row += fmt.Sprintf("  %s tools", formatIntComma(u.Tools))
			}
			if u.DurationMs > 0 {
				secs := u.DurationMs / 1000
				row += fmt.Sprintf("  %ds", secs)
			}
			row += "    " + u.CreatedAt.Format("2006-01-02")
			if u.Label != "" {
				row += "    " + u.Label
			}
			b.WriteString(row + "\n")
		}

		b.WriteString(fmt.Sprintf("Total: %s tokens across %d %s\n",
			formatIntComma(totalTokens), len(usageEntries),
			pluralEntries(len(usageEntries))))
	}

	// Phase 6 — Footer
	b.WriteString(separator + "\n")

	// Distinct session count (uses all entries for accurate count)
	seen := map[string]struct{}{}
	if t.CreatedBy != "" {
		seen[t.CreatedBy] = struct{}{}
	}
	for _, e := range entries {
		seen[e.SessionID] = struct{}{}
	}

	// Last activity time (uses all entries)
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

func pluralEntries(n int) string {
	if n == 1 {
		return "entry"
	}
	return "entries"
}
