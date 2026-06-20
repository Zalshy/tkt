package mcp

import (
	"fmt"
	"testing"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/models"
)

// ---- helpers -----------------------------------------------------------------

// seedMCPSession inserts a session row directly into the DB with the given id, name,
// and role, mirroring cmd/session_resolve_test.go's seedSessionIDName pattern but kept
// local to this package (mcp can't import the cmd package). Returns nothing — caller
// already knows id/name since they choose them.
func seedMCPSession(t *testing.T, root, id, name, role string) {
	t.Helper()
	database, err := db.Open(root)
	if err != nil {
		t.Fatalf("seedMCPSession: open db: %v", err)
	}
	defer database.Close()

	if _, err := database.Exec(
		`INSERT INTO sessions (id, role, name, created_at, last_active)
		 VALUES (?, ?, ?, datetime('now'), datetime('now'))`,
		id, role, name,
	); err != nil {
		t.Fatalf("seedMCPSession: insert: %v", err)
	}
}

// seedExpiredMCPSession inserts a session row with expired_at set in the past.
func seedExpiredMCPSession(t *testing.T, root, id, name, role string) {
	t.Helper()
	database, err := db.Open(root)
	if err != nil {
		t.Fatalf("seedExpiredMCPSession: open db: %v", err)
	}
	defer database.Close()

	if _, err := database.Exec(
		`INSERT INTO sessions (id, role, name, created_at, last_active, expired_at)
		 VALUES (?, ?, ?, datetime('now'), datetime('now'), datetime('2000-01-01'))`,
		id, role, name,
	); err != nil {
		t.Fatalf("seedExpiredMCPSession: insert: %v", err)
	}
}

// lastTicketLogSessionName queries the most recent ticket_log row's session_name for
// the given ticket id, used to assert attribution.
func lastTicketLogSessionName(t *testing.T, root string, ticketID string) string {
	t.Helper()
	database, err := db.Open(root)
	if err != nil {
		t.Fatalf("lastTicketLogSessionName: open db: %v", err)
	}
	defer database.Close()

	var sessionName string
	err = database.QueryRow(
		`SELECT session_name FROM ticket_log WHERE ticket_id = ? ORDER BY id DESC LIMIT 1`,
		ticketID,
	).Scan(&sessionName)
	if err != nil {
		t.Fatalf("lastTicketLogSessionName: query: %v", err)
	}
	return sessionName
}

// ticketCreatedBy queries a ticket's created_by column.
func ticketCreatedBy(t *testing.T, root string, ticketID string) string {
	t.Helper()
	database, err := db.Open(root)
	if err != nil {
		t.Fatalf("ticketCreatedBy: open db: %v", err)
	}
	defer database.Close()

	var createdBy string
	err = database.QueryRow(`SELECT created_by FROM tickets WHERE id = ?`, ticketID).Scan(&createdBy)
	if err != nil {
		t.Fatalf("ticketCreatedBy: query: %v", err)
	}
	return createdBy
}

// contextSessionName queries the most recent project_context row's session_name.
func contextSessionName(t *testing.T, root string) string {
	t.Helper()
	database, err := db.Open(root)
	if err != nil {
		t.Fatalf("contextSessionName: open db: %v", err)
	}
	defer database.Close()

	var sessionName string
	err = database.QueryRow(
		`SELECT session_name FROM project_context ORDER BY id DESC LIMIT 1`,
	).Scan(&sessionName)
	if err != nil {
		t.Fatalf("contextSessionName: query: %v", err)
	}
	return sessionName
}

// contextTitleAndBody queries a project_context row's current title and body.
func contextTitleAndBody(t *testing.T, root string, id int) (string, string) {
	t.Helper()
	database, err := db.Open(root)
	if err != nil {
		t.Fatalf("contextTitleAndBody: open db: %v", err)
	}
	defer database.Close()

	var title, body string
	err = database.QueryRow(
		`SELECT title, body FROM project_context WHERE id = ?`, id,
	).Scan(&title, &body)
	if err != nil {
		t.Fatalf("contextTitleAndBody: query: %v", err)
	}
	return title, body
}

// usageSessionName queries the most recent ticket_usage row's session_name.
func usageSessionName(t *testing.T, root string) string {
	t.Helper()
	database, err := db.Open(root)
	if err != nil {
		t.Fatalf("usageSessionName: open db: %v", err)
	}
	defer database.Close()

	var sessionName string
	err = database.QueryRow(
		`SELECT session_name FROM ticket_usage ORDER BY id DESC LIMIT 1`,
	).Scan(&sessionName)
	if err != nil {
		t.Fatalf("usageSessionName: query: %v", err)
	}
	return sessionName
}

// ---- tkt_new_ticket: session param matrix -------------------------------------

func TestNewTicket_Session_ValidID(t *testing.T) {
	root, s := setup(t)
	seedMCPSession(t, root, "new-sess-id", "new-sess-name", "implementer")

	res := callTool(t, s, "tkt_new_ticket", map[string]any{
		"title":   "Session-attributed ticket",
		"session": "new-sess-id",
	})
	assertOK(t, res, "new ticket session by id")

	id := extractTicketID(t, resultText(res))
	if got := ticketCreatedBy(t, root, id); got != "new-sess-name" {
		t.Errorf("created_by = %q, want %q", got, "new-sess-name")
	}
}

func TestNewTicket_Session_ValidName(t *testing.T) {
	root, s := setup(t)
	seedMCPSession(t, root, "new-sess-id2", "new-sess-name2", "implementer")

	res := callTool(t, s, "tkt_new_ticket", map[string]any{
		"title":   "Session-attributed ticket by name",
		"session": "new-sess-name2",
	})
	assertOK(t, res, "new ticket session by name")

	id := extractTicketID(t, resultText(res))
	if got := ticketCreatedBy(t, root, id); got != "new-sess-name2" {
		t.Errorf("created_by = %q, want %q", got, "new-sess-name2")
	}
}

func TestNewTicket_Session_InvalidValue_NoSilentFallback(t *testing.T) {
	root, s := setup(t)

	res := callTool(t, s, "tkt_new_ticket", map[string]any{
		"title":   "Should not be created under startup sess",
		"session": "totally-bogus-session",
	})
	assertError(t, res, "new ticket invalid session")
	assertContains(t, res, "totally-bogus-session", "error mentions value")

	// Confirm no ticket was silently created under the startup session.
	database, err := db.Open(root)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM tickets`).Scan(&count); err != nil {
		t.Fatalf("count tickets: %v", err)
	}
	if count != 0 {
		t.Errorf("expected no tickets created, got %d", count)
	}
}

func TestNewTicket_Session_Expired(t *testing.T) {
	root, s := setup(t)
	seedExpiredMCPSession(t, root, "new-sess-expired-id", "new-sess-expired-name", "implementer")

	res := callTool(t, s, "tkt_new_ticket", map[string]any{
		"title":   "Should not be created",
		"session": "new-sess-expired-id",
	})
	assertError(t, res, "new ticket expired session")
	assertContains(t, res, "expired", "error mentions expiry")
}

func TestNewTicket_Session_NoParam_Regression(t *testing.T) {
	// Regression: identical to TestNewTicket_Happy — no session param means startup sess used.
	_, s := setup(t)
	res := callTool(t, s, "tkt_new_ticket", map[string]any{"title": "No session param"})
	assertOK(t, res, "new ticket no session param")
}

// ---- tkt_add_comment: session param matrix ------------------------------------

func TestAddComment_Session_ValidID(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Commentable", models.StatusTodo)
	seedMCPSession(t, root, "comment-sess-id", "comment-sess-name", "implementer")

	res := callTool(t, s, "tkt_add_comment", map[string]any{
		"id":      id,
		"body":    "session-attributed comment",
		"session": "comment-sess-id",
	})
	assertOK(t, res, "add comment session by id")
	if got := lastTicketLogSessionName(t, root, id); got != "comment-sess-name" {
		t.Errorf("ticket_log.session_name = %q, want %q", got, "comment-sess-name")
	}
}

func TestAddComment_Session_ValidName(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Commentable", models.StatusTodo)
	seedMCPSession(t, root, "comment-sess-id2", "comment-sess-name2", "implementer")

	res := callTool(t, s, "tkt_add_comment", map[string]any{
		"id":      id,
		"body":    "session-attributed comment by name",
		"session": "comment-sess-name2",
	})
	assertOK(t, res, "add comment session by name")
	if got := lastTicketLogSessionName(t, root, id); got != "comment-sess-name2" {
		t.Errorf("ticket_log.session_name = %q, want %q", got, "comment-sess-name2")
	}
}

func TestAddComment_Session_InvalidValue(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Commentable", models.StatusTodo)

	res := callTool(t, s, "tkt_add_comment", map[string]any{
		"id":      id,
		"body":    "should not be written",
		"session": "totally-bogus-session",
	})
	assertError(t, res, "add comment invalid session")
	assertContains(t, res, "totally-bogus-session", "error mentions value")
}

func TestAddComment_Session_Expired(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Commentable", models.StatusTodo)
	seedExpiredMCPSession(t, root, "comment-sess-expired-id", "comment-sess-expired-name", "implementer")

	res := callTool(t, s, "tkt_add_comment", map[string]any{
		"id":      id,
		"body":    "should not be written",
		"session": "comment-sess-expired-id",
	})
	assertError(t, res, "add comment expired session")
	assertContains(t, res, "expired", "error mentions expiry")
}

func TestAddComment_Session_NoParam_Regression(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Commentable", models.StatusTodo)
	res := callTool(t, s, "tkt_add_comment", map[string]any{
		"id":   id,
		"body": "no session param",
	})
	assertOK(t, res, "add comment no session param")
}

// ---- tkt_submit_plan: session param matrix ------------------------------------

func TestSubmitPlan_Session_ValidID(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Plannable", models.StatusPlanning)
	seedMCPSession(t, root, "plan-sess-id", "plan-sess-name", "implementer")

	res := callTool(t, s, "tkt_submit_plan", map[string]any{
		"id":      id,
		"body":    "session-attributed plan",
		"session": "plan-sess-id",
	})
	assertOK(t, res, "submit plan session by id")
	if got := lastTicketLogSessionName(t, root, id); got != "plan-sess-name" {
		t.Errorf("ticket_log.session_name = %q, want %q", got, "plan-sess-name")
	}
}

func TestSubmitPlan_Session_ValidName(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Plannable", models.StatusPlanning)
	seedMCPSession(t, root, "plan-sess-id2", "plan-sess-name2", "implementer")

	res := callTool(t, s, "tkt_submit_plan", map[string]any{
		"id":      id,
		"body":    "session-attributed plan by name",
		"session": "plan-sess-name2",
	})
	assertOK(t, res, "submit plan session by name")
	if got := lastTicketLogSessionName(t, root, id); got != "plan-sess-name2" {
		t.Errorf("ticket_log.session_name = %q, want %q", got, "plan-sess-name2")
	}
}

func TestSubmitPlan_Session_InvalidValue(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Plannable", models.StatusPlanning)

	res := callTool(t, s, "tkt_submit_plan", map[string]any{
		"id":      id,
		"body":    "should not be written",
		"session": "totally-bogus-session",
	})
	assertError(t, res, "submit plan invalid session")
	assertContains(t, res, "totally-bogus-session", "error mentions value")
}

func TestSubmitPlan_Session_Expired(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Plannable", models.StatusPlanning)
	seedExpiredMCPSession(t, root, "plan-sess-expired-id", "plan-sess-expired-name", "implementer")

	res := callTool(t, s, "tkt_submit_plan", map[string]any{
		"id":      id,
		"body":    "should not be written",
		"session": "plan-sess-expired-id",
	})
	assertError(t, res, "submit plan expired session")
	assertContains(t, res, "expired", "error mentions expiry")
}

func TestSubmitPlan_Session_NoParam_Regression(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Plannable", models.StatusPlanning)
	res := callTool(t, s, "tkt_submit_plan", map[string]any{
		"id":   id,
		"body": "no session param",
	})
	assertOK(t, res, "submit plan no session param")
}

// ---- tkt_archive_ticket: session param matrix ---------------------------------

func TestArchiveTicket_Session_ValidID(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Verified ticket", models.StatusVerified)
	seedMCPSession(t, root, "archive-sess-id", "archive-sess-name", "implementer")

	res := callTool(t, s, "tkt_archive_ticket", map[string]any{
		"id":      id,
		"session": "archive-sess-id",
	})
	assertOK(t, res, "archive session by id")
	if got := lastTicketLogSessionName(t, root, id); got != "archive-sess-name" {
		t.Errorf("ticket_log.session_name = %q, want %q", got, "archive-sess-name")
	}
}

func TestArchiveTicket_Session_ValidName(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Verified ticket", models.StatusVerified)
	seedMCPSession(t, root, "archive-sess-id2", "archive-sess-name2", "implementer")

	res := callTool(t, s, "tkt_archive_ticket", map[string]any{
		"id":      id,
		"session": "archive-sess-name2",
	})
	assertOK(t, res, "archive session by name")
	if got := lastTicketLogSessionName(t, root, id); got != "archive-sess-name2" {
		t.Errorf("ticket_log.session_name = %q, want %q", got, "archive-sess-name2")
	}
}

func TestArchiveTicket_Session_InvalidValue(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Verified ticket", models.StatusVerified)

	res := callTool(t, s, "tkt_archive_ticket", map[string]any{
		"id":      id,
		"session": "totally-bogus-session",
	})
	assertError(t, res, "archive invalid session")
	assertContains(t, res, "totally-bogus-session", "error mentions value")
}

func TestArchiveTicket_Session_Expired(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Verified ticket", models.StatusVerified)
	seedExpiredMCPSession(t, root, "archive-sess-expired-id", "archive-sess-expired-name", "implementer")

	res := callTool(t, s, "tkt_archive_ticket", map[string]any{
		"id":      id,
		"session": "archive-sess-expired-id",
	})
	assertError(t, res, "archive expired session")
	assertContains(t, res, "expired", "error mentions expiry")
}

func TestArchiveTicket_Session_NoParam_Regression(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Verified ticket", models.StatusVerified)
	res := callTool(t, s, "tkt_archive_ticket", map[string]any{"id": id})
	assertOK(t, res, "archive no session param")
}

// ---- tkt_log_usage: session param matrix --------------------------------------

func TestLogUsage_Session_ValidID(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Usage target", models.StatusInProgress)
	seedMCPSession(t, root, "usage-sess-id", "usage-sess-name", "implementer")

	res := callTool(t, s, "tkt_log_usage", map[string]any{
		"id":      id,
		"tokens":  float64(100),
		"session": "usage-sess-id",
	})
	assertOK(t, res, "log usage session by id")
	if got := usageSessionName(t, root); got != "usage-sess-name" {
		t.Errorf("ticket_usage.session_name = %q, want %q", got, "usage-sess-name")
	}
}

func TestLogUsage_Session_ValidName(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Usage target", models.StatusInProgress)
	seedMCPSession(t, root, "usage-sess-id2", "usage-sess-name2", "implementer")

	res := callTool(t, s, "tkt_log_usage", map[string]any{
		"id":      id,
		"tokens":  float64(100),
		"session": "usage-sess-name2",
	})
	assertOK(t, res, "log usage session by name")
	if got := usageSessionName(t, root); got != "usage-sess-name2" {
		t.Errorf("ticket_usage.session_name = %q, want %q", got, "usage-sess-name2")
	}
}

func TestLogUsage_Session_InvalidValue(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Usage target", models.StatusInProgress)

	res := callTool(t, s, "tkt_log_usage", map[string]any{
		"id":      id,
		"tokens":  float64(100),
		"session": "totally-bogus-session",
	})
	assertError(t, res, "log usage invalid session")
	assertContains(t, res, "totally-bogus-session", "error mentions value")
}

func TestLogUsage_Session_Expired(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Usage target", models.StatusInProgress)
	seedExpiredMCPSession(t, root, "usage-sess-expired-id", "usage-sess-expired-name", "implementer")

	res := callTool(t, s, "tkt_log_usage", map[string]any{
		"id":      id,
		"tokens":  float64(100),
		"session": "usage-sess-expired-id",
	})
	assertError(t, res, "log usage expired session")
	assertContains(t, res, "expired", "error mentions expiry")
}

func TestLogUsage_Session_NoParam_Regression(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Usage target", models.StatusInProgress)
	res := callTool(t, s, "tkt_log_usage", map[string]any{
		"id":     id,
		"tokens": float64(100),
	})
	assertOK(t, res, "log usage no session param")
}

// ---- tkt_add_context: session param matrix ------------------------------------

func TestAddContext_Session_ValidID(t *testing.T) {
	root, s := setup(t)
	seedMCPSession(t, root, "ctx-sess-id", "ctx-sess-name", "implementer")

	res := callTool(t, s, "tkt_add_context", map[string]any{
		"title":   "Session-attributed context",
		"body":    "body text",
		"session": "ctx-sess-id",
	})
	assertOK(t, res, "add context session by id")
	if got := contextSessionName(t, root); got != "ctx-sess-name" {
		t.Errorf("project_context.session_name = %q, want %q", got, "ctx-sess-name")
	}
}

func TestAddContext_Session_ValidName(t *testing.T) {
	root, s := setup(t)
	seedMCPSession(t, root, "ctx-sess-id2", "ctx-sess-name2", "implementer")

	res := callTool(t, s, "tkt_add_context", map[string]any{
		"title":   "Session-attributed context by name",
		"body":    "body text",
		"session": "ctx-sess-name2",
	})
	assertOK(t, res, "add context session by name")
	if got := contextSessionName(t, root); got != "ctx-sess-name2" {
		t.Errorf("project_context.session_name = %q, want %q", got, "ctx-sess-name2")
	}
}

func TestAddContext_Session_InvalidValue(t *testing.T) {
	_, s := setup(t)

	res := callTool(t, s, "tkt_add_context", map[string]any{
		"title":   "Should not be created",
		"body":    "body",
		"session": "totally-bogus-session",
	})
	assertError(t, res, "add context invalid session")
	assertContains(t, res, "totally-bogus-session", "error mentions value")
}

func TestAddContext_Session_Expired(t *testing.T) {
	root, s := setup(t)
	seedExpiredMCPSession(t, root, "ctx-sess-expired-id", "ctx-sess-expired-name", "implementer")

	res := callTool(t, s, "tkt_add_context", map[string]any{
		"title":   "Should not be created",
		"body":    "body",
		"session": "ctx-sess-expired-id",
	})
	assertError(t, res, "add context expired session")
	assertContains(t, res, "expired", "error mentions expiry")
}

func TestAddContext_Session_NoParam_Regression(t *testing.T) {
	_, s := setup(t)
	res := callTool(t, s, "tkt_add_context", map[string]any{
		"title": "No session param",
		"body":  "body",
	})
	assertOK(t, res, "add context no session param")
}

// ---- tkt_update_context: session param matrix ---------------------------------
//
// NOTE: internal/context.Update discards its actor parameter (`_ = actor`) and
// never writes session_name — that column is set once by Add and is never
// mutated by Update. So there is no attribution column to read back here,
// unlike the other 7 tools. What IS observable and worth asserting on the
// happy path: the override session gates whether the call is allowed to
// proceed at all (same validation resolveActingSession already applies
// everywhere else), and the content change actually lands. The InvalidValue
// and Expired tests below already prove the gate rejects bad overrides; the
// ValidID/ValidName tests below now also assert the content mutation
// happened, so a future regression that silently no-ops on a valid override
// would be caught even though there's no session_name to compare.

func TestUpdateContext_Session_ValidID(t *testing.T) {
	root, s := setup(t)
	addRes := callTool(t, s, "tkt_add_context", map[string]any{"title": "Original", "body": "Original body"})
	assertOK(t, addRes, "add before update")
	seedMCPSession(t, root, "ctxupd-sess-id", "ctxupd-sess-name", "implementer")

	res := callTool(t, s, "tkt_update_context", map[string]any{
		"id":      "1",
		"title":   "Updated via id override",
		"body":    "Updated body via id override",
		"session": "ctxupd-sess-id",
	})
	assertOK(t, res, "update context session by id")

	gotTitle, gotBody := contextTitleAndBody(t, root, 1)
	if gotTitle != "Updated via id override" || gotBody != "Updated body via id override" {
		t.Errorf("content after override update = (%q, %q), want (%q, %q)",
			gotTitle, gotBody, "Updated via id override", "Updated body via id override")
	}
}

func TestUpdateContext_Session_ValidName(t *testing.T) {
	root, s := setup(t)
	addRes := callTool(t, s, "tkt_add_context", map[string]any{"title": "Original", "body": "Original body"})
	assertOK(t, addRes, "add before update")
	seedMCPSession(t, root, "ctxupd-sess-id2", "ctxupd-sess-name2", "implementer")

	res := callTool(t, s, "tkt_update_context", map[string]any{
		"id":      "1",
		"title":   "Updated via name override",
		"body":    "Updated body via name override",
		"session": "ctxupd-sess-name2",
	})
	assertOK(t, res, "update context session by name")

	gotTitle, gotBody := contextTitleAndBody(t, root, 1)
	if gotTitle != "Updated via name override" || gotBody != "Updated body via name override" {
		t.Errorf("content after override update = (%q, %q), want (%q, %q)",
			gotTitle, gotBody, "Updated via name override", "Updated body via name override")
	}
}

func TestUpdateContext_Session_InvalidValue(t *testing.T) {
	_, s := setup(t)
	addRes := callTool(t, s, "tkt_add_context", map[string]any{"title": "Original", "body": "Original body"})
	assertOK(t, addRes, "add before update")

	res := callTool(t, s, "tkt_update_context", map[string]any{
		"id":      "1",
		"title":   "Should not update",
		"body":    "Should not update",
		"session": "totally-bogus-session",
	})
	assertError(t, res, "update context invalid session")
	assertContains(t, res, "totally-bogus-session", "error mentions value")
}

func TestUpdateContext_Session_Expired(t *testing.T) {
	root, s := setup(t)
	addRes := callTool(t, s, "tkt_add_context", map[string]any{"title": "Original", "body": "Original body"})
	assertOK(t, addRes, "add before update")
	seedExpiredMCPSession(t, root, "ctxupd-sess-expired-id", "ctxupd-sess-expired-name", "implementer")

	res := callTool(t, s, "tkt_update_context", map[string]any{
		"id":      "1",
		"title":   "Should not update",
		"body":    "Should not update",
		"session": "ctxupd-sess-expired-id",
	})
	assertError(t, res, "update context expired session")
	assertContains(t, res, "expired", "error mentions expiry")
}

func TestUpdateContext_Session_NoParam_Regression(t *testing.T) {
	_, s := setup(t)
	addRes := callTool(t, s, "tkt_add_context", map[string]any{"title": "Original", "body": "Original body"})
	assertOK(t, addRes, "add before update")

	res := callTool(t, s, "tkt_update_context", map[string]any{
		"id":    "1",
		"title": "Updated",
		"body":  "Updated body",
	})
	assertOK(t, res, "update context no session param")
}

// ---- tkt_advance_ticket: session param matrix + composition cases -------------

func TestAdvanceTicket_Session_ValidID(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Advanceable", models.StatusTodo)
	seedMCPSession(t, root, "advance-sess-id", "advance-sess-name", "implementer")

	res := callTool(t, s, "tkt_advance_ticket", map[string]any{
		"id":      id,
		"note":    "session-attributed advance",
		"session": "advance-sess-id",
	})
	assertOK(t, res, "advance session by id")
	if got := lastTicketLogSessionName(t, root, id); got != "advance-sess-name" {
		t.Errorf("ticket_log.session_name = %q, want %q", got, "advance-sess-name")
	}
}

func TestAdvanceTicket_Session_ValidName(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Advanceable", models.StatusTodo)
	seedMCPSession(t, root, "advance-sess-id2", "advance-sess-name2", "implementer")

	res := callTool(t, s, "tkt_advance_ticket", map[string]any{
		"id":      id,
		"note":    "session-attributed advance by name",
		"session": "advance-sess-name2",
	})
	assertOK(t, res, "advance session by name")
	if got := lastTicketLogSessionName(t, root, id); got != "advance-sess-name2" {
		t.Errorf("ticket_log.session_name = %q, want %q", got, "advance-sess-name2")
	}
}

func TestAdvanceTicket_Session_InvalidValue(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Advanceable", models.StatusTodo)

	res := callTool(t, s, "tkt_advance_ticket", map[string]any{
		"id":      id,
		"note":    "should not advance",
		"session": "totally-bogus-session",
	})
	assertError(t, res, "advance invalid session")
	assertContains(t, res, "totally-bogus-session", "error mentions value")
}

func TestAdvanceTicket_Session_Expired(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Advanceable", models.StatusTodo)
	seedExpiredMCPSession(t, root, "advance-sess-expired-id", "advance-sess-expired-name", "implementer")

	res := callTool(t, s, "tkt_advance_ticket", map[string]any{
		"id":      id,
		"note":    "should not advance",
		"session": "advance-sess-expired-id",
	})
	assertError(t, res, "advance expired session")
	assertContains(t, res, "expired", "error mentions expiry")
}

func TestAdvanceTicket_Session_NoParam_Regression(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Advanceable", models.StatusTodo)
	res := callTool(t, s, "tkt_advance_ticket", map[string]any{
		"id":   id,
		"note": "no session param",
	})
	assertOK(t, res, "advance no session param")
}

// TestAdvanceTicket_SessionOnly_NoAs verifies session resolves actingSess and, since the
// resolved session is not orchestrator-role, no `as` is required — advance proceeds
// attributed to the session-resolved identity.
func TestAdvanceTicket_SessionOnly_NoAs(t *testing.T) {
	root, s := setup(t)
	id := seedTicket(t, root, "Advanceable", models.StatusTodo)
	seedMCPSession(t, root, "compose-sess-id", "compose-sess-name", "implementer")

	res := callTool(t, s, "tkt_advance_ticket", map[string]any{
		"id":      id,
		"note":    "session only, no as",
		"session": "compose-sess-id",
	})
	assertOK(t, res, "advance session-only no as")
	if got := lastTicketLogSessionName(t, root, id); got != "compose-sess-name" {
		t.Errorf("ticket_log.session_name = %q, want %q", got, "compose-sess-name")
	}
}

// TestAdvanceTicket_SessionPlusAs verifies: session resolves the caller to an
// orchestrator-role session, then `as` delegates further to an architect/implementer
// session — final attribution must be the `as`-delegated session, not the
// session-resolved orchestrator and not the startup sess.
func TestAdvanceTicket_SessionPlusAs(t *testing.T) {
	root, s := setup(t) // startup sess is implementer-role
	id := seedTicket(t, root, "Advanceable", models.StatusTodo)
	seedMCPSession(t, root, "orch-sess-id", "orch-sess-name", "orchestrator")
	seedMCPSession(t, root, "delegate-sess-id", "delegate-sess-name", "architect")

	res := callTool(t, s, "tkt_advance_ticket", map[string]any{
		"id":      id,
		"note":    "session resolves orchestrator, as delegates to architect",
		"session": "orch-sess-id",
		"as":      "delegate-sess-name",
	})
	assertOK(t, res, "advance session+as")
	if got := lastTicketLogSessionName(t, root, id); got != "delegate-sess-name" {
		t.Errorf("ticket_log.session_name = %q, want %q (as-delegated identity)", got, "delegate-sess-name")
	}
}

// TestAdvanceTicket_AsOnly_Regression proves the as-only path (no session param) behaves
// identically to pre-#254 code: startup sess must be orchestrator-role for `as` to be
// accepted at all, and delegation proceeds against the startup sess exactly as before.
func TestAdvanceTicket_AsOnly_Regression(t *testing.T) {
	root, mcpServer := setupWithRole(t, "orchestrator")
	id := seedTicket(t, root, "Advanceable", models.StatusTodo)
	seedMCPSession(t, root, "asonly-delegate-id", "asonly-delegate-name", "implementer")

	res := callTool(t, mcpServer, "tkt_advance_ticket", map[string]any{
		"id":   id,
		"note": "as only, no session",
		"as":   "asonly-delegate-name",
	})
	assertOK(t, res, "advance as-only")
	if got := lastTicketLogSessionName(t, root, id); got != "asonly-delegate-name" {
		t.Errorf("ticket_log.session_name = %q, want %q", got, "asonly-delegate-name")
	}
}

// TestAdvanceTicket_SessionResolvesNonOrchestrator_AsSupplied verifies that when `session`
// resolves to a non-orchestrator-role identity and `as` is also supplied, the existing
// "as is only valid for orchestrator sessions" error fires — now evaluated against the
// resolved identity rather than the fixed startup sess.
func TestAdvanceTicket_SessionResolvesNonOrchestrator_AsSupplied(t *testing.T) {
	root, s := setup(t) // startup sess is implementer-role — would also reject `as` on its own
	id := seedTicket(t, root, "Advanceable", models.StatusTodo)
	seedMCPSession(t, root, "nonorch-sess-id", "nonorch-sess-name", "architect")
	seedMCPSession(t, root, "nonorch-target-id", "nonorch-target-name", "implementer")

	res := callTool(t, s, "tkt_advance_ticket", map[string]any{
		"id":      id,
		"note":    "session resolves architect, as supplied — should be rejected",
		"session": "nonorch-sess-id",
		"as":      "nonorch-target-name",
	})
	assertError(t, res, "advance session-resolves-non-orchestrator with as")
	assertContains(t, res, "is only valid for orchestrator sessions", "as-rejection message")
}

// extractTicketID extracts the numeric ticket id from a "Created #N ..." style result
// string produced by tkt_new_ticket.
func extractTicketID(t *testing.T, text string) string {
	t.Helper()
	var n int
	if _, err := fmt.Sscanf(text, "Created #%d", &n); err != nil {
		t.Fatalf("could not extract ticket id from %q: %v", text, err)
	}
	return fmt.Sprintf("%d", n)
}

// setupWithRole creates a fresh tkt dir + db + session with the given role (mirrors
// setup() but lets advance-ticket `as`-only regression tests start from an
// orchestrator-role startup session instead of the default implementer).
func setupWithRole(t *testing.T, role string) (string, *mcpserver.MCPServer) {
	t.Helper()
	root := t.TempDir()
	mkTktDir(t, root)

	database, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	sess, err := EnsureMCPSession(database, role)
	if err != nil {
		t.Fatalf("EnsureMCPSession: %v", err)
	}

	s := NewServer(root, database, sess)
	return root, s
}
