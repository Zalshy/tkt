package side

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestDefaultPollInterval verifies that a nil cfg produces a 5s poll interval.
func TestDefaultPollInterval(t *testing.T) {
	m := NewRootModel(nil, nil, "")
	if m.pollInterval != 5*time.Second {
		t.Errorf("expected 5s poll interval, got %v", m.pollInterval)
	}
}

// TestMinSizeGuard verifies that View() renders a "too small" message when
// dimensions are below the 60×20 threshold.
func TestMinSizeGuard(t *testing.T) {
	m := NewRootModel(nil, nil, "")
	// zero-value model has width=0, height=0 — guard must fire
	out := m.View()
	if !strings.Contains(out, "too small") && !strings.Contains(out, "0x0") {
		t.Errorf("expected min-size guard output, got: %q", out)
	}
}

// TestPlaceholderSections verifies that View() renders all three placeholder
// sections when terminal dimensions are sufficient.
func TestPlaceholderSections(t *testing.T) {
	m := NewRootModel(nil, nil, "")
	// Set dimensions above the guard threshold via Update (value receiver — must
	// capture the returned model).
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(RootModel)

	out := m.View()
	for _, want := range []string{"loading", "TICKET ACTIVITY", "SESSIONS"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in View() output, got: %q", want, out)
		}
	}
}

// TestQuitOnQ verifies that pressing "q" returns tea.Quit.
func TestQuitOnQ(t *testing.T) {
	m := NewRootModel(nil, nil, "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(RootModel)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected a non-nil cmd for 'q'")
	}
	// tea.Quit returns a tea.QuitMsg when invoked
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

// TestQuitOnCtrlC verifies that pressing ctrl+c returns tea.Quit.
func TestQuitOnCtrlC(t *testing.T) {
	m := NewRootModel(nil, nil, "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(RootModel)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected a non-nil cmd for ctrl+c")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}
