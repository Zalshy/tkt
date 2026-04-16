package state

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zalshy/tkt/internal/models"
)

// Transition defines a valid state machine edge.
// AllowedRoles == nil (or empty) means any role is permitted.
// RequiresDifferentSession == true means the acting session must differ from
// the session that submitted the ticket to its current state.
type Transition struct {
	From                     models.Status
	To                       models.Status
	AllowedRoles             []models.Role // nil/empty = any role
	RequiresDifferentSession bool
}

// transitions is the authoritative state machine table (§5).
// "any role" entries use nil for AllowedRoles — NOT an explicit list of current roles,
// so that adding a third role in the future does not silently break these entries.
var transitions = []Transition{
	{models.StatusTodo, models.StatusPlanning, nil, false},
	{models.StatusTodo, models.StatusCanceled, nil, false},
	{models.StatusPlanning, models.StatusInProgress, []models.Role{models.RoleArchitect}, true},
	{models.StatusPlanning, models.StatusCanceled, nil, false},
	{models.StatusInProgress, models.StatusDone, []models.Role{models.RoleImplementer}, false},
	{models.StatusInProgress, models.StatusCanceled, nil, false},
	{models.StatusDone, models.StatusVerified, []models.Role{models.RoleArchitect}, true},
	{models.StatusDone, models.StatusInProgress, []models.Role{models.RoleArchitect}, false},
	{models.StatusVerified, models.StatusArchived, nil, false},
	{models.StatusCanceled, models.StatusTodo, nil, false},
}

// ForceWarning is returned (instead of a hard error) when force==true and one or more
// validation rules would otherwise block the transition. Callers use errors.As to
// distinguish it from a hard error.
type ForceWarning struct {
	Message string
}

func (w *ForceWarning) Error() string { return w.Message }

// containsRole reports whether r appears in roles.
func containsRole(roles []models.Role, r models.Role) bool {
	for _, role := range roles {
		if role == r {
			return true
		}
	}
	return false
}

// ValidateTransition checks whether the actor may move a ticket from → to.
// submitter is the session that last advanced the ticket (used for isolation checks).
// When force is true, rule violations become a *ForceWarning instead of a hard error.
func ValidateTransition(
	from, to models.Status,
	actor *models.Session,
	submitter *models.Session,
	force bool,
) error {
	// Step 1: find a matching transition.
	var found *Transition
	for i := range transitions {
		if transitions[i].From == from && transitions[i].To == to {
			found = &transitions[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("invalid transition %s → %s", from, to)
	}

	// Step 2: collect violations.
	var violations []string

	// Role check — skipped when AllowedRoles is nil/empty (any role).
	if len(found.AllowedRoles) > 0 && !containsRole(found.AllowedRoles, actor.EffectiveRole) {
		roleStrs := make([]string, len(found.AllowedRoles))
		for i, r := range found.AllowedRoles {
			roleStrs[i] = string(r)
		}
		allowedStr := strings.Join(roleStrs, "/")
		violations = append(violations, fmt.Sprintf(
			"transition %s → %s requires role '%s'\nCurrent session %s has role '%s' (effective: '%s')",
			from, to, allowedStr, actor.ID, actor.Role, actor.EffectiveRole,
		))
	}

	// Isolation check.
	if found.RequiresDifferentSession {
		if submitter == nil {
			violations = append(violations, fmt.Sprintf(
				"transition %s → %s requires a different session than the one that submitted\nSubmitted by: (unknown)  (cannot verify)",
				from, to,
			))
		} else if actor.ID == submitter.ID {
			violations = append(violations, fmt.Sprintf(
				"transition %s → %s requires a different session than the one that submitted\nSubmitted by: %s  (you)",
				from, to, submitter.ID,
			))
		}
	}

	// Step 3: return.
	if len(violations) == 0 {
		return nil
	}
	if force {
		return &ForceWarning{Message: strings.Join(violations, "\n")}
	}
	return errors.New(strings.Join(violations, "; "))
}

// NextState returns the natural forward next state for from.
// For states with a forward path, the forward direction is returned.
// Returns an error for terminal states (VERIFIED, CANCELED) that have no natural next.
func NextState(from models.Status) (models.Status, error) {
	switch from {
	case models.StatusTodo:
		return models.StatusPlanning, nil
	case models.StatusPlanning:
		return models.StatusInProgress, nil
	case models.StatusInProgress:
		return models.StatusDone, nil
	case models.StatusDone:
		return models.StatusVerified, nil
	case models.StatusVerified:
		return models.StatusVerified, fmt.Errorf("no natural next state for %s", from)
	case models.StatusCanceled:
		return models.StatusCanceled, fmt.Errorf("no natural next state for %s", from)
	case models.StatusArchived:
		return models.StatusArchived, fmt.Errorf("no natural next state for %s", from)
	default:
		return models.StatusTodo, fmt.Errorf("unknown status: %s", from)
	}
}
