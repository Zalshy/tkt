package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/project"
)

const version = "0.1.0"

// rootDir is set by the persistent --dir flag and used by all subcommands to
// locate the project root. Empty string means "use cwd / walk upward".
var rootDir string

var rootCmd = &cobra.Command{
	Use:     "tkt",
	Short:   "Project-local ticket CLI",
	Version: version,
}

// Execute runs the root command and exits on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Persistent flag available to every subcommand.
	rootCmd.PersistentFlags().StringVar(&rootDir, "dir", "", "override project root directory (default: cwd or nearest .tkt/ parent)")

	// Register implemented commands.
	rootCmd.AddCommand(initCmd)

}

// requireRoot returns the project root, preferring --dir if set.
// Returns a user-facing error on failure — never calls os.Exit.
// Callers must return this error from RunE; cobra handles the exit.
func requireRoot() (string, error) {
	if rootDir != "" {
		return rootDir, nil
	}
	root, err := project.FindRoot()
	if err != nil {
		return "", fmt.Errorf("not a tkt project. Run: tkt init")
	}
	return root, nil
}

