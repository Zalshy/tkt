package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/output"
	"github.com/zalshy/tkt/internal/ticket"
)

var (
	searchTitleOnly bool
	searchAll       bool
	searchStatus    string
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Full-text search across ticket titles and descriptions",
	Args:  cobra.ExactArgs(1),
	RunE:  runSearch,
}

func init() {
	searchCmd.Flags().BoolVar(&searchTitleOnly, "title", false, "restrict search to title only")
	searchCmd.Flags().BoolVar(&searchAll, "all", false, "include CANCELED and ARCHIVED tickets in results")
	searchCmd.Flags().StringVar(&searchStatus, "status", "", "filter results to a specific status (TODO, PLANNING, IN_PROGRESS, DONE, VERIFIED, CANCELED)")
	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := args[0]

	// Validate --status before touching the DB.
	if searchStatus != "" && !validStatuses[searchStatus] {
		return fmt.Errorf("invalid --status %q: must be one of TODO, PLANNING, IN_PROGRESS, DONE, VERIFIED, CANCELED", searchStatus)
	}

	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("search: open db: %w", err)
	}
	defer database.Close()

	opts := ticket.ListOptions{
		Query:           query,
		TitleOnly:       searchTitleOnly,
		All:             false, // never set true — keeps deleted_at IS NULL unconditional
		IncludeVerified: true,  // search should surface all statuses by default
		IncludeArchived: true,
		ExcludeCanceled: !searchAll,
		Sort:            "id",
	}

	if searchStatus != "" {
		s := models.Status(searchStatus)
		opts.Status = &s
		opts.IncludeVerified = true // explicit status overrides default hiding
	}

	result, err := ticket.List(opts, database)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	out := cmd.OutOrStdout()

	if len(result.Tickets) == 0 {
		fmt.Fprintf(out, "(no tickets matched %q)\n", query)
		return nil
	}

	fmt.Fprint(out, output.RenderList(result.Tickets, result.HasMore, 0))
	fmt.Fprintln(out)
	fmt.Fprintf(out, "\n%d tickets matched %q.\n", len(result.Tickets), query)

	return nil
}
