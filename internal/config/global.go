package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// GlobalConfig holds user-level preferences stored at ~/.config/tkt/config.json
// (or the platform equivalent resolved by os.UserConfigDir).
type GlobalConfig struct {
	// GitignoreAuto controls whether tkt init automatically appends .tkt/ to .gitignore.
	// Defaults to true.
	GitignoreAuto bool `json:"gitignore_auto"`

	// DefaultRole pre-fills the role when starting a new session.
	// Valid values: "architect", "implementer", or "" (no default).
	DefaultRole string `json:"default_role"`
}

// globalConfigPath returns the path to the global config file.
// It uses os.UserConfigDir() so the correct platform directory is used
// (e.g. ~/.config on Linux via $XDG_CONFIG_HOME, ~/Library/Application Support on macOS).
func globalConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tkt", "config.json"), nil
}

// LoadGlobal loads the global config from the user config directory.
// If the file does not exist, defaults are returned silently — the global config
// is optional and purely for convenience. Any other OS error is returned wrapped.
func LoadGlobal() (*GlobalConfig, error) {
	// Start from defaults. GitignoreAuto must be true unless explicitly overridden,
	// so we pre-populate before unmarshalling rather than relying on Go zero values.
	cfg := &GlobalConfig{
		GitignoreAuto: true,
	}

	path, err := globalConfigPath()
	if err != nil {
		// Can't resolve config dir (e.g. $HOME unset in CI). Treat as missing — defaults.
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("load global config: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("load global config: %w", err)
	}

	return cfg, nil
}
