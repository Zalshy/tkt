package help

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/tui/keys"
	"github.com/zalshy/tkt/internal/tui/styles"
)

// Render returns a pre-rendered help overlay string sized to width.
func Render(width int) string {
	boxWidth := width - 4
	if boxWidth > 60 {
		boxWidth = 60
	}
	if boxWidth < 0 {
		boxWidth = 0
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(styles.Primary).
		Bold(true)

	keyStyle := lipgloss.NewStyle().
		Foreground(styles.Secondary)

	descStyle := lipgloss.NewStyle().
		Foreground(styles.Muted)

	var body string
	for i, sec := range keys.All() {
		if i > 0 {
			body += "\n"
		}
		body += headerStyle.Render(sec.Title) + "\n"
		for _, h := range sec.Bindings {
			body += "  " + keyStyle.Render(h.Key) + "  " + descStyle.Render(h.Desc) + "\n"
		}
	}

	boxStyle := lipgloss.NewStyle().
		Background(styles.BgSurf).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Accent).
		Padding(1, 2).
		Width(boxWidth)

	return boxStyle.Render(body)
}
