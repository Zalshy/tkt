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
	advanceNote      string
	advanceNoteFile  string
	advanceNoteStdin bool
	advanceTo        string
	advanceForce     bool
	advanceDryRun    bool
	advanceExplain   bool
)

var advanceCmd = &cobra.Command{
	Use:   "advance <id[,id...]>",
	Short: "Advance a ticket to the next state",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdvance,
}

func init() {
	advanceCmd.Flags().StringVar(&advanceNote, "note", "", "required note for the transition (non-empty)")
	advanceCmd.Flags().StringVar(&advanceNoteFile, "note-file", "", "read transition note from file")
	advanceCmd.Flags().BoolVar(&advanceNoteStdin, "note-stdin", false, "read transition note from stdin")
	advanceCmd.Flags().StringVar(&advanceTo, "to", "", "target state (TODO, PLANNING, IN_PROGRESS, DONE, VERIFIED, CANCELED, ARCHIVED); default: natural next state")
	advanceCmd.Flags().BoolVar(&advanceForce, "force", false, "override role/isolation checks (violation will be recorded)")
	advanceCmd.Flags().BoolVar(&advanceDryRun, "dry-run", false, "check transition without changing state or writing log")
	advanceCmd.Flags().BoolVar(&advanceExplain, "explain", false, "explain why transition is allowed or blocked without changing state")
	rootCmd.AddCommand(advanceCmd)
}

func runAdvance(cmd *cobra.Command, args []string) error {
	if advanceDryRun && advanceExplain {
		return fmt.Errorf("advance: --dry-run and --explain cannot be used together")
	}
	note, _, err := readTextInput(cmd, textInputOptions{
		Prefix:         "advance",
		FieldName:      "note",
		InlineFlagName: "note",
		InlineValue:    advanceNote,
		StdinFlagName:  "note-stdin",
		UseStdin:       advanceNoteStdin,
		FileFlagName:   "note-file",
		FilePath:       advanceNoteFile,
		Required:       !advanceDryRun && !advanceExplain,
	})
	if err != nil {
		if err.Error() == "note is required" {
			return fmt.Errorf("flag --note is required and must be non-empty")
		}
		return err
	}

	ids, err := parseIDs(args[0])
	if err != nil {
		return fmt.Errorf("advance: %w", err)
	}

	// Validate --to before touching the DB.
	var toStatus models.Status
	if advanceTo != "" {
		if !validStatuses[advanceTo] {
			return fmt.Errorf("invalid --to %q: must be one of TODO, PLANNING, IN_PROGRESS, DONE, VERIFIED, CANCELED, ARCHIVED", advanceTo)
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

	out := cmd.OutOrStdout()
	var errs []string

	for _, ticketID := range ids {
		// Load the ticket to capture the from-state before Execute.
		t, err := ticket.GetByID(ticketID, database)
		if err != nil {
			errs = append(errs, fmt.Sprintf("#%s: %v", ticketID, err))
			continue
		}

		fromStatus := t.Status

		// Resolve the target state for output (mirrors what Execute does internally).
		displayTo := toStatus
		if displayTo == "" {
			displayTo, err = state.NextState(fromStatus)
			if err != nil {
				errs = append(errs, fmt.Sprintf("#%s: %v", ticketID, err))
				continue
			}
		}

		if advanceDryRun || advanceExplain {
			check, err := state.Check(ticketID, toStatus, sess, database, advanceForce)
			if err != nil {
				errs = append(errs, fmt.Sprintf("#%s: %v", ticketID, err))
				continue
			}
			writeAdvanceCheck(out, check, advanceExplain)
			if !check.Allowed {
				errs = append(errs, fmt.Sprintf("#%s: %s", ticketID, check.Reason))
			}
			continue
		}

		// Execute the transition.
		if err := state.Execute(ticketID, toStatus, note, sess, database, advanceForce); err != nil {
			errText := err.Error()
			if !advanceForce && (strings.Contains(errText, "requires role") || strings.Contains(errText, "requires a different session")) {
				errs = append(errs, fmt.Sprintf("#%s: %s\nUse --force to override (violation will be recorded)", ticketID, errText))
				continue
			}
			errs = append(errs, fmt.Sprintf("#%s: %v", ticketID, err))
			continue
		}

		// Success — print the transition output.
		numericID := strings.TrimPrefix(ticketID, "#")
		fmt.Fprintf(out, "#%s  %s → %s\n", numericID, fromStatus, displayTo)
		fmt.Fprintf(out, "Session: %s\n", sess.ID)
		fmt.Fprintf(out, "Note: %q\n", note)

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
	}

	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "%d error(s):\n", len(errs))
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  %s\n", e)
		}
		cmd.SilenceErrors = true
		return fmt.Errorf("")
	}

	return nil
}

func writeAdvanceCheck(out interface{ Write([]byte) (int, error) }, check state.CheckResult, explain bool) {
	status := "would advance"
	if !check.Allowed {
		status = "blocked"
	}
	fmt.Fprintf(out, "#%s  %s → %s  %s\n", check.TicketID, check.From, check.To, status)
	if !explain {
		if !check.Allowed {
			fmt.Fprintf(out, "Reason: %s\n", check.Reason)
		}
		return
	}
	fmt.Fprintf(out, "Allowed: %t\n", check.Allowed)
	fmt.Fprintf(out, "Forced: %t\n", check.Forced)
	fmt.Fprintf(out, "Reason: %s\n", check.Reason)
	if check.PlanRequired {
		fmt.Fprintf(out, "Plan required: true\n")
		fmt.Fprintf(out, "Plan present: %t\n", check.PlanPresent)
	}
	if len(check.Hints) > 0 {
		fmt.Fprintf(out, "See: %s\n", strings.Join(check.Hints, ", "))
	}
}

func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return singular
	}
	return pluralForm
}
