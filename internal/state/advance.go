package state

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	ilog "github.com/zalshy/tkt/internal/log"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/ticket"
)

// Execute orchestrates a full state transition as a single atomic operation.
// It is the ONLY function in the codebase allowed to write tickets.status.
//
// ticketID may have an optional "#" prefix ("42" or "#42").
// to is the target status; if empty (""), NextState determines it automatically.
// note is required by ilog.Append (must be non-empty).
// actor is the session performing the transition.
// force bypasses soft validation rules, converting hard errors into a ForceWarning.
func Execute(ticketID string, to models.Status, note string, actor *models.Session, db *sql.DB, force bool) error {
	// Step 1: strip "#" prefix and load the current ticket.
	normalizedID := strings.TrimPrefix(ticketID, "#")
	t, err := ticket.GetByID(normalizedID, db)
	if err != nil {
		return fmt.Errorf("state.Execute: load ticket: %w", err)
	}

	// Step 2: resolve the submitter from the last transition log entry.
	entries, err := ilog.GetAll(normalizedID, db)
	if err != nil {
		return fmt.Errorf("state.Execute: get log: %w", err)
	}
	var submitterID string
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Kind == "transition" {
			submitterID = entries[i].SessionID
			break
		}
	}
	if submitterID == "" {
		submitterID = t.CreatedBy
	}
	submitter := &models.Session{ID: submitterID}

	// Step 3: resolve target state when to is zero value.
	if to == "" {
		to, err = NextState(t.Status)
		if err != nil {
			return fmt.Errorf("state.Execute: next state: %w", err)
		}
	}

	// Step 4: validate the transition.
	var warn *ForceWarning
	if err := ValidateTransition(t.Status, to, actor, submitter, force); err != nil {
		if !errors.As(err, &warn) {
			return fmt.Errorf("state.Execute: %w", err)
		}
		// ForceWarning: store and continue.
	}

	// Step 5: plan guard — PLANNING → IN_PROGRESS requires a submitted plan.
	if t.Status == models.StatusPlanning && to == models.StatusInProgress {
		plan, err := ilog.LatestPlan(normalizedID, db)
		if err != nil {
			return fmt.Errorf("state.Execute: check plan: %w", err)
		}
		if plan == nil {
			return fmt.Errorf("plan required: no plan has been submitted for ticket #%s", normalizedID)
		}
	}

	// Step 6: emit ForceWarning to stderr (after plan guard so we don't warn then bail).
	if warn != nil {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", warn.Message)
	}

	// Step 7: parse the integer ticket ID needed by ilog.Append.
	parsedIntID, err := strconv.ParseInt(normalizedID, 10, 64)
	if err != nil {
		return fmt.Errorf("state.Execute: parse ticket id: %w", err)
	}

	// Step 8: open transaction and write atomically.
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("state.Execute: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit; intentional

	// TOCTOU guard: bind current status in WHERE clause so a concurrent writer is caught.
	result, err := tx.Exec(
		`UPDATE tickets SET status = ?, updated_at = datetime('now') WHERE id = ? AND status = ?`,
		string(to), parsedIntID, string(t.Status),
	)
	if err != nil {
		return fmt.Errorf("state.Execute: update ticket: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("state.Execute: rows affected: %w", err)
	}
	if n != 1 {
		return fmt.Errorf("concurrent modification: ticket status changed before write")
	}

	// Step 9: append the transition log entry inside the same transaction.
	fromStr := string(t.Status)
	toStr := string(to)
	if err := ilog.Append(parsedIntID, "transition", note, &fromStr, &toStr, actor, tx); err != nil {
		return fmt.Errorf("state.Execute: log append: %w", err)
	}

	// Step 10: commit — rollback fires on any earlier failure via defer.
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("state.Execute: commit: %w", err)
	}
	return nil
}
