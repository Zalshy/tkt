package help

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRender_nonEmpty(t *testing.T) {
	result := Render(120)
	if result == "" {
		t.Fatal("Render(120) returned empty string")
	}
}

func TestRender_narrow(t *testing.T) {
	result := Render(40)
	w := lipgloss.Width(result)
	if w > 40 {
		t.Fatalf("Render(40) width = %d, want <= 40", w)
	}
}

func TestRender_containsSection(t *testing.T) {
	result := Render(120)
	if !strings.Contains(result, "Global") {
		t.Fatal("Render(120) does not contain \"Global\"")
	}
}
