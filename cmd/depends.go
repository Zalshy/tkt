package cmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/session"
	"github.com/zalshy/tkt/internal/ticket"
)

var (
	dependsOn     string
	dependsRemove string
)

var dependsCmd = &cobra.Command{
	Use:   "depends <ticket-id>",
	Short: "Declare or remove ticket dependencies",
	Args:  cobra.ExactArgs(1),
	RunE:  runDepends,
}

func init() {
	dependsCmd.Flags().StringVar(&dependsOn, "on", "", "comma-separated list of ticket IDs this ticket depends on")
	dependsCmd.Flags().StringVar(&dependsRemove, "remove", "", "ticket ID of the dependency to remove")
	rootCmd.AddCommand(dependsCmd)
}

func runDepends(cmd *cobra.Command, args []string) error {
	// 1. Flag mutual exclusion — exactly one of --on / --remove must be set.
	if dependsOn == "" && dependsRemove == "" {
		return fmt.Errorf("one of --on or --remove is required")
	}
	if dependsOn != "" && dependsRemove != "" {
		return fmt.Errorf("--on and --remove are mutually exclusive")
	}

	// 2. Open DB and load session.
	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("depends: open db: %w", err)
	}
	defer database.Close()

	_, err = session.LoadActive(root, database)
	if err != nil {
		if errors.Is(err, session.ErrNoSession) {
			return fmt.Errorf("no active session. Run: tkt session --role architect\n           or: tkt session --role implementer")
		}
		return fmt.Errorf("depends: load session: %w", err)
	}

	// 3. Parse the ticket ID argument (strip # prefix).
	rawID := strings.TrimPrefix(args[0], "#")
	ticketID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return fmt.Errorf("depends: invalid ticket id %q: must be a number", args[0])
	}

	// 4. Validate that the ticket exists.
	if _, err := ticket.GetByID(rawID, database); err != nil {
		return fmt.Errorf("depends: %w", err)
	}

	// 5. --on path.
	if dependsOn != "" {
		parts := strings.Split(dependsOn, ",")
		var depIDs []int64

		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			trimmed = strings.TrimPrefix(trimmed, "#")
			if trimmed == "" {
				continue
			}
			depID, err := strconv.ParseInt(trimmed, 10, 64)
			if err != nil {
				return fmt.Errorf("depends: invalid dependency id %q: must be a number", part)
			}
			depIDs = append(depIDs, depID)
		}

		if err := ticket.AddDependencies(ticketID, depIDs, database); err != nil {
			return fmt.Errorf("depends: %w", err)
		}

		// 5f. Print result.
		var depStrs []string
		for _, depID := range depIDs {
			depStrs = append(depStrs, fmt.Sprintf("#%d", depID))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "#%d now depends on %s\n", ticketID, strings.Join(depStrs, ", "))
		return nil
	}

	// 6. --remove path.
	rawRemove := strings.TrimPrefix(dependsRemove, "#")
	removeID, err := strconv.ParseInt(rawRemove, 10, 64)
	if err != nil {
		return fmt.Errorf("depends: invalid --remove id %q: must be a number", dependsRemove)
	}

	if err := ticket.RemoveDependency(ticketID, removeID, database); err != nil {
		return fmt.Errorf("depends: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Removed dependency: #%d no longer depends on #%d\n", ticketID, removeID)
	return nil
}
