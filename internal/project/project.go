package project

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindRoot walks up from the current working directory until it finds a directory
// containing a .tkt/ entry. Returns the absolute path to that directory.
// Returns an error if no .tkt/ directory is found before the filesystem root.
func FindRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not determine working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".tkt")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no .tkt/ directory found. Run: tkt init")
		}
		dir = parent
	}
}

// TicketsDir returns the path to the .tkt directory inside root.
func TicketsDir(root string) string {
	return filepath.Join(root, ".tkt")
}

// SessionFile returns the path to the session file inside root.
func SessionFile(root string) string {
	return filepath.Join(root, ".tkt", "session")
}

// DBPath returns the path to the SQLite database file inside root.
func DBPath(root string) string {
	return filepath.Join(root, ".tkt", "db.sqlite")
}
