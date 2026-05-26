package cmd

import (
	"strings"
	"testing"
)

func TestWorkflowIntegration_MajorLifecycle(t *testing.T) {
	dir := t.TempDir()
	if err := runInitInDir(t, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	if out, err := runSessionInDir(t, dir, func() { sessionRole = "architect" }); err != nil {
		t.Fatalf("architect session: %v", err)
	} else if !strings.Contains(out, "Session created:") || !strings.Contains(out, "architect") {
		t.Fatalf("architect session output missing role: %q", out)
	}

	if out, err := runNewInDir(t, dir, []string{"Prepare fixture data"}, func() {
		newDescription = "Fixture setup for workflow integration."
		newTier = "low"
		newMainType = "test"
		newAttention = 24
	}); err != nil {
		t.Fatalf("new dependency: %v", err)
	} else if !strings.Contains(out, `Created #1  "Prepare fixture data"`) {
		t.Fatalf("dependency create output mismatch: %q", out)
	}

	if out, err := runNewInDir(t, dir, []string{"Ship major workflow"}, func() {
		newDescription = "Main ticket exercises lifecycle, logs, dependencies, and output."
		newTier = "standard"
		newMainType = "test"
		newAttention = 45
	}); err != nil {
		t.Fatalf("new main: %v", err)
	} else if !strings.Contains(out, `Created #2  "Ship major workflow"`) {
		t.Fatalf("main create output mismatch: %q", out)
	}

	if out, err := runNewInDir(t, dir, []string{"Missing plan guard"}, func() {
		newDescription = "Invalid transition target."
		newTier = "standard"
		newMainType = "test"
		newAttention = 40
	}); err != nil {
		t.Fatalf("new guard: %v", err)
	} else if !strings.Contains(out, `Created #3  "Missing plan guard"`) {
		t.Fatalf("guard create output mismatch: %q", out)
	}

	assertTicketMetadata(t, dir, 1, "low", "test", 24)
	assertTicketMetadata(t, dir, 2, "standard", "test", 45)
	assertTicketMetadata(t, dir, 3, "standard", "test", 40)

	if out, err := runDependsInDir(t, dir, []string{"2"}, func() { dependsOn = "1" }); err != nil {
		t.Fatalf("depends: %v", err)
	} else if !strings.Contains(out, "#2 now depends on #1") {
		t.Fatalf("depends output mismatch: %q", out)
	}

	batchOut, err := runBatchInDir(t, dir, 6)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	for _, want := range []string{"Phase 1", "#1", "#3", "Phase 2", "#2"} {
		if !strings.Contains(batchOut, want) {
			t.Fatalf("batch output missing %q: %q", want, batchOut)
		}
	}

	if out, err := runCommentInDir(t, dir, []string{"2", "workflow integration comment"}); err != nil {
		t.Fatalf("comment: %v", err)
	} else if !strings.Contains(out, `"workflow integration comment"`) {
		t.Fatalf("comment output mismatch: %q", out)
	}

	if out, err := runAdvanceInDir(t, dir, []string{"3"}, func() { advanceNote = "start guard planning" }); err != nil {
		t.Fatalf("guard TODO -> PLANNING: %v", err)
	} else if !strings.Contains(out, "#3  TODO → PLANNING") {
		t.Fatalf("guard planning output mismatch: %q", out)
	}

	if _, err := runSessionInDir(t, dir, func() { sessionRole = "architect" }); err != nil {
		t.Fatalf("second architect session: %v", err)
	}
	if _, err := runAdvanceInDir(t, dir, []string{"3"}, func() { advanceNote = "approve without plan" }); err == nil {
		t.Fatal("expected plan guard error for PLANNING -> IN_PROGRESS without plan")
	}
	assertTicketStatus(t, dir, 3, "PLANNING")

	if out, err := runAdvanceInDir(t, dir, []string{"2"}, func() { advanceNote = "start planning" }); err != nil {
		t.Fatalf("main TODO -> PLANNING: %v", err)
	} else if !strings.Contains(out, "#2  TODO → PLANNING") {
		t.Fatalf("main planning output mismatch: %q", out)
	}

	if _, err := runSessionInDir(t, dir, func() { sessionRole = "implementer" }); err != nil {
		t.Fatalf("implementer session: %v", err)
	}
	resetPlanFlags(t)
	if err := planCmd.Flags().Set("body", "## Plan\n1. Implement workflow test.\n2. Verify outputs."); err != nil {
		t.Fatalf("set plan body: %v", err)
	}
	planOut, err := runPlanInDir(t, dir, []string{"2"})
	resetPlanFlags(t)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !strings.Contains(planOut, "Plan updated for #2") {
		t.Fatalf("plan output mismatch: %q", planOut)
	}

	if _, err := runSessionInDir(t, dir, func() { sessionRole = "architect" }); err != nil {
		t.Fatalf("approval architect session: %v", err)
	}
	if out, err := runAdvanceInDir(t, dir, []string{"2"}, func() { advanceNote = "approve plan" }); err != nil {
		t.Fatalf("main PLANNING -> IN_PROGRESS: %v", err)
	} else if !strings.Contains(out, "#2  PLANNING → IN_PROGRESS") {
		t.Fatalf("main in-progress output mismatch: %q", out)
	}

	if _, err := runSessionInDir(t, dir, func() { sessionRole = "implementer" }); err != nil {
		t.Fatalf("done implementer session: %v", err)
	}
	if out, err := runAdvanceInDir(t, dir, []string{"2"}, func() { advanceNote = "implementation complete" }); err != nil {
		t.Fatalf("main IN_PROGRESS -> DONE: %v", err)
	} else if !strings.Contains(out, "#2  IN_PROGRESS → DONE") {
		t.Fatalf("main done output mismatch: %q", out)
	}

	if _, err := runSessionInDir(t, dir, func() { sessionRole = "architect" }); err != nil {
		t.Fatalf("verify architect session: %v", err)
	}
	if out, err := runAdvanceInDir(t, dir, []string{"2"}, func() { advanceNote = "verified" }); err != nil {
		t.Fatalf("main DONE -> VERIFIED: %v", err)
	} else if !strings.Contains(out, "#2  DONE → VERIFIED") {
		t.Fatalf("main verified output mismatch: %q", out)
	}
	assertTicketStatus(t, dir, 2, "VERIFIED")

	showOut, err := runShowInDir(t, dir, []string{"2"})
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	for _, want := range []string{"Ship major workflow", "VERIFIED", "workflow integration comment", "[plan]", "Dependencies:"} {
		if !strings.Contains(showOut, want) {
			t.Fatalf("show output missing %q: %q", want, showOut)
		}
	}

	listOut, err := runListInDir(t, dir, func() {
		listAll = true
		listVerified = true
		listSort = "id"
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, want := range []string{"#1", "Prepare fixture data", "#2", "Ship major workflow", "#3", "Missing plan guard"} {
		if !strings.Contains(listOut, want) {
			t.Fatalf("list output missing %q: %q", want, listOut)
		}
	}
}

func assertTicketMetadata(t *testing.T, dir string, id int64, wantTier, wantType string, wantAttention int) {
	t.Helper()
	database := openProjectDB(t, dir)
	var gotTier, gotType string
	var gotAttention int
	if err := database.QueryRow(`SELECT tier, main_type, attention_level FROM tickets WHERE id = ?`, id).Scan(&gotTier, &gotType, &gotAttention); err != nil {
		t.Fatalf("query ticket #%d metadata: %v", id, err)
	}
	if gotTier != wantTier || gotType != wantType || gotAttention != wantAttention {
		t.Fatalf("ticket #%d metadata = (%q, %q, %d), want (%q, %q, %d)", id, gotTier, gotType, gotAttention, wantTier, wantType, wantAttention)
	}
}

func assertTicketStatus(t *testing.T, dir string, id int64, want string) {
	t.Helper()
	database := openProjectDB(t, dir)
	var got string
	if err := database.QueryRow(`SELECT status FROM tickets WHERE id = ?`, id).Scan(&got); err != nil {
		t.Fatalf("query ticket #%d status: %v", id, err)
	}
	if got != want {
		t.Fatalf("ticket #%d status = %s, want %s", id, got, want)
	}
}
