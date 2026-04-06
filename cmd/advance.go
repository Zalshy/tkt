package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/session"
	"github.com/zalshy/tkt/internal/state"
	"github.com/zalshy/tkt/internal/ticket"
)

var (
	advanceNote  string
	advanceTo    string
	advanceForce bool
)

var advanceCmd = &cobra.Command{
	Use:   "advance <ticket-id>",
	Short: "Advance a ticket to the next state",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdvance,
}

func init() {
	advanceCmd.Flags().StringVar(&advanceNote, "note", "", "required note for the transition (non-empty)")
	advanceCmd.Flags().StringVar(&advanceTo, "to", "", "target state (TODO, PLANNING, IN_PROGRESS, DONE, VERIFIED, CANCELED); default: natural next state")
	advanceCmd.Flags().BoolVar(&advanceForce, "force", false, "override role/isolation checks (violation will be recorded)")
	rootCmd.AddCommand(advanceCmd)
}

func runAdvance(cmd *cobra.Command, args []string) error {
	// --note is required and must be non-empty.
	if advanceNote == "" {
		return fmt.Errorf("flag --note is required and must be non-empty")
	}

	ticketID := args[0]

	// Validate --to before touching the DB.
	var toStatus models.Status
	if advanceTo != "" {
		if !validStatuses[advanceTo] {
			return fmt.Errorf("invalid --to %q: must be one of TODO, PLANNING, IN_PROGRESS, DONE, VERIFIED, CANCELED", advanceTo)
		}
		toStatus = models.Status(advanceTo)
	}

	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("advance: open db: %w", err)
	}
	defer database.Close()

	sess, err := session.LoadActive(root, database)
	if err != nil {
		if errors.Is(err, session.ErrNoSession) {
			return fmt.Errorf(msgNoSession)
		}
		return fmt.Errorf("advance: load session: %w", err)
	}

	// Load the ticket to capture the from-state before Execute.
	t, err := ticket.GetByID(ticketID, database)
	if err != nil {
		return fmt.Errorf("advance: %w", err)
	}

	fromStatus := t.Status

	// Resolve the target state for output (mirrors what Execute does internally).
	displayTo := toStatus
	if displayTo == "" {
		displayTo, err = state.NextState(fromStatus)
		if err != nil {
			return fmt.Errorf("advance: %w", err)
		}
	}

	// Execute the transition.
	if err := state.Execute(ticketID, toStatus, advanceNote, sess, database, advanceForce); err != nil {
		// Soft-violation errors (role / isolation) must be printed with the exact §7
		// format including the "Use --force" suffix. Detect by known substrings.
		errText := err.Error()
		if !advanceForce && (strings.Contains(errText, "requires role") || strings.Contains(errText, "requires a different session")) {
			fmt.Fprintf(os.Stderr, "Error: %s\nUse --force to override (violation will be recorded)\n", errText)
			cmd.SilenceErrors = true
			return fmt.Errorf("")
		}
		return fmt.Errorf("advance: %w", err)
	}

	// Success — print the §7 transition output.
	numericID := strings.TrimPrefix(ticketID, "#")
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "#%s  %s → %s\n", numericID, fromStatus, displayTo)
	fmt.Fprintf(out, "Session: %s\n", sess.ID)
	fmt.Fprintf(out, "Note: %q\n", advanceNote)

	// Advisory dependency warning — informational only, never blocks.
	numericIDInt, _ := strconv.ParseInt(numericID, 10, 64)
	deps, err := ticket.GetDependencies(numericIDInt, database)
	if err == nil {
		var unresolved []models.Ticket
		for _, d := range deps {
			if d.Status != models.StatusVerified {
				unresolved = append(unresolved, d)
			}
		}
		if len(unresolved) > 0 {
			fmt.Fprintf(out, "\nWarning: #%s has %d unresolved %s\n",
				numericID, len(unresolved), plural(len(unresolved), "dependency", "dependencies"))
			for _, d := range unresolved {
				fmt.Fprintf(out, "  ○ #%d   %s\n", d.ID, d.Status)
			}
			fmt.Fprintf(out, "\nTransition recorded. Resolve dependencies before implementation begins.\n")
		}
	}

	return nil
}

func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return singular
	}
	return pluralForm
}
