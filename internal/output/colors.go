package output

import "github.com/zalshy/tkt/internal/models"

// ANSI escape codes — foreground colors and reset.
// spec §9: use ANSI codes directly for phases 1-5; no lipgloss.
const (
	Reset  = "\033[0m"
	Dim    = "\033[2m"
	Gray   = "\033[90m"
	Blue   = "\033[34m"
	Amber  = "\033[33m"
	Cyan   = "\033[36m"
	Green  = "\033[32m"
	Red    = "\033[31m"
	Purple = "\033[35m"
)

// Colorize wraps text with the given ANSI color code and appends Reset.
func Colorize(text, color string) string {
	return color + text + Reset
}

// StatusColor returns the ANSI color code for the given ticket status.
func StatusColor(s models.Status) string {
	switch s {
	case models.StatusTodo:
		return Gray
	case models.StatusPlanning:
		return Blue
	case models.StatusInProgress:
		return Amber
	case models.StatusDone:
		return Cyan
	case models.StatusVerified:
		return Green
	case models.StatusCanceled:
		return Red
	case models.StatusArchived:
		return Dim
	default:
		return Reset
	}
}

// RoleColor returns the ANSI color code for the given session role.
func RoleColor(r models.Role) string {
	switch r {
	case models.RoleArchitect:
		return Purple
	case models.RoleImplementer:
		return Cyan
	case models.RoleMonitor:
		return Gray
	default:
		return Reset
	}
}

// ColorStatus returns the status string wrapped in its status color.
func ColorStatus(s models.Status) string {
	return Colorize(string(s), StatusColor(s))
}
