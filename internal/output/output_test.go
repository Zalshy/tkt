package output

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/zalshy/tkt/internal/models"
	statsPkg "github.com/zalshy/tkt/internal/stats"
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
		{Kind: "transition", SessionName: "impl-bob", Body: "picking this up", FromState: &from, ToState: &to},
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
		{Kind: "plan", SessionName: "impl-bob", Body: "## Approach\nUse a single sessions table."},
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
		{Kind: "message", SessionName: "impl-bob", Body: "this is a message entry"},
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
		{Kind: "message", SessionName: "impl-bob", Body: "first", CreatedAt: time.Now().Add(-3 * time.Hour)},
		{Kind: "message", SessionName: "impl-carol", Body: "second", CreatedAt: time.Now().Add(-2 * time.Hour)},
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
		{Kind: "message", SessionName: "impl-bob", Body: "short"},              // 8 chars
		{Kind: "message", SessionName: "a-very-long-session-id", Body: "long"}, // 22 chars
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

func TestFormatDuration(t *testing.T) {
	tests := map[time.Duration]string{
		0:                                  "0s",
		45 * time.Second:                   "45s",
		90 * time.Second:                   "1m 30s",
		25*time.Hour + 2*time.Minute:       "1d 1h 2m",
		-(2*time.Hour + 30*time.Second):    "-2h 30s",
		1500 * time.Millisecond:            "2s",
	}
	for input, want := range tests {
		if got := FormatDuration(input); got != want {
			t.Fatalf("FormatDuration(%v) = %q, want %q", input, got, want)
		}
	}
}

func TestSparkBar(t *testing.T) {
	tests := map[string]struct {
		values []int
		want   string
	}{
		"empty": {nil, "(none)"},
		"zero":  {[]int{0, 0, 0}, "▁▁▁"},
		"scale": {[]int{0, 5, 10}, "▁▄█"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := SparkBar(tt.values); got != tt.want {
				t.Fatalf("SparkBar(%v) = %q, want %q", tt.values, got, tt.want)
			}
		})
	}
}

func TestRenderStats(t *testing.T) {
	report := sampleStatsReport()

	result := RenderStats(report)
	for _, want := range []string{
		"Stats", "Total: 1,234", "Mean cycle time: 2h", "Throughput: 7.5", "Tokens: 2,000",
		"Cycle Time", "Trend:", "Daily:", "Resource Burn", "Token trend:", "(none): total 12, mean 6", "bugfix: total 7.5, mean 3.8",
		"Status", "TODO: 3", "Tier", "    (none)", "Type", "(none): 2",
	} {
		if !strings.Contains(result, want) {
			t.Fatalf("RenderStats missing %q in:\n%s", want, result)
		}
	}
}

func TestJSONHelpers(t *testing.T) {
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.FixedZone("offset", -5*60*60))
	deleted := now.Add(time.Hour)
	ticket := TicketJSON(models.Ticket{ID: 7, Title: "T", Status: models.StatusDone, Tier: "standard", MainType: "test", AttentionLevel: 52, CreatedBy: "impl", CreatedAt: now, UpdatedAt: now, DeletedAt: &deleted})
	if ticket.CreatedAt != "2026-05-23T17:00:00Z" || ticket.DeletedAt == nil || *ticket.DeletedAt != "2026-05-23T18:00:00Z" {
		t.Fatalf("TicketJSON time conversion wrong: %+v", ticket)
	}
	if got := TicketsJSON([]models.Ticket{{ID: 1}, {ID: 2}}); len(got) != 2 || got[1].ID != 2 {
		t.Fatalf("TicketsJSON = %+v", got)
	}

	from, to := models.StatusTodo, models.StatusPlanning
	logEntry := LogEntryJSON(models.LogEntry{ID: 1, TicketID: 7, SessionName: "impl", Kind: "transition", Body: "note", FromState: &from, ToState: &to, CreatedAt: now})
	if logEntry.FromState == nil || *logEntry.FromState != "TODO" || logEntry.ToState == nil || *logEntry.ToState != "PLANNING" {
		t.Fatalf("LogEntryJSON status ptrs wrong: %+v", logEntry)
	}
	if got := LogEntriesJSON([]models.LogEntry{{ID: 3}}); len(got) != 1 || got[0].ID != 3 {
		t.Fatalf("LogEntriesJSON = %+v", got)
	}

	usage := UsageEntryJSON(models.UsageEntry{ID: 5, TicketID: 7, SessionName: "impl", Tokens: 10, Tools: 2, DurationMs: 3000, Agent: "implementer", Label: "test", CreatedAt: now, DeletedAt: &deleted})
	if usage.CreatedAt == "" || usage.DeletedAt == nil || usage.Tokens != 10 {
		t.Fatalf("UsageEntryJSON wrong: %+v", usage)
	}
	if got := UsageEntriesJSON([]models.UsageEntry{{ID: 6}}); len(got) != 1 || got[0].ID != 6 {
		t.Fatalf("UsageEntriesJSON = %+v", got)
	}

	var b bytes.Buffer
	if err := WriteJSON(&b, ticket); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}
	if !strings.Contains(b.String(), "\n  \"id\": 7") {
		t.Fatalf("WriteJSON not indented: %s", b.String())
	}
}

func TestStatsReportJSON(t *testing.T) {
	report := sampleStatsReport()
	got := StatsReportJSON(report)
	if got.Overview.CycleTime.Mean != "2h" || got.CycleTime.Summary.TotalMs != int64((4*time.Hour).Milliseconds()) {
		t.Fatalf("StatsReportJSON duration conversion wrong: %+v", got)
	}
	if len(got.CycleTime.Trend) != 2 || got.CycleTime.Trend[0].Duration != "1h" {
		t.Fatalf("StatsReportJSON trend wrong: %+v", got.CycleTime.Trend)
	}
	if len(got.Throughput.ByDay) != 2 || got.Throughput.ByDay[0].Time != "2026-05-23T00:00:00Z" {
		t.Fatalf("StatsReportJSON throughput wrong: %+v", got.Throughput.ByDay)
	}
	if len(got.ResourceBurn.Series) != 2 || got.ResourceBurn.Series[1].Value != 2.5 {
		t.Fatalf("StatsReportJSON series wrong: %+v", got.ResourceBurn.Series)
	}
}

func sampleStatsReport() statsPkg.Report {
	day := time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC)
	return statsPkg.Report{
		Overview: statsPkg.Overview{
			Total: 1234, Active: 10, Done: 3, Verified: 2, Archived: 1,
			CycleTime:   statsPkg.DurationSummary{Mean: 2 * time.Hour},
			LeadTime:    statsPkg.DurationSummary{Mean: 3 * time.Hour},
			Throughput:  statsPkg.NumericSummary{Total: 7.5},
			ResourceUse: statsPkg.NumericSummary{Total: 2000},
		},
		CycleTime: statsPkg.CycleTime{
			Summary: statsPkg.DurationSummary{Count: 2, Total: 4 * time.Hour, Mean: 2 * time.Hour, Median: time.Hour},
			Trend:   []statsPkg.TimeDurationPoint{{Time: day, Duration: time.Hour}, {Time: day.AddDate(0, 0, 1), Duration: 2 * time.Hour}},
		},
		Throughput: statsPkg.Throughput{
			Total: 5,
			ByDay: []statsPkg.TimeCountPoint{{Time: day, Count: 1}, {Time: day.AddDate(0, 0, 1), Count: 3}},
			ByWeek: []statsPkg.TimeCountPoint{{Time: day, Count: 0}, {Time: day.AddDate(0, 0, 7), Count: 2}},
		},
		ResourceBurn: statsPkg.ResourceBurn{
			Tokens:   statsPkg.NumericSummary{Count: 2, Total: 1000, Mean: 500, Median: 500},
			Tools:    statsPkg.NumericSummary{Count: 1, Total: 2, Mean: 2, Median: 2},
			Duration: statsPkg.DurationSummary{Count: 1, Total: time.Minute, Mean: time.Minute, Median: time.Minute},
			Series:   []statsPkg.TimeNumericPoint{{Time: day, Value: 0}, {Time: day.AddDate(0, 0, 1), Value: 2.5}},
			ByType:   []statsPkg.LabeledSummary{{Label: "", Summary: statsPkg.NumericSummary{Total: 12, Mean: 6}}, {Label: "bugfix", Summary: statsPkg.NumericSummary{Total: 7.5, Mean: 3.75}}},
		},
		Distribution: statsPkg.Distribution{
			Status: []statsPkg.CountBucket{{Label: "TODO", Count: 3}},
			Tier:   nil,
			Type:   []statsPkg.CountBucket{{Label: "", Count: 2}},
		},
	}
}
