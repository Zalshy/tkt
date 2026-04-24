package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// runUpdateInDir sets rootDir to dir, configures flags via setupFlags, then calls runUpdate.
// Returns captured stdout and any error.
func runUpdateInDir(t *testing.T, dir string, args []string, setupFlags func()) (string, error) {
	t.Helper()

	savedRootDir := rootDir
	defer func() {
		rootDir = savedRootDir
		updateCmd.SetOut(nil)
		// Reset flags to defaults so tests don't bleed into each other.
		updateCmd.Flags().Set("type", "")      //nolint:errcheck
		updateCmd.Flags().Set("attention", "0") //nolint:errcheck
		// Reset Changed state by re-parsing with no args (cobra tracks Changed via lookup).
		// Simplest reliable reset: mark both as not changed by re-creating flag lookup state.
		// cobra doesn't expose a public Reset, so we rely on the flag values being reset above;
		// Changed() reflects whether Parse set the value, which is cleared when we reset via Set.
		// Actually cobra's pflag.Changed is set only during Parse, not Set — so we need
		// to parse empty args to clear it. Use a fresh command approach instead:
		// Nothing to do here — Changed is set by Parse only. We control it via setupFlags.
	}()

	// Reset the Changed tracking by re-initialising flags on a fresh lookup.
	// cobra/pflag tracks Changed per-flag; calling Set() does NOT set Changed=true.
	// So the previous test's Changed state does not carry over — no extra reset needed.

	rootDir = dir

	if setupFlags != nil {
		setupFlags()
	}

	var buf bytes.Buffer
	updateCmd.SetOut(&buf)

	err := runUpdate(updateCmd, args)
	return buf.String(), err
}

// TestUpdate_NoFlags verifies that calling update with no flags returns the "provide at least one" error.
func TestUpdate_NoFlags(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-update-001")
	id := seedTicketWithStatus(t, dir, "Update target", "TODO")

	_, err := runUpdateInDir(t, dir, []string{id}, nil)
	if err == nil {
		t.Fatal("expected error for no flags, got nil")
	}
	if !strings.Contains(err.Error(), "provide at least one") {
		t.Errorf("expected 'provide at least one' in error, got: %v", err)
	}
}

// TestUpdate_TypeFlag verifies that --type updates main_type and prints "#N updated".
func TestUpdate_TypeFlag(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-update-002")
	id := seedTicketWithStatus(t, dir, "Update type target", "TODO")

	out, err := runUpdateInDir(t, dir, []string{id}, func() {
		// Use cobra's Parse to set the flag so Changed() returns true.
		updateCmd.ParseFlags([]string{"--type", "bugfix"}) //nolint:errcheck
	})
	if err != nil {
		t.Fatalf("runUpdate --type: %v", err)
	}

	want := fmt.Sprintf("#%s updated", id)
	if !strings.Contains(out, want) {
		t.Errorf("expected %q in output, got: %q", want, out)
	}
}

// TestUpdate_AttentionFlag verifies that --attention updates attention_level and prints "#N updated".
func TestUpdate_AttentionFlag(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-update-003")
	id := seedTicketWithStatus(t, dir, "Update attention target", "TODO")

	out, err := runUpdateInDir(t, dir, []string{id}, func() {
		updateCmd.ParseFlags([]string{"--attention", "75"}) //nolint:errcheck
	})
	if err != nil {
		t.Fatalf("runUpdate --attention: %v", err)
	}

	want := fmt.Sprintf("#%s updated", id)
	if !strings.Contains(out, want) {
		t.Errorf("expected %q in output, got: %q", want, out)
	}
}

// TestUpdate_BothFlags verifies that both --type and --attention can be set together.
func TestUpdate_BothFlags(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-update-004")
	id := seedTicketWithStatus(t, dir, "Update both target", "TODO")

	out, err := runUpdateInDir(t, dir, []string{id}, func() {
		updateCmd.ParseFlags([]string{"--type", "hotfix", "--attention", "80"}) //nolint:errcheck
	})
	if err != nil {
		t.Fatalf("runUpdate --type --attention: %v", err)
	}

	want := fmt.Sprintf("#%s updated", id)
	if !strings.Contains(out, want) {
		t.Errorf("expected %q in output, got: %q", want, out)
	}
}

// TestUpdate_AttentionOutOfRange verifies that --attention 150 returns a validation error.
func TestUpdate_AttentionOutOfRange(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	seedSession(t, dir, "impl-update-005")
	id := seedTicketWithStatus(t, dir, "Update oob target", "TODO")

	_, err := runUpdateInDir(t, dir, []string{id}, func() {
		updateCmd.ParseFlags([]string{"--attention", "150"}) //nolint:errcheck
	})
	if err == nil {
		t.Fatal("expected error for out-of-range attention, got nil")
	}
	if !strings.Contains(err.Error(), "attention") {
		t.Errorf("expected 'attention' in error, got: %v", err)
	}
}
