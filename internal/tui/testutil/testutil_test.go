package testutil

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// counterModel increments a counter on every Update call and returns nil cmd.
type counterModel struct {
	count int
}

func (m counterModel) Init() tea.Cmd                           { return nil }
func (m counterModel) Update(_ tea.Msg) (tea.Model, tea.Cmd) {
	m.count++
	return m, nil
}
func (m counterModel) View() string { return "" }

// cmdModel returns a fixed non-nil tea.Cmd on every Update call.
type cmdModel struct{}

func (m cmdModel) Init() tea.Cmd                           { return nil }
func (m cmdModel) Update(_ tea.Msg) (tea.Model, tea.Cmd) {
	return m, func() tea.Msg { return nil }
}
func (m cmdModel) View() string { return "" }

// mixedModel returns nil cmd on first call, non-nil on subsequent calls.
type mixedModel struct {
	calls int
}

func (m mixedModel) Init() tea.Cmd                           { return nil }
func (m mixedModel) Update(_ tea.Msg) (tea.Model, tea.Cmd) {
	m.calls++
	if m.calls == 1 {
		return m, nil
	}
	return m, func() tea.Msg { return nil }
}
func (m mixedModel) View() string { return "" }

func TestUpdate_SingleMsg(t *testing.T) {
	m := counterModel{}
	got, cmds := Update(m, tea.KeyMsg{})
	cm := got.(counterModel)
	if cm.count != 1 {
		t.Errorf("want count==1, got %d", cm.count)
	}
	if len(cmds) != 0 {
		t.Errorf("want 0 cmds, got %d", len(cmds))
	}
}

func TestUpdate_MultiMsg(t *testing.T) {
	m := counterModel{}
	got, _ := Update(m, tea.KeyMsg{}, tea.KeyMsg{}, tea.KeyMsg{})
	cm := got.(counterModel)
	if cm.count != 3 {
		t.Errorf("want count==3, got %d", cm.count)
	}
}

func TestUpdate_Empty(t *testing.T) {
	m := counterModel{count: 7}
	got, cmds := Update(m)
	cm := got.(counterModel)
	if cm.count != 7 {
		t.Errorf("want count==7 (unchanged), got %d", cm.count)
	}
	if len(cmds) != 0 {
		t.Errorf("want nil or empty cmds, got len %d", len(cmds))
	}
}

func TestUpdate_CollectsCmd(t *testing.T) {
	m := cmdModel{}
	_, cmds := Update(m, tea.KeyMsg{}, tea.KeyMsg{})
	if len(cmds) != 2 {
		t.Errorf("want 2 cmds, got %d", len(cmds))
	}
	for i, cmd := range cmds {
		if cmd == nil {
			t.Errorf("cmd[%d] is nil", i)
		}
	}
}

func TestUpdate_NilCmdDropped(t *testing.T) {
	m := mixedModel{}
	_, cmds := Update(m, tea.KeyMsg{}, tea.KeyMsg{})
	if len(cmds) != 1 {
		t.Errorf("want 1 cmd (nil dropped), got %d", len(cmds))
	}
}

func TestStripANSI_Plain(t *testing.T) {
	input := "hello world"
	got := StripANSI(input)
	if got != input {
		t.Errorf("want %q, got %q", input, got)
	}
}

func TestStripANSI_SGR(t *testing.T) {
	input := "\x1b[1;32mhello\x1b[0m"
	got := StripANSI(input)
	want := "hello"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestStripANSI_Empty(t *testing.T) {
	got := StripANSI("")
	if got != "" {
		t.Errorf("want empty string, got %q", got)
	}
}

func TestKeyMsg(t *testing.T) {
	msg := KeyMsg('q')
	if msg.Type != tea.KeyRunes {
		t.Errorf("want Type==tea.KeyRunes, got %v", msg.Type)
	}
	if len(msg.Runes) != 1 || msg.Runes[0] != 'q' {
		t.Errorf("want Runes==['q'], got %v", msg.Runes)
	}
}

func TestWindowSize(t *testing.T) {
	msg := WindowSize(120, 40)
	if msg.Width != 120 {
		t.Errorf("want Width==120, got %d", msg.Width)
	}
	if msg.Height != 40 {
		t.Errorf("want Height==40, got %d", msg.Height)
	}
}
