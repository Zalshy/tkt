package output

import (
	"fmt"
	"strings"

	"github.com/zalshy/tkt/internal/models"
)

// RenderList renders a slice of tickets as the aligned list format from §7:
//
//	#42  Add OAuth login
//	#17  Fix signup validation
//	... more tickets. Use --limit 50 or --all to see them.
//
// When hasMore is true, a hint line is appended. total is the remaining count;
// when total <= 0 the hint line omits the count (the caller may not have it).
func RenderList(tickets []models.Ticket, hasMore bool, total int) string {
	if len(tickets) == 0 {
		return "(no tickets)"
	}

	// Compute the maximum #<id> width so titles align across all rows.
	maxIDWidth := 0
	for _, t := range tickets {
		w := len(fmt.Sprintf("#%d", t.ID))
		if w > maxIDWidth {
			maxIDWidth = w
		}
	}

	var b strings.Builder
	for i, t := range tickets {
		idStr := fmt.Sprintf("#%d", t.ID)
		// %-*s right-pads idStr to maxIDWidth characters.
		line := fmt.Sprintf("%-*s  %s", maxIDWidth, idStr, t.Title)
		if t.Tier != "" && t.Tier != "standard" {
			line += fmt.Sprintf("  [%s]", t.Tier)
		}
		b.WriteString(line)
		if i < len(tickets)-1 {
			b.WriteByte('\n')
		}
	}

	if hasMore {
		b.WriteByte('\n')
		if total > 0 {
			b.WriteString(fmt.Sprintf("... %d more tickets. Use --limit 50 or --all to see them.", total))
		} else {
			b.WriteString("... more tickets. Use --limit 50 or --all to see them.")
		}
	}

	return b.String()
}
