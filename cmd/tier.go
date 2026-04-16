package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/ticket"
)

var tierCmd = &cobra.Command{
	Use:   "tier <id> <critical|standard|low>",
	Short: "Change the tier of a ticket",
	Args:  cobra.ExactArgs(2),
	RunE:  runTier,
}

func init() {
	rootCmd.AddCommand(tierCmd)
}

func runTier(cmd *cobra.Command, args []string) error {
	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("tier: open db: %w", err)
	}
	defer database.Close()

	newTier := args[1]
	if newTier != "critical" && newTier != "standard" && newTier != "low" {
		return fmt.Errorf("tier: invalid tier %q: must be critical, standard, or low", newTier)
	}

	t, err := ticket.SetTier(args[0], newTier, database)
	if err != nil {
		return fmt.Errorf("tier: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "#%d tier set to %s\n", t.ID, t.Tier)
	return nil
}
