package styles

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// resetDefaults restores all package-level vars to their original values.
// Call via t.Cleanup in any test that calls ApplyTheme.
func resetDefaults() {
	Primary   = lipgloss.Color("#F9FAFB")
	Secondary = lipgloss.Color("#9CA3AF")
	Muted     = lipgloss.Color("#6B7280")
	Faint     = lipgloss.Color("#4B5563")
	BgDeep    = lipgloss.Color("#0D0F14")
	BgMid     = lipgloss.Color("#161B22")
	BgSurf    = lipgloss.Color("#21262D")
	Warning   = lipgloss.Color("#F59E0B")
	Danger    = lipgloss.Color("#EF4444")
	Accent       = lipgloss.Color("#6366F1")
	TierCritical = lipgloss.Color("#EF4444")
	TierStandard = lipgloss.Color("#6366F1")
	TierLow      = lipgloss.Color("#10B981")
	StatusTodo     = lipgloss.Color("#94A3B8")
	StatusPlanning = lipgloss.Color("#F59E0B")
	StatusInProg   = lipgloss.Color("#3B82F6")
	StatusDone     = lipgloss.Color("#10B981")
	StatusVerified = lipgloss.Color("#6EE7B7")
	KeyHint = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9FAFB")).
		Background(lipgloss.Color("#21262D")).
		Padding(0, 1)
}

// TestApplyTheme_OverwritesColor verifies that a non-zero color field in
// Theme overwrites the corresponding package var, and that all other vars
// are left unchanged.
func TestApplyTheme_OverwritesColor(t *testing.T) {
	t.Cleanup(resetDefaults)

	originalSecondary := Secondary
	originalMuted := Muted
	originalFaint := Faint
	originalBgDeep := BgDeep
	originalBgMid := BgMid
	originalBgSurf := BgSurf
	originalWarning := Warning
	originalDanger := Danger

	newPrimary := lipgloss.Color("#FFFFFF")
	ApplyTheme(Theme{Primary: newPrimary})

	if Primary != newPrimary {
		t.Errorf("Primary: got %q, want %q", Primary, newPrimary)
	}
	if Secondary != originalSecondary {
		t.Errorf("Secondary changed unexpectedly: got %q", Secondary)
	}
	if Muted != originalMuted {
		t.Errorf("Muted changed unexpectedly: got %q", Muted)
	}
	if Faint != originalFaint {
		t.Errorf("Faint changed unexpectedly: got %q", Faint)
	}
	if BgDeep != originalBgDeep {
		t.Errorf("BgDeep changed unexpectedly: got %q", BgDeep)
	}
	if BgMid != originalBgMid {
		t.Errorf("BgMid changed unexpectedly: got %q", BgMid)
	}
	if BgSurf != originalBgSurf {
		t.Errorf("BgSurf changed unexpectedly: got %q", BgSurf)
	}
	if Warning != originalWarning {
		t.Errorf("Warning changed unexpectedly: got %q", Warning)
	}
	if Danger != originalDanger {
		t.Errorf("Danger changed unexpectedly: got %q", Danger)
	}
}

// TestApplyTheme_SkipsZeroColor verifies that a zero-value lipgloss.Color
// field in Theme does not overwrite the corresponding package var.
func TestApplyTheme_SkipsZeroColor(t *testing.T) {
	t.Cleanup(resetDefaults)

	originalPrimary := Primary

	// Apply a Theme with Primary explicitly left as zero value.
	ApplyTheme(Theme{Primary: lipgloss.Color("")})

	if Primary != originalPrimary {
		t.Errorf("Primary was overwritten by zero value: got %q, want %q", Primary, originalPrimary)
	}
}

// TestApplyTheme_KeyHint_Applied verifies that when KeyHintSet is true,
// the KeyHint style is updated, and the resulting foreground/background
// match the Muted and BgSurf values applied in the same theme.
func TestApplyTheme_KeyHint_Applied(t *testing.T) {
	t.Cleanup(resetDefaults)

	newMuted := lipgloss.Color("#AAAAAA")
	newBgSurf := lipgloss.Color("#222222")
	newKeyHint := lipgloss.NewStyle().
		Foreground(newMuted).
		Background(newBgSurf).
		Padding(0, 1)

	ApplyTheme(Theme{
		Muted:      newMuted,
		BgSurf:     newBgSurf,
		KeyHint:    newKeyHint,
		KeyHintSet: true,
	})

	if KeyHint.GetForeground() != newMuted {
		t.Errorf("KeyHint foreground: got %v, want %v", KeyHint.GetForeground(), newMuted)
	}
	if KeyHint.GetBackground() != newBgSurf {
		t.Errorf("KeyHint background: got %v, want %v", KeyHint.GetBackground(), newBgSurf)
	}
}

// TestApplyTheme_KeyHint_NotApplied verifies that when KeyHintSet is false,
// the KeyHint package var is not overwritten even if a non-zero KeyHint
// style is provided in the Theme.
func TestApplyTheme_KeyHint_NotApplied(t *testing.T) {
	t.Cleanup(resetDefaults)

	originalFg := KeyHint.GetForeground()
	originalBg := KeyHint.GetBackground()

	differentKeyHint := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#FFFFFF")).
		Padding(0, 2)

	ApplyTheme(Theme{
		KeyHint:    differentKeyHint,
		KeyHintSet: false,
	})

	if KeyHint.GetForeground() != originalFg {
		t.Errorf("KeyHint foreground changed unexpectedly: got %v, want %v", KeyHint.GetForeground(), originalFg)
	}
	if KeyHint.GetBackground() != originalBg {
		t.Errorf("KeyHint background changed unexpectedly: got %v, want %v", KeyHint.GetBackground(), originalBg)
	}
}
