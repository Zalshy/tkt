package testutil

import (
	"regexp"

	tea "github.com/charmbracelet/bubbletea"
)

// ansiRe matches CSI/SGR escape sequences emitted by lipgloss.
// OSC sequences (e.g. \x1b]) are out of scope and not stripped.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// Update calls m.Update(msg) for each msg in order, threading the returned
// model through each call. Nil cmds are dropped — appending nil entries
// panics if invoked. Returns the final model and all collected non-nil cmds.
// If msgs is empty, returns m unchanged with a nil slice.
func Update(m tea.Model, msgs ...tea.Msg) (tea.Model, []tea.Cmd) {
	if len(msgs) == 0 {
		return m, nil
	}
	var cmds []tea.Cmd
	for _, msg := range msgs {
		var cmd tea.Cmd
		m, cmd = m.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return m, cmds
}

// StripANSI removes ANSI CSI/SGR escape sequences from s and returns plain text.
func StripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// KeyMsg returns a tea.KeyMsg representing a single rune key press.
func KeyMsg(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// WindowSize returns a tea.WindowSizeMsg with the given width and height.
func WindowSize(w, h int) tea.WindowSizeMsg {
	return tea.WindowSizeMsg{Width: w, Height: h}
}

// EscMsg returns a tea.KeyMsg representing the Escape key.
func EscMsg() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyEscape}
}
