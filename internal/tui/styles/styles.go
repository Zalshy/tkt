package styles

import "github.com/charmbracelet/lipgloss"

// Text hierarchy — 4 levels, lightest to darkest on a dark background.
var (
	Primary   = lipgloss.Color("#F9FAFB") // near-white, primary text
	Secondary = lipgloss.Color("#9CA3AF") // slate-400, secondary text
	Muted     = lipgloss.Color("#6B7280") // slate-500, de-emphasized text
	Faint     = lipgloss.Color("#4B5563") // slate-600, barely visible labels
)

// Background hierarchy — 3 levels, deepest to shallowest.
var (
	BgDeep = lipgloss.Color("#0D0F14") // near-black, no blue cast
	BgMid  = lipgloss.Color("#161B22") // true dark neutral
	BgSurf = lipgloss.Color("#21262D") // elevated surface
)

// Semantic status / action colors.
var (
	Warning = lipgloss.Color("#F59E0B") // amber-400, planning state
	Danger  = lipgloss.Color("#EF4444") // red-500, error / destructive
)

// Accent and tier colors
var (
	Accent       = lipgloss.Color("#9CA3AF") // slate-400 — focus/active
	TierCritical = lipgloss.Color("#EF4444") // red-500
	TierStandard = lipgloss.Color("#6366F1") // indigo-500 — distinct from slate Accent highlight
	TierLow      = lipgloss.Color("#10B981") // emerald-500
)

// Status lane colors for column headers
var (
	StatusTodo     = lipgloss.Color("#94A3B8") // slate-400
	StatusPlanning = lipgloss.Color("#F59E0B") // amber-400 — same value as Warning
	StatusInProg   = lipgloss.Color("#F472B6") // pink-400 — distinct from blue selection highlight
	StatusDone     = lipgloss.Color("#10B981") // emerald-500
	StatusVerified = lipgloss.Color("#6EE7B7") // emerald-300
)

// KeyHint is the badge style for footer key hints (e.g. "r", "q", "↑↓").
// Renders as a small pill: muted foreground on BgSurf background with 1-cell
// horizontal padding. Matches styleKeyHint in view.go exactly.
var KeyHint = lipgloss.NewStyle().
	Foreground(Primary).
	Background(BgSurf).
	Padding(0, 1)

// PanelActive is the border style for a focused panel component.
// Uses Primary border color to indicate the active/focused state.
var PanelActive = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(Accent)

// PanelInactive is the border style for an unfocused panel component.
// Uses Faint border color to de-emphasize the inactive state.
var PanelInactive = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(Faint)

// Theme holds overrides for every semantic color and the KeyHint style.
// Zero values are ignored: a field left at its zero value keeps the current default.
type Theme struct {
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Muted     lipgloss.Color
	Faint     lipgloss.Color
	BgDeep    lipgloss.Color
	BgMid     lipgloss.Color
	BgSurf    lipgloss.Color
	Warning   lipgloss.Color
	Danger    lipgloss.Color

	Accent       lipgloss.Color
	TierCritical lipgloss.Color
	TierStandard lipgloss.Color
	TierLow      lipgloss.Color

	StatusTodo     lipgloss.Color
	StatusPlanning lipgloss.Color
	StatusInProg   lipgloss.Color
	StatusDone     lipgloss.Color
	StatusVerified lipgloss.Color

	KeyHint lipgloss.Style
	// KeyHintSet signals that the KeyHint field is intentional (zero Style is valid).
	KeyHintSet bool
}

// ApplyTheme overwrites the package-level vars with the values in t.
// Only non-zero color fields are applied. KeyHint is applied only when
// t.KeyHintSet is true.
//
// ApplyTheme is not safe for concurrent use. It is intended to be called
// once at program startup before tea.NewProgram.
func ApplyTheme(t Theme) {
	if t.Primary != "" {
		Primary = t.Primary
	}
	if t.Secondary != "" {
		Secondary = t.Secondary
	}
	if t.Muted != "" {
		Muted = t.Muted
	}
	if t.Faint != "" {
		Faint = t.Faint
	}
	if t.BgDeep != "" {
		BgDeep = t.BgDeep
	}
	if t.BgMid != "" {
		BgMid = t.BgMid
	}
	if t.BgSurf != "" {
		BgSurf = t.BgSurf
	}
	if t.Warning != "" {
		Warning = t.Warning
	}
	if t.Danger != "" {
		Danger = t.Danger
	}
	if t.Accent != "" {
		Accent = t.Accent
	}
	if t.TierCritical != "" {
		TierCritical = t.TierCritical
	}
	if t.TierStandard != "" {
		TierStandard = t.TierStandard
	}
	if t.TierLow != "" {
		TierLow = t.TierLow
	}
	if t.StatusTodo != "" {
		StatusTodo = t.StatusTodo
	}
	if t.StatusPlanning != "" {
		StatusPlanning = t.StatusPlanning
	}
	if t.StatusInProg != "" {
		StatusInProg = t.StatusInProg
	}
	if t.StatusDone != "" {
		StatusDone = t.StatusDone
	}
	if t.StatusVerified != "" {
		StatusVerified = t.StatusVerified
	}
	if t.KeyHintSet {
		KeyHint = t.KeyHint
	}
}
