// Package modal provides an enum-ordered priority system for TUI overlay management.
// Modals are stored in a Manager by Kind; the lowest numeric Kind wins when multiple
// modals are active simultaneously.
//
// All Manager methods use value receivers and return copies. Callers must assign the
// return value to apply any change — e.g. m.modals = m.modals.Show(...). Forgetting to
// assign is a silent no-op; this is consistent with the rest of the project's immutable-
// update pattern.
package modal

import (
	"github.com/charmbracelet/lipgloss"
)

// Kind identifies a modal type. The numeric value determines render priority:
// lower value = higher priority. KindNone (0) is a sentinel and is never stored.
// The ordering is load-bearing — do not reorder constants without updating tests.
type Kind int

const (
	KindNone    Kind = iota // 0 — sentinel; never stored in a slot
	KindHelp                // 1 — help overlay; highest priority
	KindConfirm             // 2 — confirmation dialog
	KindDetail              // 3 — ticket detail modal overlay
	KindToast               // 4 — toast notification; lowest priority
	numKinds                // sentinel for array sizing; keep last
)

// Modal holds the pre-rendered content string and the terminal width that was
// current when the content was rendered. Both fields are unexported; access them
// through Manager methods.
type Modal struct {
	content  string
	width    int
	occupied bool
}

// Manager stores one Modal slot per Kind. It is a value type; all methods return
// a new Manager. The zero value is valid and represents no active modals.
//
// Cache invalidation contract: Manager never re-renders content. The caller is
// responsible for detecting staleness by comparing WidthFor(kind) against the
// current terminal width and calling Show again when they differ.
type Manager struct {
	slots [numKinds]Modal // indexed by Kind; slot 0 (KindNone) is always zero-value
}

// NewManager returns a zero-value Manager with no active modals.
func NewManager() Manager {
	return Manager{}
}

// Show stores pre-rendered content for the given kind, recording width alongside it
// so callers can detect staleness on terminal resize via WidthFor. Show is a no-op
// when kind == KindNone or kind is out of range.
func (m Manager) Show(kind Kind, content string, width int) Manager {
	if kind <= KindNone || kind >= numKinds {
		return m
	}
	m.slots[kind] = Modal{content: content, width: width, occupied: true}
	return m
}

// Dismiss clears the slot for kind. It is a no-op when kind == KindNone, kind is
// out of range, or the slot is already empty.
func (m Manager) Dismiss(kind Kind) Manager {
	if kind <= KindNone || kind >= numKinds {
		return m
	}
	m.slots[kind] = Modal{}
	return m
}

// DismissAll returns a Manager with every slot cleared.
func (m Manager) DismissAll() Manager {
	return Manager{}
}

// Active returns the highest-priority occupied slot (lowest non-zero Kind index).
// Returns (KindNone, "") when no modals are active.
func (m Manager) Active() (Kind, string) {
	for i := Kind(1); i < numKinds; i++ {
		if m.slots[i].occupied {
			return i, m.slots[i].content
		}
	}
	return KindNone, ""
}

// HasActive returns true when at least one slot holds content.
func (m Manager) HasActive() bool {
	kind, _ := m.Active()
	return kind != KindNone
}

// WidthFor returns the stored width for the given kind. Returns 0 if the kind is
// not set or is out of range. Used by callers to detect when modal content needs
// to be re-rendered after a terminal resize.
func (m Manager) WidthFor(kind Kind) int {
	if kind <= KindNone || kind >= numKinds {
		return 0
	}
	return m.slots[kind].width
}

// Overlay centers modal content over a canvas of the given dimensions using
// lipgloss.Place. lipgloss.Place positions modal within a width×height canvas;
// it does not alpha-composite rendered strings. RootModel replaces the panesView
// string with the return value of this function when a modal is active.
func Overlay(modal string, width, height int) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, modal)
}
