package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/db"
	ilog "github.com/zalshy/tkt/internal/log"
	"github.com/zalshy/tkt/internal/output"
	"github.com/zalshy/tkt/internal/ticket"
)

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show full details of a ticket",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("show: open db: %w", err)
	}
	defer database.Close()

	t, err := ticket.GetByID(args[0], database)
	if err != nil {
		if errors.Is(err, ticket.ErrNotFound) {
			// Print error ourselves and silence cobra to avoid the "Error: " prefix doubling.
			fmt.Fprintf(os.Stderr, "Error: ticket %s not found\n", args[0])
			cmd.SilenceErrors = true
			return fmt.Errorf("")
		}
		return fmt.Errorf("show: get ticket: %w", err)
	}

	entries, err := ilog.GetAll(args[0], database)
	if err != nil {
		return fmt.Errorf("show: get log: %w", err)
	}

	deps, err := ticket.GetDependencies(t.ID, database)
	if err != nil {
		return fmt.Errorf("show: get dependencies: %w", err)
	}

	out := output.RenderTicket(*t, entries)
	out += output.RenderDependencies(deps)
	fmt.Fprint(cmd.OutOrStdout(), out)
	return nil
}
