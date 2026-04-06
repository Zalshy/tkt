package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ProjectConfig holds per-project settings stored at .tkt/config.json.
// Written by tkt init; read by any command that needs project metadata.
type ProjectConfig struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"` // RFC3339 string; formatted by the caller

	// MonitorInterval is the TUI poll interval in seconds.
	// A value of 0 means "use the default" (2 seconds). Set by tkt init.
	MonitorInterval int `json:"monitor_interval"`
}

// projectConfigPath returns the path to the project config file.
func projectConfigPath(root string) string {
	return filepath.Join(root, ".tkt", "config.json")
}

// LoadProject loads the project config from root/.tkt/config.json.
// Returns an error if the file does not exist — an absent project config means
// the project has not been initialised with tkt init.
func LoadProject(root string) (*ProjectConfig, error) {
	path := projectConfigPath(root)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load project config: %w", err)
	}

	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("load project config: %w", err)
	}

	return &cfg, nil
}

// WriteProject serialises cfg to root/.tkt/config.json, creating the file if it does
// not exist and overwriting it if it does. The .tkt/ directory must already exist —
// creating it is the responsibility of the caller (tkt init).
func WriteProject(root string, cfg *ProjectConfig) error {
	path := projectConfigPath(root)

	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return fmt.Errorf("write project config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write project config: %w", err)
	}

	return nil
}
