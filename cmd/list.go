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
	listStatus   string
	listLimit    int
	listAll      bool
	listVerified bool
	listSort     string
	listReady    bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List tickets",
	Args:  cobra.NoArgs,
	RunE:  runList,
}

func init() {
	listCmd.Flags().StringVar(&listStatus, "status", "", "filter by status (TODO, PLANNING, IN_PROGRESS, DONE, VERIFIED, CANCELED)")
	listCmd.Flags().IntVar(&listLimit, "limit", 10, "maximum number of tickets to show")
	listCmd.Flags().BoolVar(&listAll, "all", false, "show all tickets including CANCELED and soft-deleted")
	listCmd.Flags().BoolVar(&listVerified, "verified", false, "include VERIFIED tickets")
	listCmd.Flags().StringVar(&listSort, "sort", "updated", "sort order: updated or id")
	listCmd.Flags().BoolVar(&listReady, "ready", false, "show only tickets with no unresolved dependencies")
	rootCmd.AddCommand(listCmd)
}

var validStatuses = map[string]bool{
	"TODO": true, "PLANNING": true, "IN_PROGRESS": true,
	"DONE": true, "VERIFIED": true, "CANCELED": true,
}

func runList(cmd *cobra.Command, args []string) error {
	// Validate flags before touching the DB.
	if listStatus != "" && !validStatuses[listStatus] {
		return fmt.Errorf("invalid --status %q: must be one of TODO, PLANNING, IN_PROGRESS, DONE, VERIFIED, CANCELED", listStatus)
	}
	if listSort != "updated" && listSort != "id" {
		return fmt.Errorf("invalid --sort %q: must be 'updated' or 'id'", listSort)
	}

	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("list: open db: %w", err)
	}
	defer database.Close()

	opts := ticket.ListOptions{
		Limit:           listLimit,
		All:             listAll,
		IncludeVerified: listVerified,
		Sort:            listSort,
		Ready:           listReady,
	}
	if listStatus != "" {
		s := models.Status(listStatus)
		opts.Status = &s
	}

	result, err := ticket.List(opts, database)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}

	if len(result.Tickets) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no tickets) — Run: tkt new \"<title>\" to create one.")
		return nil
	}

	out := cmd.OutOrStdout()
	fmt.Fprint(out, output.RenderList(result.Tickets, result.HasMore, 0))
	fmt.Fprintln(out)

	if listStatus != "" && !result.HasMore {
		fmt.Fprintf(out, "\n%d tickets in %s.\n", len(result.Tickets), listStatus)
	}

	return nil
}
