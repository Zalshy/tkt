package state

import (
	"errors"
	"testing"

	"github.com/zalshy/tkt/internal/models"
)

// helper builds a session with a given ID and role.
// For built-in roles (architect, implementer), EffectiveRole equals Role.
func sess(id string, role models.Role) *models.Session {
	return &models.Session{ID: id, Role: role, EffectiveRole: role}
}

func TestValidateTransition_AllValid(t *testing.T) {
	arch := sess("arch-alice-1111", models.RoleArchitect)
	impl := sess("impl-bob-2222", models.RoleImplementer)
	// submitter is always a different session from the actor.
	otherArch := sess("arch-carol-3333", models.RoleArchitect)
	otherImpl := sess("impl-dave-4444", models.RoleImplementer)

	tests := []struct {
		from      models.Status
		to        models.Status
		actor     *models.Session
		submitter *models.Session
	}{
		{models.StatusTodo, models.StatusPlanning, arch, otherImpl},
		{models.StatusTodo, models.StatusPlanning, impl, otherArch},
		{models.StatusTodo, models.StatusCanceled, arch, otherImpl},
		{models.StatusTodo, models.StatusCanceled, impl, otherArch},
		{models.StatusPlanning, models.StatusInProgress, arch, otherImpl}, // arch, different session
		{models.StatusPlanning, models.StatusCanceled, arch, otherImpl},
		{models.StatusPlanning, models.StatusCanceled, impl, otherArch},
		{models.StatusInProgress, models.StatusDone, impl, otherArch},
		{models.StatusInProgress, models.StatusCanceled, arch, otherImpl},
		{models.StatusInProgress, models.StatusCanceled, impl, otherArch},
		{models.StatusDone, models.StatusVerified, arch, otherImpl}, // arch, different session
		{models.StatusDone, models.StatusInProgress, arch, otherImpl},
		{models.StatusCanceled, models.StatusTodo, arch, otherImpl},
		{models.StatusCanceled, models.StatusTodo, impl, otherArch},
	}

	for _, tt := range tests {
		err := ValidateTransition(tt.from, tt.to, tt.actor, tt.submitter, false)
		if err != nil {
			t.Errorf("ValidateTransition(%s→%s, role=%s) unexpected error: %v",
				tt.from, tt.to, tt.actor.Role, err)
		}
	}
}

func TestValidateTransition_InvalidPair(t *testing.T) {
	actor := sess("arch-alice-1111", models.RoleArchitect)
	err := ValidateTransition(models.StatusTodo, models.StatusDone, actor, nil, false)
	if err == nil {
		t.Fatal("expected error for invalid TODO→DONE transition, got nil")
	}
}

func TestValidateTransition_RoleViolation(t *testing.T) {
	// IN_PROGRESS → DONE requires implementer; supply architect.
	arch := sess("arch-alice-1111", models.RoleArchitect)
	impl := sess("impl-bob-2222", models.RoleImplementer)

	// Without force: hard error.
	err := ValidateTransition(models.StatusInProgress, models.StatusDone, arch, impl, false)
	if err == nil {
		t.Fatal("expected role violation error, got nil")
	}
	// Must not be a ForceWarning.
	var fw *ForceWarning
	if errors.As(err, &fw) {
		t.Fatalf("expected hard error, got ForceWarning: %v", err)
	}

	// With force: ForceWarning, not a hard error.
	err = ValidateTransition(models.StatusInProgress, models.StatusDone, arch, impl, true)
	if err == nil {
		t.Fatal("expected ForceWarning, got nil")
	}
	if !errors.As(err, &fw) {
		t.Fatalf("expected ForceWarning, got %T: %v", err, err)
	}
}

func TestValidateTransition_IsolationViolation(t *testing.T) {
	// DONE → VERIFIED requires different session; supply same ID.
	arch := sess("arch-alice-1111", models.RoleArchitect)
	sameSession := sess("arch-alice-1111", models.RoleArchitect) // same ID

	// Without force: hard error.
	err := ValidateTransition(models.StatusDone, models.StatusVerified, arch, sameSession, false)
	if err == nil {
		t.Fatal("expected isolation violation error, got nil")
	}
	var fw *ForceWarning
	if errors.As(err, &fw) {
		t.Fatalf("expected hard error, got ForceWarning: %v", err)
	}

	// With force: ForceWarning.
	err = ValidateTransition(models.StatusDone, models.StatusVerified, arch, sameSession, true)
	if err == nil {
		t.Fatal("expected ForceWarning, got nil")
	}
	if !errors.As(err, &fw) {
		t.Fatalf("expected ForceWarning, got %T: %v", err, err)
	}
}

func TestNextState_AllForwardPaths(t *testing.T) {
	tests := []struct {
		from models.Status
		want models.Status
	}{
		{models.StatusTodo, models.StatusPlanning},
		{models.StatusPlanning, models.StatusInProgress},
		{models.StatusInProgress, models.StatusDone},
		{models.StatusDone, models.StatusVerified},
	}
	for _, tt := range tests {
		got, err := NextState(tt.from)
		if err != nil {
			t.Errorf("NextState(%s) unexpected error: %v", tt.from, err)
			continue
		}
		if got != tt.want {
			t.Errorf("NextState(%s) = %s, want %s", tt.from, got, tt.want)
		}
	}
}

func TestNextState_TerminalStates(t *testing.T) {
	for _, s := range []models.Status{models.StatusVerified, models.StatusCanceled} {
		_, err := NextState(s)
		if err == nil {
			t.Errorf("NextState(%s) expected error, got nil", s)
		}
	}
}
