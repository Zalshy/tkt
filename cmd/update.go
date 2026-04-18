package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/ticket"
)

var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update main_type or attention_level of a ticket",
	Args:  cobra.ExactArgs(1),
	RunE:  runUpdate,
}

func init() {
	updateCmd.Flags().StringP("type",      "t", "", "ticket type label (e.g. feature, bugfix, refactor)")
	updateCmd.Flags().IntP(   "attention", "a",  0, "attention level 0-99 (0 = unset)")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("update: open db: %w", err)
	}
	defer database.Close()

	typeChanged      := cmd.Flags().Changed("type")
	attentionChanged := cmd.Flags().Changed("attention")

	if !typeChanged && !attentionChanged {
		return fmt.Errorf("update: provide at least one of --type or --attention")
	}

	var mainType       *string
	var attentionLevel *int

	if typeChanged {
		v, _ := cmd.Flags().GetString("type")
		mainType = &v
	}
	if attentionChanged {
		v, _ := cmd.Flags().GetInt("attention")
		attentionLevel = &v
	}

	t, err := ticket.Update(args[0], mainType, attentionLevel, database)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "#%d updated\n", t.ID)
	return nil
}
