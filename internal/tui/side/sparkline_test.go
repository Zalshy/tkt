package side

import (
	"database/sql"
	"math"
	"testing"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

func TestSparklineConstantsUseFiveSecondBucketsOverTwoHours(t *testing.T) {
	if sparklineBuckets != 1440 {
		t.Fatalf("sparklineBuckets = %d, want 1440", sparklineBuckets)
	}
	if sparklineBucketSec != 5 {
		t.Fatalf("sparklineBucketSec = %d, want 5", sparklineBucketSec)
	}
	if got := sparklineBuckets * sparklineBucketSec; got != 2*60*60 {
		t.Fatalf("sparkline window = %d seconds, want 7200", got)
	}
}

func TestSmoothGaussianZeroPadsBoundaries(t *testing.T) {
	src := []int{10, 0, 0, 0, 0}

	got := smoothGaussian(src, 1)

	// With zero-padding, out-of-range samples add weight but no value.
	// Edge-clamping would duplicate src[0] into negative indexes and produce ~7.0.
	if math.Abs(got[0]-3.99050205549689) > 0.000001 {
		t.Fatalf("smoothed edge = %.12f, want zero-padded value near 3.990502", got[0])
	}
}

func TestLoadSparklineClampsNewestOverflowToLastBucket(t *testing.T) {
	db := openSparklineTestDB(t)
	defer db.Close()

	// Future-relative timestamp intentionally computes beyond sparklineBuckets.
	// loadSparkline should clamp newest-edge overflow into the last bucket.
	if _, err := db.Exec(`
		INSERT INTO ticket_log (kind, created_at)
		VALUES ('transition', datetime('now', '+5 seconds'))
	`); err != nil {
		t.Fatalf("insert transition: %v", err)
	}

	data, err := loadSparkline(db)
	if err != nil {
		t.Fatalf("loadSparkline: %v", err)
	}
	if len(data.buckets) != sparklineBuckets {
		t.Fatalf("len(data.buckets) = %d, want %d", len(data.buckets), sparklineBuckets)
	}
	if got := data.buckets[sparklineBuckets-1]; got != 1 {
		t.Fatalf("last bucket count = %d, want 1", got)
	}
}

func openSparklineTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE ticket_log (
			kind TEXT NOT NULL,
			created_at TEXT NOT NULL,
			deleted_at TEXT
		)
	`); err != nil {
		db.Close()
		t.Fatalf("create ticket_log: %v", err)
	}
	return db
}
