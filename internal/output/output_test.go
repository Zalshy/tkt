package output

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/zalshy/tkt/internal/models"
)

// stripANSI removes ANSI escape sequences from s.
func stripANSI(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

func TestColorize(t *testing.T) {
	result := Colorize("hello", Blue)
	if !strings.HasPrefix(result, Blue) {
		t.Errorf("Colorize result does not start with color code: %q", result)
	}
	if !strings.HasSuffix(result, Reset) {
		t.Errorf("Colorize result does not end with Reset: %q", result)
	}
}

func TestStatusColor(t *testing.T) {
	statuses := []models.Status{
		models.StatusTodo,
		models.StatusPlanning,
		models.StatusInProgress,
		models.StatusDone,
		models.StatusVerified,
		models.StatusCanceled,
	}
	for _, s := range statuses {
		color := StatusColor(s)
		if color == "" {
			t.Errorf("StatusColor(%s) returned empty string", s)
		}
	}
}

func TestRenderList_Alignment(t *testing.T) {
	tickets := []models.Ticket{
		{ID: 1, Title: "First ticket"},
		{ID: 42, Title: "Forty-second ticket"},
	}
	result := RenderList(tickets, false, 0)
	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	// Both lines should have the same column start for the title (after #<id> padding).
	// "#1 " should be padded to "#42" width (3 chars), so "#1 " = 3 chars, then "  title".
	if !strings.HasPrefix(lines[0], "#1 ") {
		t.Errorf("line 0 should have #1 padded to width 3, got: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "#42") {
		t.Errorf("line 1 should start with #42, got: %q", lines[1])
	}
}

func TestRenderList_HasMoreHint(t *testing.T) {
	tickets := []models.Ticket{{ID: 1, Title: "A"}}
	result := RenderList(tickets, true, 0)
	if !strings.Contains(result, "more tickets") {
		t.Errorf("expected hint line in output, got: %q", result)
	}
}

func TestRenderList_HasMoreWithCount(t *testing.T) {
	tickets := []models.Ticket{{ID: 1, Title: "A"}}
	result := RenderList(tickets, true, 23)
	if !strings.Contains(result, "23 more tickets") {
		t.Errorf("expected count in hint line, got: %q", result)
	}
}

func TestRenderTicket_Header(t *testing.T) {
	ticket := models.Ticket{ID: 3, Status: models.StatusInProgress, Title: "implement session store"}
	result := stripANSI(RenderTicket(ticket, nil, nil))
	if !strings.Contains(result, separator) {
		t.Error("expected separator in output")
	}
	if !strings.Contains(result, "#3  ·  IN_PROGRESS") {
		t.Errorf("expected header line, got:\n%s", result)
	}
	if !strings.Contains(result, "implement session store") {
		t.Error("expected title in output")
	}
	sepIdx := strings.Index(result, separator)
	titleIdx := strings.Index(result, "implement session store")
	if titleIdx < sepIdx {
		t.Error("title should appear after first separator")
	}
}

func TestRenderTicket_SyntheticCreated(t *testing.T) {
	ticket := models.Ticket{
		ID: 1, Status: models.StatusTodo, Title: "T",
		CreatedBy:   "arch-alice",
		Description: "needs a persistent session store",
	}
	result := stripANSI(RenderTicket(ticket, nil, nil))
	if !strings.Contains(result, "arch-alice") {
		t.Error("expected CreatedBy in output")
	}
	if !strings.Contains(result, "○ created") {
		t.Error("expected synthetic created entry")
	}
	if !strings.Contains(result, "needs a persistent session store") {
		t.Error("expected description in output")
	}
	createdIdx := strings.Index(result, "○ created")
	descIdx := strings.Index(result, "needs a persistent session store")
	if descIdx <= createdIdx {
		t.Error("description should appear after ○ created line")
	}
}

func TestRenderTicket_TransitionEntry(t *testing.T) {
	from := models.StatusTodo
	to := models.StatusPlanning
	entries := []models.LogEntry{
		{Kind: "transition", SessionID: "impl-bob", Body: "picking this up", FromState: &from, ToState: &to},
	}
	result := stripANSI(RenderTicket(models.Ticket{ID: 1, Title: "T", CreatedBy: "arch"}, entries, nil))
	if !strings.Contains(result, "↳ TODO → PLANNING") {
		t.Errorf("expected transition arrow, got:\n%s", result)
	}
	if !strings.Contains(result, "picking this up") {
		t.Error("expected transition body")
	}
	// body should be indented (line starts with spaces)
	for _, line := range strings.Split(result, "\n") {
		if strings.Contains(line, "picking this up") {
			if !strings.HasPrefix(line, " ") {
				t.Errorf("transition body should be indented, got: %q", line)
			}
		}
	}
}

func TestRenderTicket_PlanEntry(t *testing.T) {
	entries := []models.LogEntry{
		{Kind: "plan", SessionID: "impl-bob", Body: "## Approach\nUse a single sessions table."},
	}
	result := stripANSI(RenderTicket(models.Ticket{ID: 1, Title: "T", CreatedBy: "arch"}, entries, nil))
	if !strings.Contains(result, "[plan]") {
		t.Error("expected [plan] label")
	}
	if !strings.Contains(result, "## Approach") {
		t.Error("expected plan body first line")
	}
	if !strings.Contains(result, "Use a single sessions table.") {
		t.Error("expected plan body second line")
	}
	for _, line := range strings.Split(result, "\n") {
		if strings.Contains(line, "## Approach") {
			if !strings.HasPrefix(line, " ") {
				t.Errorf("plan body should be indented, got: %q", line)
			}
		}
	}
}

func TestRenderTicket_MessageEntry(t *testing.T) {
	entries := []models.LogEntry{
		{Kind: "message", SessionID: "impl-bob", Body: "this is a message entry"},
	}
	result := stripANSI(RenderTicket(models.Ticket{ID: 1, Title: "T", CreatedBy: "arch"}, entries, nil))
	if !strings.Contains(result, "this is a message entry") {
		t.Error("expected message body")
	}
	// body should be on same line as session ID
	for _, line := range strings.Split(result, "\n") {
		if strings.Contains(line, "this is a message entry") {
			if !strings.Contains(line, "impl-bob") {
				t.Errorf("message body and session ID should be on same line, got: %q", line)
			}
		}
	}
}

func TestRenderTicket_Footer(t *testing.T) {
	entries := []models.LogEntry{
		{Kind: "message", SessionID: "impl-bob", Body: "first", CreatedAt: time.Now().Add(-3 * time.Hour)},
		{Kind: "message", SessionID: "impl-carol", Body: "second", CreatedAt: time.Now().Add(-2 * time.Hour)},
	}
	ticket := models.Ticket{ID: 1, Title: "T", CreatedBy: "arch-alice"}
	result := stripANSI(RenderTicket(ticket, entries, nil))
	if !strings.Contains(result, "3 sessions") {
		t.Errorf("expected 3 sessions, got:\n%s", result)
	}
	if !strings.Contains(result, "2 entries") {
		t.Errorf("expected 2 entries, got:\n%s", result)
	}
	if !strings.Contains(result, "2h ago") {
		t.Errorf("expected 2h ago, got:\n%s", result)
	}
}

func TestRenderTicket_EmptyEntries(t *testing.T) {
	ticket := models.Ticket{
		ID: 1, Title: "T",
		CreatedBy: "arch-alice",
		CreatedAt: time.Now().Add(-1 * time.Hour),
	}
	result := stripANSI(RenderTicket(ticket, nil, nil))
	if !strings.Contains(result, "○ created") {
		t.Error("expected synthetic created entry")
	}
	if !strings.Contains(result, "0 entries") {
		t.Errorf("expected 0 entries in footer, got:\n%s", result)
	}
	if !strings.Contains(result, "1 sessions") {
		t.Errorf("expected 1 sessions in footer, got:\n%s", result)
	}
	if !strings.Contains(result, "1h ago") {
		t.Errorf("expected 1h ago, got:\n%s", result)
	}
}

func TestRenderDependencies_Empty(t *testing.T) {
	result := RenderDependencies(nil)
	if result != "" {
		t.Errorf("expected empty string for nil deps, got: %q", result)
	}
}

func TestRenderDependencies_AllBlocked(t *testing.T) {
	deps := []models.Ticket{
		{ID: 7, Status: models.StatusTodo},
		{ID: 9, Status: models.StatusTodo},
	}
	result := stripANSI(RenderDependencies(deps))
	if !strings.Contains(result, "Blocked by 2") {
		t.Errorf("expected 'Blocked by 2', got:\n%s", result)
	}
	if strings.Count(result, "○") != 2 {
		t.Errorf("expected 2 '○' glyphs, got:\n%s", result)
	}
	if strings.Count(result, "← blocking") != 2 {
		t.Errorf("expected 2 '← blocking' suffixes, got:\n%s", result)
	}
	if strings.Contains(result, "✓") {
		t.Errorf("expected no '✓' glyphs, got:\n%s", result)
	}
}

func TestRenderDependencies_AllVerified(t *testing.T) {
	deps := []models.Ticket{
		{ID: 5, Status: models.StatusVerified},
		{ID: 6, Status: models.StatusVerified},
	}
	result := stripANSI(RenderDependencies(deps))
	if !strings.Contains(result, "All dependencies resolved.") {
		t.Errorf("expected 'All dependencies resolved.', got:\n%s", result)
	}
	if strings.Count(result, "✓") != 2 {
		t.Errorf("expected 2 '✓' glyphs, got:\n%s", result)
	}
	if strings.Contains(result, "← blocking") {
		t.Errorf("expected no '← blocking' suffix, got:\n%s", result)
	}
}

func TestRenderTicket_ColumnAlignment(t *testing.T) {
	// "a-very-long-session-id" is 22 chars — widest ID, sets maxWidth
	entries := []models.LogEntry{
		{Kind: "message", SessionID: "impl-bob", Body: "short"},              // 8 chars
		{Kind: "message", SessionID: "a-very-long-session-id", Body: "long"}, // 22 chars
	}
	// t.CreatedBy "arch-alice" is 10 chars
	ticket := models.Ticket{ID: 1, Title: "T", CreatedBy: "arch-alice"}
	result := stripANSI(RenderTicket(ticket, entries, nil))

	// impl-bob (8 chars) should be padded to 22, then 4 spaces = 26 chars before content
	for _, line := range strings.Split(result, "\n") {
		if strings.Contains(line, "short") && !strings.HasPrefix(line, " ") {
			// impl-bob line: should be "impl-bob" + 14 spaces + "    short"
			expected := "impl-bob" + strings.Repeat(" ", 14) + "    short"
			if !strings.HasPrefix(line, "impl-bob"+strings.Repeat(" ", 14)) {
				t.Errorf("impl-bob line not padded correctly, got: %q, want prefix: %q",
					line, "impl-bob"+strings.Repeat(" ", 14))
			}
			_ = expected
		}
	}
}
