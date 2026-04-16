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

var newDescription string
var newAfter string
var newTier string

var newCmd = &cobra.Command{
	Use:   "new \"<title>\"",
	Short: "Create a new ticket",
	Args:  cobra.ExactArgs(1),
	RunE:  runNew,
}

func init() {
	newCmd.Flags().StringVar(&newDescription, "description", "", "ticket description")
	newCmd.Flags().StringVar(&newAfter, "after", "", "comma-separated dependency ticket IDs (e.g. 5,7)")
	newCmd.Flags().StringVar(&newTier, "tier", "standard", "ticket tier (critical, standard, low)")
	rootCmd.AddCommand(newCmd)
}

func runNew(cmd *cobra.Command, args []string) error {
	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("new: open db: %w", err)
	}
	defer database.Close()

	sess, err := session.LoadActive(root, database)
	if err != nil {
		if errors.Is(err, session.ErrNoSession) {
			return fmt.Errorf(msgNoSession)
		}
		return fmt.Errorf("new: load session: %w", err)
	}

	t, err := ticket.Create(args[0], newDescription, newTier, sess, database)
	if err != nil {
		return fmt.Errorf("new: create: %w", err)
	}

	var depIDs []int64
	if newAfter != "" {
		parts, err := parseIDs(newAfter)
		if err != nil {
			return fmt.Errorf("new: --after: %w", err)
		}
		for _, raw := range parts {
			n, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return fmt.Errorf("new: --after: %q is not a valid ticket ID. Use comma-separated integers (e.g. --after 5,7)", raw)
			}
			depIDs = append(depIDs, n)
		}

		if err := ticket.AddDependencies(t.ID, depIDs, database); err != nil {
			return fmt.Errorf("new: --after: %w\nTicket #%d was created. Delete it with: tkt cancel #%d", err, t.ID, t.ID)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created #%d  %q\n", t.ID, t.Title)
	if len(depIDs) > 0 {
		depStrs := make([]string, len(depIDs))
		for i, id := range depIDs {
			depStrs[i] = fmt.Sprintf("#%d", id)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Depends on: %s\n", strings.Join(depStrs, ", "))
	}
	return nil
}
