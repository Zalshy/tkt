package footer

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/tui/testutil"
)

// TestView_ContextHints verifies that each context renders its expected key strings.
func TestView_ContextHints(t *testing.T) {
	cases := []struct {
		ctx      Context
		expected []string
	}{
		{ContextGlobal, []string{"q", "?"}},
		{ContextList, []string{"j/k", "enter", "/", "q"}},
		{ContextDetail, []string{"j/k", "esc"}},
		{ContextSearch, []string{"esc", "enter"}},
	}

	for _, tc := range cases {
		result := testutil.StripANSI(New(200, tc.ctx).View())
		for _, key := range tc.expected {
			if !strings.Contains(result, key) {
				t.Errorf("context %d: expected key %q in output %q", tc.ctx, key, result)
			}
		}
	}
}

// TestView_Truncation verifies that output does not exceed the configured width.
func TestView_Truncation(t *testing.T) {
	result := New(20, ContextList).View()
	w := lipgloss.Width(result)
	if w > 20 {
		t.Errorf("expected width <= 20, got %d", w)
	}
	// no panic is also verified by reaching this line
}

// TestView_WidthZeroNoPanic verifies that a zero width does not panic.
func TestView_WidthZeroNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("View() panicked with width=0: %v", r)
		}
	}()
	New(0, ContextList).View()
}

// TestView_VeryNarrowNoPanic verifies that a width of 1 does not panic.
func TestView_VeryNarrowNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("View() panicked with width=1: %v", r)
		}
	}()
	New(1, ContextList).View()
}

// TestView_SetChaining verifies that SetContext and SetWidth method chaining works.
func TestView_SetChaining(t *testing.T) {
	result := testutil.StripANSI(New(80, ContextGlobal).SetContext(ContextList).SetWidth(120).View())
	for _, key := range []string{"j/k", "enter", "/"} {
		if !strings.Contains(result, key) {
			t.Errorf("chained model: expected list-context key %q in output %q", key, result)
		}
	}
}
