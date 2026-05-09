package side

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/tui/styles"
)

// clockTickMsg is sent when the next wall-clock minute boundary arrives.
type clockTickMsg struct{}

// clockModel holds the current time for the header clock.
type clockModel struct {
	now time.Time
}

// newClockModel returns a clockModel initialised to the current time.
func newClockModel() clockModel {
	return clockModel{now: time.Now()}
}

// clockCmd returns a tea.Cmd that fires at the next wall-clock minute boundary.
// It computes the exact duration until the next minute so the clock is always
// pinned to real minute transitions rather than drifting over time.
func clockCmd() tea.Cmd {
	next := time.Now().Truncate(time.Minute).Add(time.Minute)
	return tea.Tick(time.Until(next), func(time.Time) tea.Msg {
		return clockTickMsg{}
	})
}

// update advances the clock on a clockTickMsg and reschedules the next tick.
// All other messages are ignored.
func (c clockModel) update(msg tea.Msg) (clockModel, tea.Cmd) {
	if _, ok := msg.(clockTickMsg); ok {
		c.now = time.Now()
		return c, clockCmd()
	}
	return c, nil
}

// renderHeader renders the single-line side-monitor header.
// Left side: "  tkt side" in styles.Primary, bold.
// Right side: current time formatted as "HH:MM" in styles.Muted.
// The two pieces are joined to fill exactly width columns.
func renderHeader(c clockModel, width int) string {
	left := lipgloss.NewStyle().
		Foreground(styles.Primary).
		Bold(true).
		Render("  tkt side")

	right := lipgloss.NewStyle().
		Foreground(styles.Muted).
		Render(c.now.Format("15:04"))

	// Calculate the number of spaces needed to push the clock to the right edge.
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	spacer := width - leftWidth - rightWidth
	if spacer < 0 {
		spacer = 0
	}

	return lipgloss.NewStyle().
		Width(width).
		Render(left + lipgloss.NewStyle().Width(spacer).Render("") + right)
}
