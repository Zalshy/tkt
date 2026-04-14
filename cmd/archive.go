package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/session"
	"github.com/zalshy/tkt/internal/state"
	"github.com/zalshy/tkt/internal/ticket"
)

var archiveCmd = &cobra.Command{
	Use:   "archive <ticket-id[,id...]>",
	Short: "Archive one or more VERIFIED tickets",
	Args:  cobra.ExactArgs(1),
	RunE:  runArchive,
}

func init() {
	rootCmd.AddCommand(archiveCmd)
}

func runArchive(cmd *cobra.Command, args []string) error {
	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("archive: open db: %w", err)
	}
	defer database.Close()

	sess, err := session.LoadActive(root, database)
	if err != nil {
		if errors.Is(err, session.ErrNoSession) {
			return fmt.Errorf(msgNoSession)
		}
		return fmt.Errorf("archive: load session: %w", err)
	}

	rawIDs := strings.Split(args[0], ",")
	out := cmd.OutOrStdout()

	var errs []string
	for _, raw := range rawIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}

		t, err := ticket.GetByID(id, database)
		if err != nil {
			errs = append(errs, fmt.Sprintf("#%s: %v", id, err))
			continue
		}

		fromStatus := t.Status

		if err := state.Execute(id, models.StatusArchived, "archived", sess, database, false); err != nil {
			errs = append(errs, fmt.Sprintf("#%s: %v", id, err))
			continue
		}

		numericID := strings.TrimPrefix(id, "#")
		fmt.Fprintf(out, "#%s  %s → ARCHIVED\n", numericID, fromStatus)
	}

	if len(errs) > 0 {
		return fmt.Errorf("archive: %s", strings.Join(errs, "; "))
	}
	return nil
}
