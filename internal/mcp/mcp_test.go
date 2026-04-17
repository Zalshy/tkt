package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/models"
)

// ---- helpers ----------------------------------------------------------------

// mkTktDir creates the .tkt/ subdirectory inside root so db.Open can succeed.
func mkTktDir(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".tkt"), 0o700); err != nil {
		t.Fatalf("mkTktDir: %v", err)
	}
}

// setup creates a fresh tkt dir + db + implementer session.
// Returns (root, mcpServer).
func setup(t *testing.T) (string, *mcpserver.MCPServer) {
	t.Helper()
	root := t.TempDir()
	mkTktDir(t, root)

	database, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	// Create system session (implementer role — built-in).
	sess, err := EnsureMCPSession(database, "implementer")
	if err != nil {
		t.Fatalf("EnsureMCPSession: %v", err)
	}

	s := NewServer(root, database, sess)
	return root, s
}

// setupReadonly creates a readonly server (no session → only read tools).
func setupReadonly(t *testing.T) (string, *mcpserver.MCPServer) {
	t.Helper()
	root := t.TempDir()
	mkTktDir(t, root)

	database, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	s := NewServer(root, database, nil)
	return root, s
}

// callTool invokes a registered tool by name with the given args map.
// Returns the result; never panics even on unknown tool.
func callTool(t *testing.T, s *mcpserver.MCPServer, name string, args map[string]any) *mcplib.CallToolResult {
	t.Helper()
	tool := s.GetTool(name)
	if tool == nil {
		t.Fatalf("tool %q not registered", name)
	}
	req := mcplib.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("tool %q: unexpected Go error: %v", name, err)
	}
	return res
}

// resultText extracts the text content from a CallToolResult.
func resultText(res *mcplib.CallToolResult) string {
	if res == nil {
		return ""
	}
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcplib.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

// assertOK fails the test if the result is an error result.
func assertOK(t *testing.T, res *mcplib.CallToolResult, context string) {
	t.Helper()
	if res.IsError {
		t.Errorf("%s: expected success, got MCP error: %s", context, resultText(res))
	}
}

// assertError fails the test if the result is NOT an error result.
func assertError(t *testing.T, res *mcplib.CallToolResult, context string) {
	t.Helper()
	if !res.IsError {
		t.Errorf("%s: expected MCP error, got success: %s", context, resultText(res))
	}
}

// assertContains fails if text is not found in the result text.
func assertContains(t *testing.T, res *mcplib.CallToolResult, substr, ctx string) {
	t.Helper()
	text := resultText(res)
	if !strings.Contains(text, substr) {
		t.Errorf("%s: expected %q in result, got: %q", ctx, substr, text)
	}
}

// seedTicket inserts a ticket directly via DB. Returns string ID.
func seedTicket(t *testing.T, root string, title string, status models.Status) string {
	t.Helper()
	database, err := db.Open(root)
	if err != nil {
		t.Fatalf("seedTicket: open db: %v", err)
	}
	defer database.Close()

	res, err := database.Exec(
		`INSERT INTO tickets (title, description, status, created_by) VALUES (?, '', ?, 'test-seed')`,
		title, string(status),
	)
	if err != nil {
		t.Fatalf("seedTicket: insert: %v", err)
	}
	id, _ := res.LastInsertId()
	return fmt.Sprintf("%d", id)
}

// ---- read tools -------------------------------------------------------------

func TestListTickets_Empty(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_list_tickets", nil)
	assertOK(t, res, "list empty")
	// Empty DB → "No tickets found."
	assertContains(t, res, "No tickets found", "empty list message")
}

func TestListTickets_ReturnsTickets(t *testing.T) {
	root, s := setup(t)
	seedTicket(t, root, "Alpha ticket", models.StatusTodo)
	seedTicket(t, root, "Beta ticket", models.StatusInProgress)

	res := callTool(t, s, "tkt_list_tickets", map[string]any{"all": true})
	assertOK(t, res, "list with tickets")
	assertContains(t, res, "Alpha ticket", "alpha in list")
	assertContains(t, res, "Beta ticket", "beta in list")
}

func TestListTickets_StatusFilter(t *testing.T) {
	root, s := setup(t)
	seedTicket(t, root, "Todo item", models.StatusTodo)
	seedTicket(t, root, "Done item", models.StatusDone)

	res := callTool(t, s, "tkt_list_tickets", map[string]any{
		"status": "TODO",
		"all":    true,
	})
	assertOK(t, res, "filter by TODO")
	assertContains(t, res, "Todo item", "todo ticket in result")
	text := resultText(res)
	if strings.Contains(text, "Done item") {
		t.Errorf("DONE ticket should not appear in TODO filter: %q", text)
	}
}

func TestShowTicket_Happy(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Show me", models.StatusTodo)

	res := callTool(t, s, "tkt_show_ticket", map[string]any{"id": id})
	assertOK(t, res, "show ticket")
	assertContains(t, res, "Show me", "title in output")
}

func TestShowTicket_MissingID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_show_ticket", map[string]any{})
	assertError(t, res, "show without id")
}

func TestShowTicket_InvalidID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_show_ticket", map[string]any{"id": "99999"})
	assertError(t, res, "show nonexistent ticket")
}

func TestSearchTickets_Happy(t *testing.T) {
	root, s := setup(t)
	seedTicket(t, root, "Feature: user login", models.StatusTodo)
	seedTicket(t, root, "Bug: crash on load", models.StatusTodo)

	res := callTool(t, s, "tkt_search_tickets", map[string]any{"query": "login"})
	assertOK(t, res, "search happy")
	assertContains(t, res, "login", "search result contains query term")
	text := resultText(res)
	if strings.Contains(text, "crash") {
		t.Errorf("unrelated ticket should not appear: %q", text)
	}
}

func TestSearchTickets_NoQuery(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_search_tickets", map[string]any{})
	assertError(t, res, "search without query")
}

func TestSearchTickets_NoResults(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_search_tickets", map[string]any{"query": "zzz_nonexistent_zzz"})
	assertOK(t, res, "search no results")
	assertContains(t, res, "No tickets found", "no results message")
}

func TestBatch_Empty(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_batch", nil)
	assertOK(t, res, "batch empty")
	assertContains(t, res, "No active tickets", "empty batch message")
}

func TestBatch_WithTickets(t *testing.T) {
	root, s := setup(t)
	seedTicket(t, root, "Task A", models.StatusTodo)
	seedTicket(t, root, "Task B", models.StatusPlanning)

	res := callTool(t, s, "tkt_batch", map[string]any{"n": float64(3)})
	assertOK(t, res, "batch with tickets")
	text := resultText(res)
	if !strings.Contains(text, "Phase") {
		t.Errorf("expected Phase in batch output, got: %q", text)
	}
}

func TestListContext_Empty(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_list_context", nil)
	assertOK(t, res, "list context empty")
	assertContains(t, res, "No context entries", "empty context message")
}

func TestListDocs_Empty(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_list_docs", map[string]any{})
	assertOK(t, res, "list docs empty")
	assertContains(t, res, "No documents found", "empty docs message")
}

func TestReadDoc_MissingID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_read_doc", map[string]any{})
	assertError(t, res, "read doc missing id")
}

func TestReadDoc_NonExistent(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_read_doc", map[string]any{"id_or_slug": "99-nope"})
	assertError(t, res, "read doc nonexistent")
}

func TestListRoles_Happy(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_list_roles", nil)
	assertOK(t, res, "list roles")
	// Built-in roles architect + implementer must appear.
	assertContains(t, res, "architect", "architect role listed")
	assertContains(t, res, "implementer", "implementer role listed")
}

// ---- write tools ------------------------------------------------------------

func TestNewTicket_Happy(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_new_ticket", map[string]any{"title": "My new ticket"})
	assertOK(t, res, "new ticket happy")
	assertContains(t, res, "Created", "created message")
	assertContains(t, res, "My new ticket", "title in output")
}

func TestNewTicket_MissingTitle(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_new_ticket", map[string]any{})
	assertError(t, res, "new ticket no title")
}

func TestNewTicket_WithDependency(t *testing.T) {
	root, s := setup(t)
	dep := seedTicket(t, root, "Dep ticket", models.StatusTodo)

	res := callTool(t, s, "tkt_new_ticket", map[string]any{
		"title": "Dependent ticket",
		"after": dep,
	})
	assertOK(t, res, "new with dep")
	assertContains(t, res, "Depends on", "dep line present")
}

func TestNewTicket_InvalidAfterID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_new_ticket", map[string]any{
		"title": "Bad dep",
		"after": "not-a-number",
	})
	assertError(t, res, "new ticket invalid after")
}

func TestNewTicket_Tiers(t *testing.T) {
	_, s := setup(t)
	for _, tier := range []string{"critical", "standard", "low"} {
		res := callTool(t, s, "tkt_new_ticket", map[string]any{
			"title": "Tier test " + tier,
			"tier":  tier,
		})
		assertOK(t, res, "new ticket tier "+tier)
	}
}

func TestAdvanceTicket_Happy(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Advanceable", models.StatusTodo)

	res := callTool(t, s, "tkt_advance_ticket", map[string]any{
		"id":   id,
		"note": "moving forward",
	})
	assertOK(t, res, "advance happy")
	assertContains(t, res, "→", "transition arrow in output")
}

func TestAdvanceTicket_MissingID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_advance_ticket", map[string]any{
		"note": "some note",
	})
	assertError(t, res, "advance missing id")
}

func TestAdvanceTicket_MissingNote(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_advance_ticket", map[string]any{
		"id": "1",
	})
	assertError(t, res, "advance missing note")
}

func TestAdvanceTicket_InvalidID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_advance_ticket", map[string]any{
		"id":   "99999",
		"note": "some note",
	})
	// ID not found → error reported in output (but result itself may still be "text" with error section)
	// The tool puts errors in the text body; let's check for "Errors:" section.
	text := resultText(res)
	if !strings.Contains(text, "Errors:") && !res.IsError {
		t.Errorf("advance invalid id: expected error indication, got: %q", text)
	}
}

func TestAdvanceTicket_MultiID(t *testing.T) {
	root, s := setup(t)
	id1 := seedTicket(t, root, "Multi A", models.StatusTodo)
	id2 := seedTicket(t, root, "Multi B", models.StatusTodo)

	res := callTool(t, s, "tkt_advance_ticket", map[string]any{
		"id":   id1 + "," + id2,
		"note": "batch advance",
	})
	assertOK(t, res, "advance multi-id")
	text := resultText(res)
	if !strings.Contains(text, "#"+id1) || !strings.Contains(text, "#"+id2) {
		t.Errorf("advance multi: expected both IDs in output, got: %q", text)
	}
}

func TestAddComment_Happy(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Commentable", models.StatusTodo)

	res := callTool(t, s, "tkt_add_comment", map[string]any{
		"id":   id,
		"body": "This is a comment",
	})
	assertOK(t, res, "add comment happy")
	assertContains(t, res, "comment added", "comment added message")
}

func TestAddComment_MissingID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_add_comment", map[string]any{"body": "hi"})
	assertError(t, res, "comment missing id")
}

func TestAddComment_MissingBody(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_add_comment", map[string]any{"id": "1"})
	assertError(t, res, "comment missing body")
}

func TestAddComment_InvalidID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_add_comment", map[string]any{
		"id":   "99999",
		"body": "hi",
	})
	// Tool reports error in text body with "Errors:" section.
	text := resultText(res)
	if !strings.Contains(text, "Errors:") && !res.IsError {
		t.Errorf("comment invalid id: expected error indication, got: %q", text)
	}
}

func TestSubmitPlan_Happy(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Plannable", models.StatusPlanning)

	res := callTool(t, s, "tkt_submit_plan", map[string]any{
		"id":   id,
		"body": "My full plan text here",
	})
	assertOK(t, res, "submit plan happy")
	assertContains(t, res, "plan submitted", "plan submitted message")
}

func TestSubmitPlan_MissingID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_submit_plan", map[string]any{"body": "plan"})
	assertError(t, res, "submit plan missing id")
}

func TestSubmitPlan_MissingBody(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_submit_plan", map[string]any{"id": "1"})
	assertError(t, res, "submit plan missing body")
}

func TestSubmitPlan_InvalidID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_submit_plan", map[string]any{
		"id":   "99999",
		"body": "plan",
	})
	assertError(t, res, "submit plan invalid id")
}

func TestAddDepends_Happy(t *testing.T) {
	root, s := setup(t)
	id1 := seedTicket(t, root, "Main ticket", models.StatusTodo)
	id2 := seedTicket(t, root, "Dep ticket", models.StatusTodo)

	res := callTool(t, s, "tkt_add_depends", map[string]any{
		"id": id1,
		"on": id2,
	})
	assertOK(t, res, "add depends happy")
	assertContains(t, res, "depends on", "depends on message")
}

func TestAddDepends_MissingID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_add_depends", map[string]any{"on": "1"})
	assertError(t, res, "add depends missing id")
}

func TestAddDepends_MissingOn(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_add_depends", map[string]any{"id": "1"})
	assertError(t, res, "add depends missing on")
}

func TestAddDepends_InvalidOnID(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Main", models.StatusTodo)
	res := callTool(t, s, "tkt_add_depends", map[string]any{
		"id": id,
		"on": "not-a-number",
	})
	assertError(t, res, "add depends invalid on")
}

func TestSetTier_Happy(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Tierable", models.StatusTodo)

	res := callTool(t, s, "tkt_set_tier", map[string]any{
		"id":   id,
		"tier": "critical",
	})
	assertOK(t, res, "set tier happy")
	assertContains(t, res, "critical", "tier in output")
}

func TestSetTier_MissingID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_set_tier", map[string]any{"tier": "low"})
	assertError(t, res, "set tier missing id")
}

func TestSetTier_MissingTier(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_set_tier", map[string]any{"id": "1"})
	assertError(t, res, "set tier missing tier")
}

func TestSetTier_InvalidID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_set_tier", map[string]any{
		"id":   "99999",
		"tier": "low",
	})
	assertError(t, res, "set tier invalid id")
}

func TestArchiveTicket_MissingID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_archive_ticket", map[string]any{})
	assertError(t, res, "archive missing id")
}

func TestArchiveTicket_NonVerifiedFails(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Not verified", models.StatusTodo)

	res := callTool(t, s, "tkt_archive_ticket", map[string]any{"id": id})
	// Should report error — only VERIFIED can be archived.
	text := resultText(res)
	if !strings.Contains(text, "Errors:") && !res.IsError {
		t.Errorf("archive non-verified: expected error, got: %q", text)
	}
}

func TestArchiveTicket_VerifiedSucceeds(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Verified ticket", models.StatusVerified)

	res := callTool(t, s, "tkt_archive_ticket", map[string]any{"id": id})
	assertOK(t, res, "archive verified")
	assertContains(t, res, "ARCHIVED", "archived in output")
}

func TestLogUsage_Happy(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Usage target", models.StatusInProgress)

	res := callTool(t, s, "tkt_log_usage", map[string]any{
		"id":     id,
		"tokens": float64(1500),
		"tools":  float64(10),
	})
	assertOK(t, res, "log usage happy")
	assertContains(t, res, "1500 tokens", "token count in output")
}

func TestLogUsage_MissingID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_log_usage", map[string]any{"tokens": float64(100)})
	assertError(t, res, "log usage missing id")
}

func TestLogUsage_ZeroTokens(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_log_usage", map[string]any{
		"id":     "1",
		"tokens": float64(0),
	})
	assertError(t, res, "log usage zero tokens")
}

func TestLogUsage_InvalidID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_log_usage", map[string]any{
		"id":     "99999",
		"tokens": float64(100),
	})
	assertError(t, res, "log usage invalid id")
}

// ---- admin tools ------------------------------------------------------------

func TestAddContext_Happy(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_add_context", map[string]any{
		"title": "Test context entry",
		"body":  "Important caveat about the project",
	})
	assertOK(t, res, "add context happy")
	assertContains(t, res, "Context #", "context id in output")
}

func TestAddContext_MissingTitle(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_add_context", map[string]any{"body": "body"})
	assertError(t, res, "add context missing title")
}

func TestAddContext_MissingBody(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_add_context", map[string]any{"title": "title"})
	assertError(t, res, "add context missing body")
}

func TestUpdateContext_Happy(t *testing.T) {
	_, s := setup(t)
	// First add one.
	addRes := callTool(t, s, "tkt_add_context", map[string]any{
		"title": "Original",
		"body":  "Original body",
	})
	assertOK(t, addRes, "add before update")

	// Extract ID from result "Context #N added"
	text := resultText(addRes)
	var ctxID string
	if _, err := fmt.Sscanf(text, "Context #%s", &ctxID); err == nil {
		ctxID = strings.TrimSuffix(ctxID, " added:")
	}
	// Just use "1" — it's the first entry.
	ctxID = "1"

	res := callTool(t, s, "tkt_update_context", map[string]any{
		"id":    ctxID,
		"title": "Updated title",
		"body":  "Updated body",
	})
	assertOK(t, res, "update context happy")
	assertContains(t, res, "updated", "updated message")
}

func TestUpdateContext_MissingID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_update_context", map[string]any{
		"title": "t",
		"body":  "b",
	})
	assertError(t, res, "update context missing id")
}

func TestUpdateContext_InvalidID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_update_context", map[string]any{
		"id":    "not-a-number",
		"title": "t",
		"body":  "b",
	})
	assertError(t, res, "update context invalid id")
}

func TestDeleteContext_Happy(t *testing.T) {
	_, s := setup(t)
	callTool(t, s, "tkt_add_context", map[string]any{
		"title": "To be deleted",
		"body":  "gone soon",
	})

	res := callTool(t, s, "tkt_delete_context", map[string]any{"id": "1"})
	assertOK(t, res, "delete context happy")
	assertContains(t, res, "deleted", "deleted message")
}

func TestDeleteContext_MissingID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_delete_context", map[string]any{})
	assertError(t, res, "delete context missing id")
}

func TestDeleteContext_InvalidID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_delete_context", map[string]any{"id": "abc"})
	assertError(t, res, "delete context invalid id")
}

func TestAddDoc_Happy(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_add_doc", map[string]any{
		"slug":  "test-doc",
		"title": "Test Document",
		"body":  "Document body content",
		"type":  "analysis",
	})
	assertOK(t, res, "add doc happy")
	assertContains(t, res, "created docs/", "created doc message")
}

func TestAddDoc_MissingSlug(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_add_doc", map[string]any{
		"title": "t",
		"body":  "b",
		"type":  "analysis",
	})
	assertError(t, res, "add doc missing slug")
}

func TestAddDoc_MissingTitle(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_add_doc", map[string]any{
		"slug": "test",
		"body": "b",
		"type": "analysis",
	})
	assertError(t, res, "add doc missing title")
}

func TestAddDoc_MissingBody(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_add_doc", map[string]any{
		"slug":  "test",
		"title": "t",
		"type":  "analysis",
	})
	assertError(t, res, "add doc missing body")
}

func TestAddDoc_MissingType(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_add_doc", map[string]any{
		"slug":  "test",
		"title": "t",
		"body":  "b",
	})
	assertError(t, res, "add doc missing type")
}

func TestAddDoc_InvalidSlug(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_add_doc", map[string]any{
		"slug":  "INVALID SLUG!",
		"title": "t",
		"body":  "b",
		"type":  "analysis",
	})
	assertError(t, res, "add doc invalid slug")
}

func TestArchiveDoc_MissingID(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_archive_doc", map[string]any{})
	assertError(t, res, "archive doc missing id")
}

func TestArchiveDoc_NonExistent(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_archive_doc", map[string]any{"id_or_slug": "99-nope"})
	assertError(t, res, "archive doc nonexistent")
}

func TestArchiveDoc_Happy(t *testing.T) {
	_, s := setup(t)
	// Create a doc first.
	callTool(t, s, "tkt_add_doc", map[string]any{
		"slug":  "archive-me",
		"title": "To Archive",
		"body":  "body",
		"type":  "summary",
	})

	// Now archive it.
	res := callTool(t, s, "tkt_archive_doc", map[string]any{"id_or_slug": "archive-me"})
	assertOK(t, res, "archive doc happy")
	assertContains(t, res, "archived", "archived message")
}

func TestCreateRole_Happy(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_create_role", map[string]any{
		"name": "cave_tester",
		"like": "implementer",
	})
	assertOK(t, res, "create role happy")
	assertContains(t, res, "cave_tester", "role name in output")
}

func TestCreateRole_MissingName(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_create_role", map[string]any{"like": "implementer"})
	assertError(t, res, "create role missing name")
}

func TestCreateRole_MissingLike(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_create_role", map[string]any{"name": "testrole"})
	assertError(t, res, "create role missing like")
}

func TestCreateRole_InvalidBase(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_create_role", map[string]any{
		"name": "badrole",
		"like": "wizard",
	})
	assertError(t, res, "create role invalid base")
}

func TestDeleteRole_Happy(t *testing.T) {
	_, s := setup(t)
	// Create then delete.
	callTool(t, s, "tkt_create_role", map[string]any{
		"name": "temp_role",
		"like": "architect",
	})
	res := callTool(t, s, "tkt_delete_role", map[string]any{"name": "temp_role"})
	assertOK(t, res, "delete role happy")
	assertContains(t, res, "deleted", "deleted message")
}

func TestDeleteRole_MissingName(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_delete_role", map[string]any{})
	assertError(t, res, "delete role missing name")
}

func TestDeleteRole_NonExistent(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_delete_role", map[string]any{"name": "nonexistent_role_xyz"})
	assertError(t, res, "delete role nonexistent")
}

func TestDeleteRole_BuiltinBlocked(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_delete_role", map[string]any{"name": "architect"})
	// Built-in roles cannot be deleted.
	assertError(t, res, "delete builtin role")
}

func TestCleanup_DryRun(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_cleanup", map[string]any{"dry_run": true})
	assertOK(t, res, "cleanup dry run")
	assertContains(t, res, "dry_run", "dry run message")
}

func TestCleanup_Real(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_cleanup", map[string]any{})
	assertOK(t, res, "cleanup real")
	assertContains(t, res, "expired", "expired message")
}

// ---- readonly mode ----------------------------------------------------------

func TestReadonly_WriteToolsAbsent(t *testing.T) {
	_, s := setupReadonly(t)

	writeTools := []string{
		"tkt_new_ticket",
		"tkt_advance_ticket",
		"tkt_add_comment",
		"tkt_submit_plan",
		"tkt_add_depends",
		"tkt_set_tier",
		"tkt_archive_ticket",
		"tkt_log_usage",
		"tkt_add_context",
		"tkt_update_context",
		"tkt_delete_context",
		"tkt_add_doc",
		"tkt_archive_doc",
		"tkt_create_role",
		"tkt_delete_role",
		"tkt_cleanup",
	}

	for _, name := range writeTools {
		if tool := s.GetTool(name); tool != nil {
			t.Errorf("readonly: write tool %q should not be registered, but is", name)
		}
	}
}

func TestReadonly_ReadToolsPresent(t *testing.T) {
	_, s := setupReadonly(t)

	readTools := []string{
		"tkt_list_tickets",
		"tkt_show_ticket",
		"tkt_search_tickets",
		"tkt_batch",
		"tkt_list_context",
		"tkt_list_docs",
		"tkt_read_doc",
		"tkt_list_roles",
	}

	for _, name := range readTools {
		if tool := s.GetTool(name); tool == nil {
			t.Errorf("readonly: read tool %q should be registered, but is absent", name)
		}
	}
}

// ---- round-trip tests -------------------------------------------------------

func TestListDocs_AfterAddDoc(t *testing.T) {
	_, s := setup(t)
	callTool(t, s, "tkt_add_doc", map[string]any{
		"slug":  "my-doc",
		"title": "My Doc Title",
		"body":  "content here",
		"type":  "plan",
	})

	res := callTool(t, s, "tkt_list_docs", map[string]any{})
	assertOK(t, res, "list docs after add")
	assertContains(t, res, "My Doc Title", "doc title in list")
}

func TestReadDoc_AfterAddDoc(t *testing.T) {
	_, s := setup(t)
	callTool(t, s, "tkt_add_doc", map[string]any{
		"slug":  "readable-doc",
		"title": "Readable",
		"body":  "unique content xyz",
		"type":  "design",
	})

	res := callTool(t, s, "tkt_read_doc", map[string]any{"id_or_slug": "readable-doc"})
	assertOK(t, res, "read doc after add")
	assertContains(t, res, "unique content xyz", "body in read doc")
}

func TestListContext_AfterAddContext(t *testing.T) {
	_, s := setup(t)
	callTool(t, s, "tkt_add_context", map[string]any{
		"title": "My Context Entry",
		"body":  "very important caveat",
	})

	res := callTool(t, s, "tkt_list_context", map[string]any{})
	assertOK(t, res, "list context after add")
	assertContains(t, res, "My Context Entry", "context title in list")
}

func TestShowTicket_AfterPlan(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "With plan", models.StatusPlanning)

	callTool(t, s, "tkt_submit_plan", map[string]any{
		"id":   id,
		"body": "The plan: do stuff",
	})

	res := callTool(t, s, "tkt_show_ticket", map[string]any{"id": id})
	assertOK(t, res, "show ticket after plan")
	assertContains(t, res, "The plan: do stuff", "plan in show output")
}

// ---- edge cases -------------------------------------------------------------

func TestNewTicket_DefaultTier(t *testing.T) {
	_, s := setup(t)
	// No tier specified — should default to standard (no error).
	res := callTool(t, s, "tkt_new_ticket", map[string]any{
		"title": "Default tier ticket",
	})
	assertOK(t, res, "new ticket default tier")
}

func TestAdvanceTicket_WithExplicitTo(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Force status", models.StatusTodo)

	res := callTool(t, s, "tkt_advance_ticket", map[string]any{
		"id":    id,
		"note":  "moving to canceled directly",
		"to":    "CANCELED",
		"force": true,
	})
	assertOK(t, res, "advance with explicit to")
	assertContains(t, res, "CANCELED", "CANCELED in output")
}

func TestLogUsage_WithLabel(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Usage with label", models.StatusInProgress)

	res := callTool(t, s, "tkt_log_usage", map[string]any{
		"id":       id,
		"tokens":   float64(500),
		"duration": float64(120),
		"label":    "phase-one",
	})
	assertOK(t, res, "log usage with label")
	assertContains(t, res, "500 tokens", "token count")
}

func TestAddDoc_ListArchived(t *testing.T) {
	root, s := setup(t)

	// Create a doc.
	callTool(t, s, "tkt_add_doc", map[string]any{
		"slug":  "soon-archived",
		"title": "Will Be Archived",
		"body":  "body",
		"type":  "summary",
	})

	// Archive it.
	callTool(t, s, "tkt_archive_doc", map[string]any{"id_or_slug": "soon-archived"})

	// List archived docs.
	res := callTool(t, s, "tkt_list_docs", map[string]any{"archived": true})
	assertOK(t, res, "list archived docs")

	// List active docs — should be empty now.
	res2 := callTool(t, s, "tkt_list_docs", map[string]any{})
	text := resultText(res2)

	// Doc should no longer be in active list.
	docsDir := filepath.Join(root, ".tkt", "docs")
	entries, _ := os.ReadDir(docsDir)
	activeMDs := 0
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".md" {
			activeMDs++
		}
	}
	if activeMDs > 0 && strings.Contains(text, "Will Be Archived") {
		t.Errorf("archived doc should not appear in active list: %q", text)
	}
}
