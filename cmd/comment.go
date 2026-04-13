package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/db"
	tktlog "github.com/zalshy/tkt/internal/log"
	"github.com/zalshy/tkt/internal/session"
	"github.com/zalshy/tkt/internal/ticket"
)

var commentCmd = &cobra.Command{
	Use:   "comment <id[,id...]> \"<body>\"",
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

	ids, err := parseIDs(args[0])
	if err != nil {
		return fmt.Errorf("comment: %w", err)
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

	out := cmd.OutOrStdout()
	var errs []string

	for _, ticketID := range ids {
		t, err := ticket.GetByID(ticketID, database)
		if err != nil {
			errs = append(errs, fmt.Sprintf("#%s: %v", ticketID, err))
			continue
		}

		if err := tktlog.Append(t.ID, "message", body, nil, nil, sess, database); err != nil {
			errs = append(errs, fmt.Sprintf("#%s: %v", ticketID, err))
			continue
		}

		fmt.Fprintf(out, "#%d  %s\n%q\n", t.ID, sess.ID, body)
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
