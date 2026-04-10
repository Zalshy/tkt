package modal

import (
	"strings"
	"testing"

	"github.com/zalshy/tkt/internal/tui/testutil"
)

// TestShow_StoresModal verifies that Show stores content and Active returns it.
func TestShow_StoresModal(t *testing.T) {
	m := NewManager().Show(KindHelp, "help content", 80)
	kind, content := m.Active()
	if kind != KindHelp {
		t.Errorf("Active() kind = %v, want KindHelp", kind)
	}
	if content != "help content" {
		t.Errorf("Active() content = %q, want %q", content, "help content")
	}
	if !m.HasActive() {
		t.Error("HasActive() = false, want true")
	}
}

// TestActive_Priority verifies that KindHelp wins over KindToast (lower index = higher priority).
func TestActive_Priority(t *testing.T) {
	m := NewManager().
		Show(KindHelp, "help", 80).
		Show(KindToast, "toast", 80)

	kind, content := m.Active()
	if kind != KindHelp {
		t.Errorf("Active() kind = %v, want KindHelp", kind)
	}
	if content != "help" {
		t.Errorf("Active() content = %q, want %q", content, "help")
	}

	// After dismissing KindHelp, KindToast should become active.
	m2 := m.Dismiss(KindHelp)
	kind2, content2 := m2.Active()
	if kind2 != KindToast {
		t.Errorf("after Dismiss(KindHelp), Active() kind = %v, want KindToast", kind2)
	}
	if content2 != "toast" {
		t.Errorf("after Dismiss(KindHelp), Active() content = %q, want %q", content2, "toast")
	}
}

// TestDismiss_ClearsSlot verifies that Dismiss removes a modal and HasActive becomes false.
func TestDismiss_ClearsSlot(t *testing.T) {
	m := NewManager().Show(KindToast, "toast", 80).Dismiss(KindToast)
	if m.HasActive() {
		t.Error("HasActive() = true after Dismiss, want false")
	}
	kind, content := m.Active()
	if kind != KindNone {
		t.Errorf("Active() kind = %v, want KindNone", kind)
	}
	if content != "" {
		t.Errorf("Active() content = %q, want empty", content)
	}
}

// TestDismissAll_ClearsAll verifies that DismissAll clears every slot.
func TestDismissAll_ClearsAll(t *testing.T) {
	m := NewManager().
		Show(KindHelp, "help", 80).
		Show(KindToast, "toast", 80).
		DismissAll()
	if m.HasActive() {
		t.Error("HasActive() = true after DismissAll, want false")
	}
}

// TestHasActive_False verifies that a fresh Manager reports no active modals.
func TestHasActive_False(t *testing.T) {
	m := NewManager()
	if m.HasActive() {
		t.Error("HasActive() = true on fresh Manager, want false")
	}
}

// TestWidthFor verifies WidthFor returns the stored width and 0 after Dismiss.
func TestWidthFor(t *testing.T) {
	m := NewManager().Show(KindHelp, "help", 120)
	if w := m.WidthFor(KindHelp); w != 120 {
		t.Errorf("WidthFor(KindHelp) = %d, want 120", w)
	}
	m2 := m.Dismiss(KindHelp)
	if w := m2.WidthFor(KindHelp); w != 0 {
		t.Errorf("WidthFor(KindHelp) after Dismiss = %d, want 0", w)
	}
}

// TestOverlay_ContainsBoth verifies that the modal content appears in the Overlay output.
func TestOverlay_ContainsBoth(t *testing.T) {
	result := Overlay("background content", "modal content", 80, 24)
	stripped := testutil.StripANSI(result)
	if !strings.Contains(stripped, "modal content") {
		t.Errorf("Overlay output does not contain %q; got: %q", "modal content", stripped)
	}
}

// TestKindDetail_Priority verifies that KindHelp wins over KindDetail which wins over KindToast.
func TestKindDetail_Priority(t *testing.T) {
	m := NewManager().
		Show(KindHelp, "help", 80).
		Show(KindDetail, "detail", 80).
		Show(KindToast, "toast", 80)

	kind, _ := m.Active()
	if kind != KindHelp {
		t.Errorf("Active() kind = %v, want KindHelp", kind)
	}

	// After dismissing KindHelp, KindDetail should win.
	m2 := m.Dismiss(KindHelp)
	kind2, _ := m2.Active()
	if kind2 != KindDetail {
		t.Errorf("after Dismiss(KindHelp), Active() kind = %v, want KindDetail", kind2)
	}

	// After dismissing KindDetail, KindToast should win.
	m3 := m2.Dismiss(KindDetail)
	kind3, _ := m3.Active()
	if kind3 != KindToast {
		t.Errorf("after Dismiss(KindDetail), Active() kind = %v, want KindToast", kind3)
	}
}
