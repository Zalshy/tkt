package keys

import "testing"

func TestAllReturnsFourSections(t *testing.T) {
	if got := len(All()); got != 4 {
		t.Errorf("All() returned %d sections, want 4", got)
	}
}

func TestForKnownScope(t *testing.T) {
	bindings := For(ScopeList)
	if len(bindings) == 0 {
		t.Fatal("For(ScopeList) returned empty slice")
	}
	for i, b := range bindings {
		if b.Key == "" {
			t.Errorf("binding[%d].Key is empty", i)
		}
		if b.Desc == "" {
			t.Errorf("binding[%d].Desc is empty", i)
		}
	}
}

func TestForUnknownScope(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("For(Scope(99)) panicked: %v", r)
		}
	}()
	result := For(Scope(99))
	if len(result) != 0 {
		t.Errorf("For(Scope(99)) returned non-empty: %v", result)
	}
}
