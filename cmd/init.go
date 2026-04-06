package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/config"
	"github.com/zalshy/tkt/internal/db"
)

var initName string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new tkt project in the current directory",
	Long: `Creates a .tkt/ directory with a SQLite database and project config.
Run this once per project. Does not require an active session.`,
	Args: cobra.NoArgs,
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVar(&initName, "name", "", "project name (default: directory basename)")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Determine the target directory.
	dir, err := resolveInitDir()
	if err != nil {
		return err
	}

	tktDir := filepath.Join(dir, ".tkt")

	// Guard: refuse double-init.
	if _, err := os.Stat(tktDir); err == nil {
		return fmt.Errorf(".tkt/ already exists in %s", dir)
	}

	// Create .tkt/ directory.
	if err := os.MkdirAll(tktDir, 0o755); err != nil {
		return fmt.Errorf("create .tkt/: %w", err)
	}

	// Open (create + migrate) the database.
	database, err := db.Open(dir)
	if err != nil {
		return fmt.Errorf("init db: %w", err)
	}
	database.Close()

	// Resolve project name.
	name := initName
	if name == "" {
		name = filepath.Base(dir)
	}

	// Write project config.
	cfg := &config.ProjectConfig{
		Name:      name,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := config.WriteProject(dir, cfg); err != nil {
		return fmt.Errorf("write project config: %w", err)
	}

	// Handle .gitignore.
	gitignoreModified, err := handleGitignore(dir)
	if err != nil {
		// Non-fatal: warn but don't abort a successful init.
		fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %v\n", err)
	}

	// Print success output (§7 format).
	fmt.Printf("Initialized tkt project in %s\n", tktDir)
	if gitignoreModified {
		fmt.Println("Added .tkt/ to .gitignore")
	}

	return nil
}

// resolveInitDir returns the directory in which to initialise the project.
// If --dir was provided it is used; otherwise the current working directory.
func resolveInitDir() (string, error) {
	if rootDir != "" {
		abs, err := filepath.Abs(rootDir)
		if err != nil {
			return "", fmt.Errorf("resolve --dir: %w", err)
		}
		return abs, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	return dir, nil
}

// handleGitignore checks the global config's GitignoreAuto setting and, if
// enabled, ensures ".tkt/" appears in <dir>/.gitignore. It creates the file if
// absent. Returns true when the file was created or modified.
func handleGitignore(dir string) (bool, error) {
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return false, fmt.Errorf("load global config: %w", err)
	}
	if !globalCfg.GitignoreAuto {
		return false, nil
	}

	gitignorePath := filepath.Join(dir, ".gitignore")

	// If the file doesn't exist, create it.
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		if err := os.WriteFile(gitignorePath, []byte(".tkt/\n"), 0o644); err != nil {
			return false, fmt.Errorf("create .gitignore: %w", err)
		}
		return true, nil
	}

	// File exists — check whether .tkt/ (or .tkt) is already listed.
	f, err := os.Open(gitignorePath)
	if err != nil {
		return false, fmt.Errorf("open .gitignore: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == ".tkt/" || line == ".tkt" {
			return false, nil // already present
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("read .gitignore: %w", err)
	}
	f.Close()

	// Append .tkt/ on its own line.
	af, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return false, fmt.Errorf("open .gitignore for append: %w", err)
	}
	defer af.Close()

	if _, err := af.WriteString("\n.tkt/\n"); err != nil {
		return false, fmt.Errorf("append to .gitignore: %w", err)
	}

	return true, nil
}
