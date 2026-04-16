package output

import (
	"fmt"
	"strings"

	"github.com/zalshy/tkt/internal/models"
)

// RenderContext renders a single context entry in the §7 format:
//
//	─────────────────────────────────────────────
//	Context #N  ·  <title>
//	<session_id>  ·  <date>
//
//	<body>
func RenderContext(c models.Context) string {
	var b strings.Builder
	b.WriteString(separator + "\n")
	b.WriteString(fmt.Sprintf("Context #%d  ·  %s\n", c.ID, c.Title))
	b.WriteString(fmt.Sprintf("%s  ·  %s\n", c.CreatedBy, c.CreatedAt.Format("2006-01-02 15:04")))
	b.WriteString("\n")
	b.WriteString(c.Body + "\n")
	b.WriteString("\n")
	return b.String()
}

// RenderContextList renders all context entries followed by a count footer.
// When entries is empty, returns "(no context entries)\n" immediately — no footer.
func RenderContextList(entries []models.Context) string {
	if len(entries) == 0 {
		return "(no context entries)\n"
	}
	var b strings.Builder
	for _, c := range entries {
		b.WriteString(RenderContext(c))
	}
	n := len(entries)
	if n == 1 {
		b.WriteString("1 context entry.\n")
	} else {
		b.WriteString(fmt.Sprintf("%d context entries.\n", n))
	}
	return b.String()
}
