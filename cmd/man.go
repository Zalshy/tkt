package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/man"
)

var manCmd = &cobra.Command{
	Use:   "man [page]",
	Short: "Read built-in manual pages",
	Long:  "List or read built-in tkt manual pages. Start with `tkt man minimal` for a compact human/LLM guide.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runMan,
}

func init() {
	rootCmd.AddCommand(manCmd)
}

func runMan(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	if len(args) == 0 {
		pages, err := man.ListPages()
		if err != nil {
			return err
		}
		fmt.Fprintln(out, "Built-in manual pages:")
		for _, page := range pages {
			fmt.Fprintf(out, "  %-15s  %s\n", page.Name, page.Title)
		}
		fmt.Fprintln(out, "\nStart with: tkt man minimal")
		return nil
	}

	page, err := man.ReadPage(args[0])
	if err != nil {
		return err
	}
	fmt.Fprint(out, page.Body)
	return nil
}
