package toast

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/tui/styles"
)

// Variant controls the visual style of the toast notification.
type Variant int

const (
	Success Variant = iota
	Error
)

// Render returns a pre-rendered toast string sized to width.
// width is the full terminal width; the rendered box is capped at min(width-4, 40).
// A bounds check ensures boxWidth never goes below 0.
func Render(message string, variant Variant, width int) string {
	boxWidth := width - 4
	if boxWidth > 40 {
		boxWidth = 40
	}
	if boxWidth < 0 {
		boxWidth = 0
	}

	var fg lipgloss.Color
	var prefix string
	switch variant {
	case Success:
		fg = styles.Primary
		prefix = "✓ "
	case Error:
		fg = styles.Danger
		prefix = "✗ "
	}

	textStyle := lipgloss.NewStyle().
		Foreground(fg)

	body := textStyle.Render(prefix + message)

	boxStyle := lipgloss.NewStyle().
		Background(styles.BgMid).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(fg).
		Padding(0, 1).
		Width(boxWidth)

	return boxStyle.Render(body)
}

// ExpireCmd returns a tea.Cmd that sleeps 3 seconds then emits ToastExpiredMsg.
// The goroutine spawned by time.Sleep has no cancellation path. This is
// acceptable: the toast lifetime is 3 seconds and any in-flight sleep exits
// with the process when BubbleTea calls tea.Quit.
func ExpireCmd() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(3 * time.Second)
		return ToastExpiredMsg{}
	}
}

// ToastExpiredMsg is sent by ExpireCmd when the auto-dismiss timer fires.
type ToastExpiredMsg struct{}
