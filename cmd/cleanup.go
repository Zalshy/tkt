package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/db"
)

var cleanupDryRun bool

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Expire stale sessions and run maintenance",
	RunE:  runCleanup,
}

func init() {
	cleanupCmd.Flags().BoolVar(&cleanupDryRun, "dry-run", false, "print what would be affected without writing")
	rootCmd.AddCommand(cleanupCmd)
}

func runCleanup(cmd *cobra.Command, args []string) error {
	root, err := requireRoot()
	if err != nil {
		return err
	}
	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("cleanup: open db: %w", err)
	}
	defer database.Close()

	n, err := db.CleanupStaleSessions(database, cleanupDryRun)
	if err != nil {
		return fmt.Errorf("cleanup: %w", err)
	}

	switch {
	case n == 0:
		fmt.Fprintln(cmd.OutOrStdout(), "Nothing to clean up.")
	case cleanupDryRun:
		fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Would expire %d stale session(s) (last active > 48h ago).\n", n)
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "Expired %d stale session(s) (last active > 48h ago).\n", n)
	}
	return nil
}
