package header

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/tui/styles"
)

// TickMsg is sent by TickCmd on each animation frame.
type TickMsg struct{}

// TickCmd returns a tea.Cmd that fires after 16ms sending a TickMsg.
// The root model calls this and routes the result back to the header.
func TickCmd() tea.Cmd {
	return tea.Tick(16*time.Millisecond, func(time.Time) tea.Msg {
		return TickMsg{}
	})
}

// InitCmd returns TickCmd — the root model calls header.InitCmd() from its
// Init() to kick off the intro animation.
func InitCmd() tea.Cmd {
	return TickCmd()
}

// Model holds the state for the header component.
type Model struct {
	width        int
	activeTab    int
	tabs         []string
	animDone     bool
	animProgress float64 // 0.0 → 1.0+ (overshoot) → 1.0
	animTick     int     // counts ticks elapsed
}

// New constructs a Model with the fixed tab set. activeTab is clamped to valid range.
func New(width, activeTab int) Model {
	m := Model{
		width: width,
		tabs:  []string{"TODO", "PLANNING", "DONE"},
	}
	m.activeTab = clampTab(activeTab, len(m.tabs))
	return m
}

// SetWidth returns a new Model with the width updated.
func (m Model) SetWidth(w int) Model {
	m.width = w
	return m
}

// SetActiveTab returns a new Model with activeTab clamped to [0, len(tabs)-1].
func (m Model) SetActiveTab(tab int) Model {
	m.activeTab = clampTab(tab, len(m.tabs))
	return m
}

// Update handles animation tick messages. On each tickMsg it advances the
// animation and returns the next TickCmd until the animation completes.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg.(type) {
	case TickMsg:
		if m.animDone {
			return m, nil
		}
		m.animTick++
		t := float64(m.animTick) * 16 / 600 // 0..1 over 600ms
		if t >= 1.0 {
			m.animProgress = 1.0
			m.animDone = true
			return m, nil
		}
		overshoot := math.Sin(t*math.Pi) * 0.15
		m.animProgress = t + overshoot
		return m, TickCmd()
	}
	return m, nil
}

// View renders the header as exactly two lines joined with "\n".
// Line 1: "tkt" title (bold, Accent-coloured, no background so it reads on any terminal).
// Line 2: a full-width separator in Muted colour that visually anchors the header.
// If width <= 0, returns "\n\n" (two empty lines).
func (m Model) View() string {
	if m.width <= 0 {
		return "\n\n"
	}

	// Line 1 — title bar
	var titleLine string
	if m.animDone {
		titleLine = lipgloss.NewStyle().
			Foreground(styles.Accent).
			Bold(true).
			Width(m.width).
			Render("  tkt")
	} else {
		title := []rune("tkt")
		var sb strings.Builder
		sb.WriteString("  ") // leading padding
		for i, r := range title {
			factor := m.animProgress - float64(i)*0.15
			if factor < 0 {
				factor = 0
			} else if factor > 1 {
				factor = 1
			}
			color := lerpColor(styles.Secondary, styles.Accent, factor)
			sb.WriteString(lipgloss.NewStyle().
				Foreground(color).
				Bold(true).
				Render(string(r)))
		}
		// Pad remaining width (plain spaces, terminal default bg)
		rendered := sb.String()
		pad := m.width - 2 - len(title)
		if pad > 0 {
			rendered += strings.Repeat(" ", pad)
		}
		titleLine = rendered
	}

	return titleLine + "\n" + m.renderTabBar()
}

// renderTabBar builds the tab bar by rendering each rune individually with its own
// per-position background color and appropriate foreground color.
func (m Model) renderTabBar() string {
	w := m.width

	// Build the full rune sequence for the tab bar text.
	// Format: " TODO  PLANNING  IN_PROGRESS  DONE  VERIFIED " padded to width.
	runes := make([]rune, 0, w)
	tabIndex := make([]int, 0, w) // parallel: tab index per rune, or -1 for padding

	// One leading space (padding before first tab)
	runes = append(runes, ' ')
	tabIndex = append(tabIndex, -1)

	for ti, label := range m.tabs {
		// One leading space before label
		runes = append(runes, ' ')
		tabIndex = append(tabIndex, -1)
		// Label runes
		for _, r := range label {
			runes = append(runes, r)
			tabIndex = append(tabIndex, ti)
		}
		// One trailing space after label
		runes = append(runes, ' ')
		tabIndex = append(tabIndex, -1)
	}

	// Pad or truncate to exactly width
	for len(runes) < w {
		runes = append(runes, ' ')
		tabIndex = append(tabIndex, -1)
	}
	if len(runes) > w {
		runes = runes[:w]
		tabIndex = tabIndex[:w]
	}

	// Precompute the dimmed inactive color
	dimmed := dimColor(styles.Primary)

	// Render each rune with its own style
	denom := float64(max(w-1, 1))
	parts := make([]string, w)
	for i := 0; i < w; i++ {
		t := float64(i) / denom
		bg := lerpColor(styles.BgMid, styles.BgDeep, t)

		s := lipgloss.NewStyle().Background(bg)

		ti := tabIndex[i]
		if ti == m.activeTab {
			s = s.Foreground(styles.Primary)
		} else if ti >= 0 {
			s = s.Foreground(dimmed)
		}
		// Padding runes: no explicit foreground

		parts[i] = s.Render(string(runes[i]))
	}

	return strings.Join(parts, "")
}

// dimColor applies the "35% + 30 offset" dimming formula per channel to color c.
// If c is not a 6-digit hex string (e.g. named color), falls back to styles.Muted.
func dimColor(c lipgloss.Color) lipgloss.Color {
	r, g, b, ok := parseHex(string(c))
	if !ok {
		// Fall back gracefully — named color or unrecognized format
		return styles.Muted
	}
	rd := clampByte(int(r)*35/100 + 30)
	gd := clampByte(int(g)*35/100 + 30)
	bd := clampByte(int(b)*35/100 + 30)
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", rd, gd, bd))
}

// lerpColor linearly interpolates between colors a and b by factor t (0.0=a, 1.0=b).
// If either color is not a 6-digit hex string, falls back to a unchanged.
func lerpColor(a, b lipgloss.Color, t float64) lipgloss.Color {
	ra, ga, ba, ok1 := parseHex(string(a))
	rb, gb, bb, ok2 := parseHex(string(b))
	if !ok1 || !ok2 {
		// Fall back gracefully — unrecognized color format
		return a
	}
	r := int(math.Round(float64(ra) + t*float64(int(rb)-int(ra))))
	g := int(math.Round(float64(ga) + t*float64(int(gb)-int(ga))))
	bv := int(math.Round(float64(ba) + t*float64(int(bb)-int(ba))))
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X",
		clampByte(r), clampByte(g), clampByte(bv)))
}

// parseHex parses a "#RRGGBB" color string into its R, G, B components.
// Returns ok=false if the string is not in that exact format.
func parseHex(s string) (r, g, b uint8, ok bool) {
	if len(s) != 7 || s[0] != '#' {
		return 0, 0, 0, false
	}
	val, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	r = uint8(val >> 16)
	g = uint8((val >> 8) & 0xFF)
	b = uint8(val & 0xFF)
	return r, g, b, true
}

// clampByte clamps an int to [0, 255].
func clampByte(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// clampTab clamps a tab index to [0, n-1].
func clampTab(tab, n int) int {
	if tab < 0 {
		return 0
	}
	if tab >= n {
		return n - 1
	}
	return tab
}
