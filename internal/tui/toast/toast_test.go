package toast

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRender_success_nonEmpty(t *testing.T) {
	result := Render("saved", Success, 120)
	if result == "" {
		t.Fatal("expected non-empty string from Render with Success variant")
	}
}

func TestRender_error_nonEmpty(t *testing.T) {
	result := Render("failed", Error, 120)
	if result == "" {
		t.Fatal("expected non-empty string from Render with Error variant")
	}
}

func TestRender_narrow(t *testing.T) {
	result := Render("x", Success, 40)
	w := lipgloss.Width(result)
	if w > 40 {
		t.Fatalf("expected rendered width <= 40, got %d", w)
	}
}

func TestExpireCmd_returnsMsg(t *testing.T) {
	if testing.Short() {
		t.Skip("skips 3s timer")
	}
	msg := ExpireCmd()()
	if _, ok := msg.(ToastExpiredMsg); !ok {
		t.Fatalf("expected ToastExpiredMsg, got %T", msg)
	}
}
