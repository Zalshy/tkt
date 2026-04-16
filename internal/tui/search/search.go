package search

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/tui/styles"
)

// Model holds the state for the fuzzy search overlay.
// All methods use value receivers and return modified copies.
type Model struct {
	query  string
	active bool
	width  int
}

// New returns a new Model with the given width. The overlay starts inactive.
func New(width int) Model {
	return Model{width: width}
}

// SetWidth returns a copy of m with the width set to w.
func (m Model) SetWidth(w int) Model {
	m.width = w
	return m
}

// Open returns a copy of m with active=true and query cleared.
func (m Model) Open() Model {
	m.active = true
	m.query = ""
	return m
}

// Close returns a copy of m with active=false. The query is preserved.
func (m Model) Close() Model {
	m.active = false
	return m
}

// IsActive reports whether the search overlay is currently open.
func (m Model) IsActive() bool {
	return m.active
}

// Query returns the current search query string.
func (m Model) Query() string {
	return m.query
}

// Update handles incoming tea messages. When inactive it is a no-op.
// KeyEscape closes the overlay, KeyBackspace removes the last rune (rune-safe),
// and KeyRunes appends the typed runes to the query. Always returns a nil cmd.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.Type {
	case tea.KeyEscape:
		return m.Close(), nil
	case tea.KeyBackspace:
		runes := []rune(m.query)
		if len(runes) > 0 {
			m.query = string(runes[:len(runes)-1])
		}
		return m, nil
	case tea.KeyRunes:
		m.query += string(keyMsg.Runes)
		return m, nil
	default:
		return m, nil
	}
}

// View renders the search bar. Returns an empty string when inactive.
// When active, renders a single line padded to the model's width.
func (m Model) View() string {
	if !m.active {
		return ""
	}
	return lipgloss.NewStyle().
		Background(styles.BgMid).
		Foreground(styles.Primary).
		Width(m.width).
		Render("/ " + m.query + "█")
}

// Filter returns the subset of tickets whose "#ID Title" string fuzzy-matches
// the current query. An empty query returns the original slice unchanged.
// When no tickets match, a non-nil empty slice is returned.
func (m Model) Filter(tickets []models.Ticket) []models.Ticket {
	if m.query == "" {
		return tickets
	}

	// Build a list of target strings in input order.
	targets := make([]string, len(tickets))
	for i, t := range tickets {
		targets[i] = fmt.Sprintf("#%d %s", t.ID, t.Title)
	}

	// Use FindNoSort to preserve original order in the matches we collect.
	matches := fuzzy.FindNoSort(m.query, targets)
	if len(matches) == 0 {
		return []models.Ticket{}
	}

	// Sort by original insertion index, not by fuzzy match score.
	// Insertion order equals board display order (top-to-bottom across columns),
	// so filtered results appear in the same sequence as the board — making it
	// easy to predict where a matched ticket sits. Relevance scoring is
	// deliberately not used: stable ordering is more useful than ranking in a
	// small, fixed list where the user already knows roughly what they are
	// looking for.
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Index < matches[j].Index
	})

	result := make([]models.Ticket, len(matches))
	for i, match := range matches {
		result[i] = tickets[match.Index]
	}
	return result
}
