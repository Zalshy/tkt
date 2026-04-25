package stats

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/models"
)

func setupStatsDB(t *testing.T) *sql.DB {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".tkt"), 0o700); err != nil {
		t.Fatalf("mkdir .tkt: %v", err)
	}
	database, err := db.Open(root)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

type seededTicket struct {
	ID        int64
	Status    models.Status
	Tier      string
	MainType  string
	CreatedBy string
	CreatedAt time.Time
}

func seedStatsTicket(t *testing.T, database *sql.DB, title string, status models.Status, tier, mainType, createdBy string, createdAt time.Time) seededTicket {
	t.Helper()
	res, err := database.Exec(
		`INSERT INTO tickets (title, description, status, tier, main_type, created_by, created_at, updated_at)
		 VALUES (?, '', ?, ?, ?, ?, ?, ?)`,
		title,
		string(status),
		tier,
		mainType,
		createdBy,
		createdAt,
		createdAt,
	)
	if err != nil {
		t.Fatalf("insert ticket %q: %v", title, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	return seededTicket{
		ID:        id,
		Status:    status,
		Tier:      tier,
		MainType:  mainType,
		CreatedBy: createdBy,
		CreatedAt: createdAt,
	}
}

func seedTransition(t *testing.T, database *sql.DB, ticketID int64, from, to models.Status, at time.Time) {
	t.Helper()
	_, err := database.Exec(
		`INSERT INTO ticket_log (ticket_id, session_name, kind, body, from_state, to_state, created_at)
		 VALUES (?, 'stats-test', 'transition', ?, ?, ?, ?)`,
		ticketID,
		"transition",
		string(from),
		string(to),
		at,
	)
	if err != nil {
		t.Fatalf("insert transition for ticket %d: %v", ticketID, err)
	}
}

func seedUsage(t *testing.T, database *sql.DB, ticketID int64, tokens, tools, durationMS int, at time.Time) {
	t.Helper()
	_, err := database.Exec(
		`INSERT INTO ticket_usage (ticket_id, session_name, tokens, tools, duration_ms, agent, label, created_at)
		 VALUES (?, 'stats-test', ?, ?, ?, 'implementer', 'unit', ?)`,
		ticketID,
		tokens,
		tools,
		durationMS,
		at,
	)
	if err != nil {
		t.Fatalf("insert usage for ticket %d: %v", ticketID, err)
	}
}

func seedStatsFixture(t *testing.T, database *sql.DB) map[string]seededTicket {
	t.Helper()
	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)

	tickets := map[string]seededTicket{}
	tickets["todo"] = seedStatsTicket(t, database, "todo", models.StatusTodo, "standard", "feature", "alice", base)
	tickets["progress"] = seedStatsTicket(t, database, "progress", models.StatusInProgress, "low", "bugfix", "bob", base.AddDate(0, 0, 1))
	tickets["doneA"] = seedStatsTicket(t, database, "done A", models.StatusDone, "critical", "feature", "alice", base.AddDate(0, 0, 2))
	tickets["doneB"] = seedStatsTicket(t, database, "done B", models.StatusDone, "standard", "docs", "bob", base.AddDate(0, 0, 3))
	tickets["verified"] = seedStatsTicket(t, database, "verified", models.StatusVerified, "standard", "feature", "alice", base.AddDate(0, 0, 4))
	tickets["archived"] = seedStatsTicket(t, database, "archived", models.StatusArchived, "low", "refactor", "carol", base.AddDate(0, 0, 5))

	seedTransition(t, database, tickets["doneA"].ID, models.StatusInProgress, models.StatusDone, tickets["doneA"].CreatedAt.Add(24*time.Hour))
	seedTransition(t, database, tickets["doneB"].ID, models.StatusInProgress, models.StatusDone, tickets["doneB"].CreatedAt.Add(48*time.Hour))
	seedTransition(t, database, tickets["verified"].ID, models.StatusInProgress, models.StatusDone, tickets["verified"].CreatedAt.Add(72*time.Hour))
	seedTransition(t, database, tickets["verified"].ID, models.StatusDone, models.StatusVerified, tickets["verified"].CreatedAt.Add(96*time.Hour))
	seedTransition(t, database, tickets["archived"].ID, models.StatusInProgress, models.StatusDone, tickets["archived"].CreatedAt.Add(120*time.Hour))

	seedUsage(t, database, tickets["todo"].ID, 100, 1, 1000, base.Add(1*time.Hour))
	seedUsage(t, database, tickets["doneA"].ID, 300, 3, 3000, tickets["doneA"].CreatedAt.Add(1*time.Hour))
	seedUsage(t, database, tickets["doneB"].ID, 500, 5, 5000, tickets["doneB"].CreatedAt.Add(1*time.Hour))
	seedUsage(t, database, tickets["verified"].ID, 700, 7, 7000, tickets["verified"].CreatedAt.Add(1*time.Hour))
	seedUsage(t, database, tickets["archived"].ID, 900, 9, 9000, tickets["archived"].CreatedAt.Add(1*time.Hour))

	return tickets
}

func TestCompute_EmptyDB(t *testing.T) {
	database := setupStatsDB(t)

	report, err := Compute(database, Options{})
	if err != nil {
		t.Fatalf("Compute empty DB: %v", err)
	}

	if report.Overview.Total != 0 || report.Overview.Active != 0 || report.Overview.Done != 0 || report.Overview.Verified != 0 || report.Overview.Archived != 0 {
		t.Fatalf("overview counts = %+v, want all zero", report.Overview)
	}
	if report.CycleTime.Summary != (DurationSummary{}) {
		t.Fatalf("cycle summary = %+v, want zero", report.CycleTime.Summary)
	}
	if report.ResourceBurn.Tokens != (NumericSummary{}) {
		t.Fatalf("tokens summary = %+v, want zero", report.ResourceBurn.Tokens)
	}
	if len(report.CycleTime.Trend) != 0 || len(report.Throughput.ByDay) != 0 || len(report.ResourceBurn.Series) != 0 {
		t.Fatalf("trend series should be empty: cycle=%d day=%d usage=%d", len(report.CycleTime.Trend), len(report.Throughput.ByDay), len(report.ResourceBurn.Series))
	}
	if len(report.Distribution.Status) != 0 || len(report.Distribution.Tier) != 0 || len(report.Distribution.Type) != 0 {
		t.Fatalf("distribution should be empty: %+v", report.Distribution)
	}
}

func TestCompute_PopulatedReport(t *testing.T) {
	database := setupStatsDB(t)
	seedStatsFixture(t, database)

	report, err := Compute(database, Options{})
	if err != nil {
		t.Fatalf("Compute populated DB: %v", err)
	}

	if report.Overview.Total != 4 || report.Overview.Active != 2 || report.Overview.Done != 2 || report.Overview.Verified != 0 || report.Overview.Archived != 0 {
		t.Fatalf("overview counts = %+v", report.Overview)
	}

	wantCycleTotal := 72 * time.Hour
	if report.CycleTime.Summary.Count != 2 || report.CycleTime.Summary.Total != wantCycleTotal || report.CycleTime.Summary.Mean != 36*time.Hour || report.CycleTime.Summary.Median != 36*time.Hour {
		t.Fatalf("cycle summary = %+v", report.CycleTime.Summary)
	}
	if len(report.CycleTime.Trend) != 2 || report.CycleTime.Trend[0].Duration != 24*time.Hour || report.CycleTime.Trend[1].Duration != 48*time.Hour {
		t.Fatalf("cycle trend = %+v", report.CycleTime.Trend)
	}

	if report.Throughput.Total != 2 {
		t.Fatalf("throughput total = %d, want 2", report.Throughput.Total)
	}
	if len(report.Throughput.ByDay) != 2 || report.Throughput.ByDay[0].Count != 1 || report.Throughput.ByDay[1].Count != 1 {
		t.Fatalf("daily throughput = %+v", report.Throughput.ByDay)
	}
	if len(report.Throughput.ByWeek) != 2 || report.Throughput.ByWeek[0].Count != 1 || report.Throughput.ByWeek[1].Count != 1 {
		t.Fatalf("weekly throughput = %+v", report.Throughput.ByWeek)
	}

	assertNumericSummary(t, report.ResourceBurn.Tokens, NumericSummary{Count: 3, Total: 900, Mean: 300, Median: 300}, "tokens")
	assertNumericSummary(t, report.ResourceBurn.Tools, NumericSummary{Count: 3, Total: 9, Mean: 3, Median: 3}, "tools")
	if report.ResourceBurn.Duration.Count != 3 || report.ResourceBurn.Duration.Total != 9*time.Second || report.ResourceBurn.Duration.Mean != 3*time.Second || report.ResourceBurn.Duration.Median != 3*time.Second {
		t.Fatalf("duration summary = %+v", report.ResourceBurn.Duration)
	}
	if len(report.ResourceBurn.Series) != 3 || report.ResourceBurn.Series[0].Value != 100 || report.ResourceBurn.Series[2].Value != 500 {
		t.Fatalf("resource series = %+v", report.ResourceBurn.Series)
	}

	wantByType := map[string]NumericSummary{
		"docs":    {Count: 1, Total: 500, Mean: 500, Median: 500},
		"feature": {Count: 2, Total: 400, Mean: 200, Median: 200},
	}
	if len(report.ResourceBurn.ByType) != len(wantByType) {
		t.Fatalf("by type = %+v", report.ResourceBurn.ByType)
	}
	for _, item := range report.ResourceBurn.ByType {
		assertNumericSummary(t, item.Summary, wantByType[item.Label], "by type "+item.Label)
	}

	assertBuckets(t, report.Distribution.Status, map[string]int{"DONE": 2, "IN_PROGRESS": 1, "TODO": 1}, "status")
	assertBuckets(t, report.Distribution.Tier, map[string]int{"critical": 1, "low": 1, "standard": 2}, "tier")
	assertBuckets(t, report.Distribution.Type, map[string]int{"bugfix": 1, "docs": 1, "feature": 2}, "type")

	assertNumericSummary(t, report.Overview.Throughput, NumericSummary{Count: 2, Total: 2, Mean: 2, Median: 2}, "overview throughput")
	assertNumericSummary(t, report.Overview.ResourceUse, report.ResourceBurn.Tokens, "overview resource use")
}

func TestCompute_Filters(t *testing.T) {
	database := setupStatsDB(t)
	tickets := seedStatsFixture(t, database)

	t.Run("status", func(t *testing.T) {
		status := models.StatusDone
		report, err := Compute(database, Options{Status: &status})
		if err != nil {
			t.Fatal(err)
		}
		if report.Overview.Total != 2 || report.Overview.Done != 2 || report.Throughput.Total != 2 {
			t.Fatalf("DONE report overview=%+v throughput=%+v", report.Overview, report.Throughput)
		}
	})

	t.Run("tier", func(t *testing.T) {
		report, err := Compute(database, Options{Tier: "critical"})
		if err != nil {
			t.Fatal(err)
		}
		if report.Overview.Total != 1 || report.Overview.Done != 1 || report.ResourceBurn.Tokens.Total != 300 {
			t.Fatalf("critical report overview=%+v tokens=%+v", report.Overview, report.ResourceBurn.Tokens)
		}
	})

	t.Run("type", func(t *testing.T) {
		report, err := Compute(database, Options{Type: "docs"})
		if err != nil {
			t.Fatal(err)
		}
		if report.Overview.Total != 1 || report.ResourceBurn.Tokens.Total != 500 {
			t.Fatalf("docs report overview=%+v tokens=%+v", report.Overview, report.ResourceBurn.Tokens)
		}
	})

	t.Run("created by", func(t *testing.T) {
		report, err := Compute(database, Options{CreatedBy: "bob"})
		if err != nil {
			t.Fatal(err)
		}
		if report.Overview.Total != 2 || report.Overview.Active != 1 || report.Overview.Done != 1 {
			t.Fatalf("bob report overview=%+v", report.Overview)
		}
	})

	t.Run("since until", func(t *testing.T) {
		since := tickets["doneA"].CreatedAt
		until := tickets["doneB"].CreatedAt
		report, err := Compute(database, Options{Since: &since, Until: &until})
		if err != nil {
			t.Fatal(err)
		}
		if report.Overview.Total != 2 || report.Overview.Done != 2 {
			t.Fatalf("date filtered report overview=%+v", report.Overview)
		}
	})

	t.Run("include verified", func(t *testing.T) {
		report, err := Compute(database, Options{IncludeVerified: true})
		if err != nil {
			t.Fatal(err)
		}
		if report.Overview.Total != 5 || report.Overview.Verified != 1 || report.Throughput.Total != 3 || report.Overview.LeadTime.Count != 1 {
			t.Fatalf("verified report overview=%+v throughput=%+v", report.Overview, report.Throughput)
		}
	})

	t.Run("include archived", func(t *testing.T) {
		report, err := Compute(database, Options{IncludeArchived: true})
		if err != nil {
			t.Fatal(err)
		}
		if report.Overview.Total != 5 || report.Overview.Archived != 1 || report.ResourceBurn.Tokens.Total != 1800 {
			t.Fatalf("archived report overview=%+v tokens=%+v", report.Overview, report.ResourceBurn.Tokens)
		}
	})

	t.Run("explicit archived status", func(t *testing.T) {
		status := models.StatusArchived
		report, err := Compute(database, Options{Status: &status})
		if err != nil {
			t.Fatal(err)
		}
		if report.Overview.Total != 1 || report.Overview.Archived != 1 || report.Throughput.Total != 1 {
			t.Fatalf("explicit archived report overview=%+v throughput=%+v", report.Overview, report.Throughput)
		}
	})
}

func assertNumericSummary(t *testing.T, got, want NumericSummary, label string) {
	t.Helper()
	if got.Count != want.Count || got.Total != want.Total || got.Mean != want.Mean || got.Median != want.Median {
		t.Fatalf("%s summary = %+v, want %+v", label, got, want)
	}
}

func assertBuckets(t *testing.T, got []CountBucket, want map[string]int, label string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s buckets = %+v, want %+v", label, got, want)
	}
	for _, bucket := range got {
		if want[bucket.Label] != bucket.Count {
			t.Fatalf("%s bucket %q = %d, want %d (all buckets %+v)", label, bucket.Label, bucket.Count, want[bucket.Label], got)
		}
		delete(want, bucket.Label)
	}
	if len(want) != 0 {
		t.Fatalf("%s missing buckets: %+v", label, want)
	}
}
