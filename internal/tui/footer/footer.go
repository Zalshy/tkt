package footer

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/tui/keys"
	"github.com/zalshy/tkt/internal/tui/styles"
)

// Context identifies which pane or modal is currently active, so the footer
// can show the appropriate key hints.
type Context int

const (
	ContextGlobal Context = iota // zero value — always safe default
	ContextList
	ContextDetail
	ContextSearch
	ContextModal // modal covering screen — show only esc hint
)

// Hint is a single key-label pair rendered in the footer.
type Hint struct {
	Key   string
	Label string
}

func scopeFor(ctx Context) keys.Scope {
	switch ctx {
	case ContextList:
		return keys.ScopeList
	case ContextDetail:
		return keys.ScopeDetail
	case ContextSearch:
		return keys.ScopeSearch
	default:
		return keys.ScopeGlobal
	}
}

// hintsFor returns the hint set for the given context.
func hintsFor(ctx Context) []Hint {
	if ctx == ContextModal {
		return []Hint{{"esc", "close"}}
	}
	bindings := keys.For(scopeFor(ctx))
	out := make([]Hint, len(bindings))
	for i, b := range bindings {
		out[i] = Hint{Key: b.Key, Label: b.Desc}
	}
	return out
}

// SessionCounts holds the number of active sessions per base role.
type SessionCounts struct {
	Arch int
	Impl int
}

// Model holds the rendering state for the footer component.
// All methods use value receivers and return by value — no hidden mutation.
type Model struct {
	width  int
	ctx    Context
	counts SessionCounts
}

// New returns a new Model with the given terminal width and active context.
func New(width int, ctx Context) Model {
	return Model{width: width, ctx: ctx}
}

// SetWidth returns a copy of m with the width updated.
func (m Model) SetWidth(w int) Model {
	m.width = w
	return m
}

// SetContext returns a copy of m with the context updated.
func (m Model) SetContext(ctx Context) Model {
	m.ctx = ctx
	return m
}

// SetSessionCounts returns a copy of m with the session counts updated.
func (m Model) SetSessionCounts(c SessionCounts) Model {
	m.counts = c
	return m
}

// View renders the footer as two lines joined with "\n":
//  1. Session count line: "🧠 arch: N   ⚙️  impl: N" — muted style, centered.
//  2. Key hint badges, truncated to fit within m.width.
//
// When m.width is 0, no truncation or centering is applied.
func (m Model) View() string {
	// — session count line —
	sessionText := fmt.Sprintf(
		"🧠 arch: %d   ⚙️  impl: %d",
		m.counts.Arch,
		m.counts.Impl,
	)
	muted := lipgloss.NewStyle().Foreground(styles.Muted)
	sessionLine := lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(
		muted.Render(sessionText),
	)

	// — hint line —
	hints := hintsFor(m.ctx)
	sep := "  "
	result := ""

	for i, h := range hints {
		part := styles.KeyHint.Render(h.Key) + " " + muted.Render(h.Label)
		candidate := result
		if i > 0 {
			candidate += sep
		}
		candidate += part
		if m.width > 0 && lipgloss.Width(candidate) > m.width {
			break
		}
		result = candidate
	}

	hintLine := result
	if m.width > 0 {
		hintLine = lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(result)
	}
	return sessionLine + "\n" + hintLine
}
