package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/config"
	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/session"
	"github.com/zalshy/tkt/internal/tui"
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Launch the read-only TUI dashboard",
	Args:  cobra.NoArgs,
	RunE:  runMonitor,
}

func init() {
	rootCmd.AddCommand(monitorCmd)
}

func runMonitor(cmd *cobra.Command, args []string) error {
	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("monitor: open db: %w", err)
	}
	defer database.Close()

	cfg, err := config.LoadProject(root)
	if err != nil {
		return fmt.Errorf("monitor: load config: %w", err)
	}

	monSess, err := session.CreateSystem(models.RoleMonitor, database)
	if err != nil {
		return fmt.Errorf("monitor: create system session: %w", err)
	}
	defer func() {
		if err := session.ExpireByID(monSess.ID, database); err != nil {
			fmt.Fprintf(os.Stderr, "warning: expire monitor session: %v\n", err)
		}
	}()

	model := tui.NewRootModel(database, cfg, root, monSess)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("monitor: tui: %w", err)
	}
	return nil
}
