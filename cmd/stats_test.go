package cmd

import (
	"testing"
	"time"

	statsPkg "github.com/zalshy/tkt/internal/stats"
)

func TestParseStatsWindow(t *testing.T) {
	tests := []struct {
		value string
		want  time.Duration
	}{
		{"24h", 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"30d", 30 * 24 * time.Hour},
	}
	for _, tt := range tests {
		got, err := statsPkg.ParseWindow(tt.value)
		if err != nil {
			t.Fatalf("parseStatsWindow(%q): %v", tt.value, err)
		}
		if got != tt.want {
			t.Fatalf("parseStatsWindow(%q)=%s want %s", tt.value, got, tt.want)
		}
	}
}

func TestParseStatsWindowInvalid(t *testing.T) {
	for _, value := range []string{"", "0", "0h", "-1h", "0d", "xd"} {
		if _, err := statsPkg.ParseWindow(value); err == nil {
			t.Fatalf("expected error for %q", value)
		}
	}
}

func TestStatsWindowConflictsWithSinceUntil(t *testing.T) {
	savedSince, savedUntil, savedWindow := statsSince, statsUntil, statsWindow
	defer func() { statsSince, statsUntil, statsWindow = savedSince, savedUntil, savedWindow }()

	statsSince = "2026-04-01"
	statsUntil = ""
	statsWindow = "24h"
	if _, err := statsOptionsFromFlags(); err == nil {
		t.Fatal("expected --window/--since conflict")
	}

	statsSince = ""
	statsUntil = "2026-04-02"
	statsWindow = "24h"
	if _, err := statsOptionsFromFlags(); err == nil {
		t.Fatal("expected --window/--until conflict")
	}
}

func TestStatsDefaultScopeExcludesWindow(t *testing.T) {
	saved := statsWindow
	defer func() { statsWindow = saved }()
	statsWindow = "24h"
	if statsDefaultScopeActive() {
		t.Fatal("window should disable default scope")
	}
}
