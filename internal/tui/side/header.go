package side

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// clockTickMsg is sent when the next wall-clock minute boundary arrives.
type clockTickMsg struct{}

// clockModel holds the current time for the header clock.
type clockModel struct {
	now time.Time
}

func newClockModel() clockModel {
	return clockModel{now: time.Now()}
}

func clockCmd() tea.Cmd {
	next := time.Now().Truncate(time.Minute).Add(time.Minute)
	return tea.Tick(time.Until(next), func(time.Time) tea.Msg {
		return clockTickMsg{}
	})
}

func (c clockModel) update(msg tea.Msg) (clockModel, tea.Cmd) {
	if _, ok := msg.(clockTickMsg); ok {
		c.now = time.Now()
		return c, clockCmd()
	}
	return c, nil
}

// Header colour constants.
var (
	pillBg  = lipgloss.Color("#C678DD") // purple-pink — fills the entire title area
	pillFg  = lipgloss.Color("#0D0F14") // near-black text on bright badge
	clockBg = lipgloss.Color("#56B6C2") // cyan clock badge on the right
	clockFg = lipgloss.Color("#0D0F14")
)

// renderHeader renders a full-width header bar:
//
//	│           tkt monitoring           │ 17:12 │
//
// The entire left portion uses the purple-pink background with the title
// centered. The clock keeps its cyan badge pinned to the right edge.
func renderHeader(c clockModel, width int) string {
	clockBadge := lipgloss.NewStyle().
		Background(clockBg).
		Foreground(clockFg).
		Bold(true).
		Padding(0, 1).
		Render(c.now.Format("15:04"))

	clockW := lipgloss.Width(clockBadge)
	titleW := width - clockW
	if titleW < 0 {
		titleW = 0
	}

	titlePart := lipgloss.NewStyle().
		Background(pillBg).
		Foreground(pillFg).
		Bold(true).
		Width(titleW).
		Align(lipgloss.Center).
		Render("tkt monitoring")

	return lipgloss.JoinHorizontal(lipgloss.Top, titlePart, clockBadge)
}
