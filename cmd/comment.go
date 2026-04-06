package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/db"
	tktlog "github.com/zalshy/tkt/internal/log"
	"github.com/zalshy/tkt/internal/session"
	"github.com/zalshy/tkt/internal/ticket"
)

var commentCmd = &cobra.Command{
	Use:   "comment <ticket-id> \"<body>\"",
	Short: "Add a message to a ticket's log",
	Args:  cobra.ExactArgs(2),
	RunE:  runComment,
}

func init() {
	rootCmd.AddCommand(commentCmd)
}

func runComment(cmd *cobra.Command, args []string) error {
	body := args[1]
	if body == "" {
		return fmt.Errorf("comment body is required")
	}

	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("comment: open db: %w", err)
	}
	defer database.Close()

	sess, err := session.LoadActive(root, database)
	if err != nil {
		if errors.Is(err, session.ErrNoSession) {
			return fmt.Errorf(msgNoSession)
		}
		return fmt.Errorf("comment: load session: %w", err)
	}

	t, err := ticket.GetByID(args[0], database)
	if err != nil {
		return err
	}

	if err := tktlog.Append(t.ID, "message", body, nil, nil, sess, database); err != nil {
		return fmt.Errorf("comment: append: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "#%d  %s\n%q\n", t.ID, sess.ID, body)
	return nil
}
