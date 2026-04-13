package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/project"
	"github.com/zalshy/tkt/internal/role"
	"github.com/zalshy/tkt/internal/session"
)

var (
	sessionRole string
	sessionEnd  bool
	sessionName string
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Declare or inspect the active session",
	Long: `Without flags: show the current session.
With --role: create a new session as architect or implementer.
With --end: mark the current session as expired.`,
	Args: cobra.NoArgs,
	RunE: runSession,
}

func init() {
	sessionCmd.Flags().StringVar(&sessionRole, "role", "", "role for the new session: a registered role (e.g. architect or implementer)")
	sessionCmd.Flags().BoolVar(&sessionEnd, "end", false, "mark the current session as expired")
	sessionCmd.Flags().StringVar(&sessionName, "name", "", "explicit session name (requires --role); lowercase alphanumeric + hyphens, max 32 chars")
	sessionCmd.MarkFlagsMutuallyExclusive("role", "end")

	rootCmd.AddCommand(sessionCmd)
}

func runSession(cmd *cobra.Command, args []string) error {
	// --name requires --role; reject early before any DB work.
	if sessionName != "" && sessionRole == "" {
		return fmt.Errorf("--name requires --role")
	}
	switch {
	case sessionRole != "":
		return runSessionCreate(cmd)
	case sessionEnd:
		return runSessionEnd(cmd)
	default:
		return runSessionShow(cmd)
	}
}

// runSessionCreate handles `tkt session --role <role> [--name <name>]`.
func runSessionCreate(cmd *cobra.Command) error {
	// Validate --name before opening the DB.
	if sessionName != "" {
		if err := session.ValidateName(sessionName); err != nil {
			return fmt.Errorf("session: invalid name: %w", err)
		}
	}

	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("session: open db: %w", err)
	}
	defer database.Close()

	ok, err := role.Exists(sessionRole, database)
	if err != nil {
		return fmt.Errorf("session: role check: %w", err)
	}
	if !ok {
		return fmt.Errorf("invalid role %q — not a registered role", sessionRole)
	}

	s, err := session.Create(models.Role(sessionRole), sessionName, database, root)
	if err != nil {
		return fmt.Errorf("session: create: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Session created: %s\nRole: %s\n", s.ID, s.Role)
	return nil
}

// runSessionEnd handles `tkt session --end`.
func runSessionEnd(cmd *cobra.Command) error {
	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("session: open db: %w", err)
	}
	defer database.Close()

	id, err := session.End(root, database)
	if err != nil {
		if errors.Is(err, session.ErrNoSession) {
			return fmt.Errorf("no active session")
		}
		return fmt.Errorf("session: end: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Session %s ended.\n", id)
	return nil
}

// runSessionShow handles `tkt session` (no flags).
func runSessionShow(cmd *cobra.Command) error {
	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("session: open db: %w", err)
	}
	defer database.Close()

	s, err := session.LoadActive(root, database)
	if err != nil {
		if errors.Is(err, session.ErrNoSession) {
			// This is a help message, not a command error. We print to stdout then
			// return a sentinel empty error. cobra treats any non-nil error from RunE
			// as exit 1. Setting SilenceErrors suppresses the redundant "Error: "
			// prefix that cobra would otherwise prepend to the empty string.
			fmt.Fprintln(cmd.OutOrStdout(), "No active session. Run: tkt session --role architect")
			fmt.Fprintln(cmd.OutOrStdout(), "                   or: tkt session --role implementer")
			cmd.SilenceErrors = true
			return fmt.Errorf("")
		}
		if errors.Is(err, session.ErrExpiredSession) {
			// LoadActive returns nil for the session on expiry, so re-read the file
			// to get the session ID for the error message.
			data, readErr := os.ReadFile(project.SessionFile(root))
			id := "unknown"
			if readErr == nil {
				id = strings.TrimSpace(string(data))
			}
			return fmt.Errorf("session %s has expired.\nRun: tkt session --role architect\n  or: tkt session --role implementer", id)
		}
		return fmt.Errorf("session: load: %w", err)
	}

	// Column-align all labels to 13 chars (including the colon) so values line up at column 15.
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%-13s %s\n", "Session:", s.ID)
	fmt.Fprintf(out, "%-13s %s\n", "Role:", string(s.Role))
	fmt.Fprintf(out, "%-13s %s\n", "Status:", "active")
	fmt.Fprintf(out, "%-13s %s\n", "Active since:", s.CreatedAt.Format("2006-01-02 15:04:05"))
	return nil
}
