package state

import (
	"context"
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

type CheckResult struct {
	TicketID     string
	From         models.Status
	To           models.Status
	Allowed      bool
	Forced       bool
	Reason       string
	PlanRequired bool
	PlanPresent  bool
	Hints        []string
}

type transitionPreflight struct {
	result CheckResult
	ticket *models.Ticket
	warn   *ForceWarning
}

func Check(ticketID string, to models.Status, actor *models.Session, db *sql.DB, force bool) (CheckResult, error) {
	pre, err := preflight(ticketID, to, actor, db, force)
	if err != nil {
		return CheckResult{}, err
	}
	return pre.result, nil
}

func preflight(ticketID string, to models.Status, actor *models.Session, db *sql.DB, force bool) (transitionPreflight, error) {
	normalizedID := strings.TrimPrefix(ticketID, "#")
	t, err := ticket.GetByID(normalizedID, db)
	if err != nil {
		return transitionPreflight{}, fmt.Errorf("state.Check: load ticket: %w", err)
	}

	entries, err := ilog.GetAll(context.Background(), normalizedID, db)
	if err != nil {
		return transitionPreflight{}, fmt.Errorf("state.Check: get log: %w", err)
	}
	var submitterName string
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Kind == "transition" {
			submitterName = entries[i].SessionName
			break
		}
	}
	if submitterName == "" {
		submitterName = t.CreatedBy
	}
	submitter := &models.Session{Name: submitterName}

	if to == "" {
		to, err = NextState(t.Status)
		if err != nil {
			return transitionPreflight{}, fmt.Errorf("state.Check: next state: %w", err)
		}
	}

	result := CheckResult{
		TicketID: normalizedID,
		From:     t.Status,
		To:       to,
		Allowed:  true,
		Forced:   force,
		Hints:    []string{"tkt man advance", "tkt man state-machine"},
	}

	var warn *ForceWarning
	if err := ValidateTransition(t.Status, to, actor, submitter, force); err != nil {
		if errors.As(err, &warn) {
			result.Reason = warn.Message
		} else {
			result.Allowed = false
			result.Reason = err.Error()
			return transitionPreflight{result: result, ticket: t}, nil
		}
	}

	if t.Status == models.StatusPlanning && to == models.StatusInProgress {
		result.PlanRequired = true
		plan, err := ilog.LatestPlan(context.Background(), normalizedID, db)
		if err != nil {
			return transitionPreflight{}, fmt.Errorf("state.Check: check plan: %w", err)
		}
		result.PlanPresent = plan != nil
		if plan == nil {
			result.Allowed = false
			result.Reason = fmt.Sprintf("plan required: no plan has been submitted for ticket #%s", normalizedID)
			result.Hints = []string{"tkt man plan", "tkt man advance", "tkt man state-machine"}
			return transitionPreflight{result: result, ticket: t, warn: warn}, nil
		}
	}

	if result.Reason == "" {
		result.Reason = "transition allowed"
	}
	return transitionPreflight{result: result, ticket: t, warn: warn}, nil
}

// Execute orchestrates a full state transition as a single atomic operation.
// It is the ONLY function in the codebase allowed to write tickets.status.
//
// ticketID may have an optional "#" prefix ("42" or "#42").
// to is the target status; if empty (""), NextState determines it automatically.
// note is required by ilog.Append (must be non-empty).
// actor is the session performing the transition.
// force bypasses soft validation rules, converting hard errors into a ForceWarning.
func Execute(ticketID string, to models.Status, note string, actor *models.Session, db *sql.DB, force bool) error {
	pre, err := preflight(ticketID, to, actor, db, force)
	if err != nil {
		return errors.New(strings.Replace(err.Error(), "state.Check", "state.Execute", 1))
	}
	if !pre.result.Allowed {
		return fmt.Errorf("state.Execute: %s", pre.result.Reason)
	}
	normalizedID := pre.result.TicketID
	t := pre.ticket
	to = pre.result.To

	// Step 6: emit ForceWarning to stderr (after plan guard so we don't warn then bail).
	if pre.warn != nil {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", pre.warn.Message)
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
	// The deleted_at IS NULL guard prevents status changes on soft-deleted tickets.
	result, err := tx.Exec(
		`UPDATE tickets SET status = ?, updated_at = datetime('now') WHERE id = ? AND status = ? AND deleted_at IS NULL`,
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
		return fmt.Errorf("advance: ticket %v not found, already deleted, or status mismatch", parsedIntID)
	}

	// Step 9: append the transition log entry inside the same transaction.
	// Forced transitions must persist the bypassed validation details in audit
	// history, not just print them to stderr.
	logBody := note
	if pre.warn != nil {
		logBody = note + "\n\nFORCE VIOLATION:\n" + pre.warn.Message
	}
	fromStr := string(t.Status)
	toStr := string(to)
	if err := ilog.Append(context.Background(), parsedIntID, "transition", logBody, &fromStr, &toStr, actor, tx); err != nil {
		return fmt.Errorf("state.Execute: log append: %w", err)
	}

	// Step 10: commit — rollback fires on any earlier failure via defer.
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("state.Execute: commit: %w", err)
	}
	return nil
}
