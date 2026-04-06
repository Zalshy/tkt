package cmd

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// runBatchInDir sets rootDir and batchN, invokes runBatch, returns captured stdout and error.
func runBatchInDir(t *testing.T, dir string, n int) (string, error) {
	t.Helper()

	savedRootDir := rootDir
	savedN := batchN
	defer func() {
		rootDir = savedRootDir
		batchN = savedN
		batchCmd.SetOut(nil)
	}()

	rootDir = dir
	batchN = n

	var buf bytes.Buffer
	batchCmd.SetOut(&buf)

	err := runBatch(batchCmd, nil)
	return buf.String(), err
}

// mustParseID converts a string ticket ID (as returned by seedTicketWithStatus) to int64.
func mustParseID(t *testing.T, s string) int64 {
	t.Helper()
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Fatalf("mustParseID: %v", err)
	}
	return id
}

// TestBatch_EmptyProject verifies that an empty project prints "No active tickets."
func TestBatch_EmptyProject(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	out, err := runBatchInDir(t, dir, 6)
	if err != nil {
		t.Fatalf("runBatch: %v", err)
	}
	if !strings.Contains(out, "No active tickets.") {
		t.Errorf("expected 'No active tickets.', got: %q", out)
	}
}

// TestBatch_AllVerifiedOrCanceled verifies that only VERIFIED/CANCELED tickets → "No active tickets."
func TestBatch_AllVerifiedOrCanceled(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	seedTicketWithStatus(t, dir, "Verified ticket", "VERIFIED")
	seedTicketWithStatus(t, dir, "Canceled ticket", "CANCELED")

	out, err := runBatchInDir(t, dir, 6)
	if err != nil {
		t.Fatalf("runBatch: %v", err)
	}
	if !strings.Contains(out, "No active tickets.") {
		t.Errorf("expected 'No active tickets.', got: %q", out)
	}
}

// TestBatch_SingleTicketNoDeps verifies a single TODO ticket with no deps appears in phase 1.
func TestBatch_SingleTicketNoDeps(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	idStr := seedTicketWithStatus(t, dir, "Solo ticket", "TODO")
	id := mustParseID(t, idStr)

	out, err := runBatchInDir(t, dir, 6)
	if err != nil {
		t.Fatalf("runBatch: %v", err)
	}
	if !strings.Contains(out, "Phase 1") {
		t.Errorf("expected 'Phase 1' in output, got: %q", out)
	}
	if !strings.Contains(out, fmt.Sprintf("#%d", id)) {
		t.Errorf("expected '#%d' in output, got: %q", id, out)
	}
	if !strings.Contains(out, "1 phases remaining") {
		t.Errorf("expected '1 phases remaining' in summary, got: %q", out)
	}
}

// TestBatch_LinearChain verifies A→B→C produces 3 phases with one ticket each.
func TestBatch_LinearChain(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	aStr := seedTicketWithStatus(t, dir, "A", "TODO")
	bStr := seedTicketWithStatus(t, dir, "B", "TODO")
	cStr := seedTicketWithStatus(t, dir, "C", "TODO")
	a, b, c := mustParseID(t, aStr), mustParseID(t, bStr), mustParseID(t, cStr)

	// B depends on A; C depends on B.
	insertDependency(t, dir, b, a)
	insertDependency(t, dir, c, b)

	out, err := runBatchInDir(t, dir, 6)
	if err != nil {
		t.Fatalf("runBatch: %v", err)
	}

	if !strings.Contains(out, "3 phases remaining") {
		t.Errorf("expected '3 phases remaining', got: %q", out)
	}

	lines := strings.Split(out, "\n")
	phase1Line, phase2Line, phase3Line := "", "", ""
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(trimmed, "Phase 1"):
			phase1Line = l
		case strings.HasPrefix(trimmed, "Phase 2"):
			phase2Line = l
		case strings.HasPrefix(trimmed, "Phase 3"):
			phase3Line = l
		}
	}

	if !strings.Contains(phase1Line, fmt.Sprintf("#%d", a)) {
		t.Errorf("Phase 1 should contain #%d (A), got: %q", a, phase1Line)
	}
	if !strings.Contains(phase2Line, fmt.Sprintf("#%d", b)) {
		t.Errorf("Phase 2 should contain #%d (B), got: %q", b, phase2Line)
	}
	if !strings.Contains(phase3Line, fmt.Sprintf("#%d", c)) {
		t.Errorf("Phase 3 should contain #%d (C), got: %q", c, phase3Line)
	}
}

// TestBatch_Diamond verifies diamond (A→C, B→C): phase 1 has A and B, phase 2 has C.
func TestBatch_Diamond(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	aStr := seedTicketWithStatus(t, dir, "A", "TODO")
	bStr := seedTicketWithStatus(t, dir, "B", "TODO")
	cStr := seedTicketWithStatus(t, dir, "C", "TODO")
	a, b, c := mustParseID(t, aStr), mustParseID(t, bStr), mustParseID(t, cStr)

	// C depends on A and B.
	insertDependency(t, dir, c, a)
	insertDependency(t, dir, c, b)

	out, err := runBatchInDir(t, dir, 6)
	if err != nil {
		t.Fatalf("runBatch: %v", err)
	}

	if !strings.Contains(out, "2 phases remaining") {
		t.Errorf("expected '2 phases remaining', got: %q", out)
	}

	lines := strings.Split(out, "\n")
	phase1Line, phase2Line := "", ""
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(trimmed, "Phase 1"):
			phase1Line = l
		case strings.HasPrefix(trimmed, "Phase 2"):
			phase2Line = l
		}
	}

	if !strings.Contains(phase1Line, fmt.Sprintf("#%d", a)) {
		t.Errorf("Phase 1 should contain #%d (A), got: %q", a, phase1Line)
	}
	if !strings.Contains(phase1Line, fmt.Sprintf("#%d", b)) {
		t.Errorf("Phase 1 should contain #%d (B), got: %q", b, phase1Line)
	}
	if !strings.Contains(phase2Line, fmt.Sprintf("#%d", c)) {
		t.Errorf("Phase 2 should contain #%d (C), got: %q", c, phase2Line)
	}
}

// TestBatch_NFlag verifies --n 2 on a 4-phase chain returns only 2 phases.
func TestBatch_NFlag(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	aStr := seedTicketWithStatus(t, dir, "A", "TODO")
	bStr := seedTicketWithStatus(t, dir, "B", "TODO")
	cStr := seedTicketWithStatus(t, dir, "C", "TODO")
	dStr := seedTicketWithStatus(t, dir, "D", "TODO")
	a, b, c, d := mustParseID(t, aStr), mustParseID(t, bStr), mustParseID(t, cStr), mustParseID(t, dStr)

	insertDependency(t, dir, b, a)
	insertDependency(t, dir, c, b)
	insertDependency(t, dir, d, c)

	out, err := runBatchInDir(t, dir, 2)
	if err != nil {
		t.Fatalf("runBatch: %v", err)
	}

	if !strings.Contains(out, "2 phases remaining") {
		t.Errorf("expected '2 phases remaining', got: %q", out)
	}
	if strings.Contains(out, "Phase 3") {
		t.Errorf("expected no 'Phase 3' with --n 2, got: %q", out)
	}
	if strings.Contains(out, "Phase 4") {
		t.Errorf("expected no 'Phase 4' with --n 2, got: %q", out)
	}
	// Suppress unused variable warnings.
	_ = d
}

// TestBatch_InProgressGlyph verifies IN_PROGRESS ticket gets ⟳ glyph.
func TestBatch_InProgressGlyph(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	idStr := seedTicketWithStatus(t, dir, "In-progress ticket", "IN_PROGRESS")
	id := mustParseID(t, idStr)

	out, err := runBatchInDir(t, dir, 6)
	if err != nil {
		t.Fatalf("runBatch: %v", err)
	}
	if !strings.Contains(out, "⟳") {
		t.Errorf("expected ⟳ glyph for IN_PROGRESS ticket, got: %q", out)
	}
	if !strings.Contains(out, fmt.Sprintf("#%d", id)) {
		t.Errorf("expected '#%d' in output, got: %q", id, out)
	}
}

// TestBatch_PlanningGlyph verifies PLANNING ticket gets ● glyph.
func TestBatch_PlanningGlyph(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	idStr := seedTicketWithStatus(t, dir, "Planning ticket", "PLANNING")
	id := mustParseID(t, idStr)

	out, err := runBatchInDir(t, dir, 6)
	if err != nil {
		t.Fatalf("runBatch: %v", err)
	}
	if !strings.Contains(out, "●") {
		t.Errorf("expected ● glyph for PLANNING ticket, got: %q", out)
	}
	if !strings.Contains(out, fmt.Sprintf("#%d", id)) {
		t.Errorf("expected '#%d' in output, got: %q", id, out)
	}
}

// TestBatch_VerifiedDepResolved verifies that a VERIFIED dep is treated as resolved
// so the downstream ticket appears in phase 1.
func TestBatch_VerifiedDepResolved(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	verifiedIDStr := seedTicketWithStatus(t, dir, "Verified dep", "VERIFIED")
	activeIDStr := seedTicketWithStatus(t, dir, "Downstream", "TODO")
	verifiedID, activeID := mustParseID(t, verifiedIDStr), mustParseID(t, activeIDStr)

	// activeID depends on verifiedID (already VERIFIED → resolved).
	insertDependency(t, dir, activeID, verifiedID)

	out, err := runBatchInDir(t, dir, 6)
	if err != nil {
		t.Fatalf("runBatch: %v", err)
	}

	lines := strings.Split(out, "\n")
	phase1Line := ""
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "Phase 1") {
			phase1Line = l
			break
		}
	}
	if !strings.Contains(phase1Line, fmt.Sprintf("#%d", activeID)) {
		t.Errorf("expected '#%d' in Phase 1 (VERIFIED dep treated as resolved), got: %q", activeID, phase1Line)
	}
}

// TestBatch_CanceledDepResolved verifies that a CANCELED dep is treated as resolved
// so the downstream ticket appears in phase 1.
func TestBatch_CanceledDepResolved(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	canceledIDStr := seedTicketWithStatus(t, dir, "Canceled dep", "CANCELED")
	activeIDStr := seedTicketWithStatus(t, dir, "Downstream", "TODO")
	canceledID, activeID := mustParseID(t, canceledIDStr), mustParseID(t, activeIDStr)

	// activeID depends on canceledID (CANCELED → resolved).
	insertDependency(t, dir, activeID, canceledID)

	out, err := runBatchInDir(t, dir, 6)
	if err != nil {
		t.Fatalf("runBatch: %v", err)
	}

	lines := strings.Split(out, "\n")
	phase1Line := ""
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "Phase 1") {
			phase1Line = l
			break
		}
	}
	if !strings.Contains(phase1Line, fmt.Sprintf("#%d", activeID)) {
		t.Errorf("expected '#%d' in Phase 1 (CANCELED dep treated as resolved), got: %q", activeID, phase1Line)
	}
}
