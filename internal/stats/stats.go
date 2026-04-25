package stats

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/zalshy/tkt/internal/models"
)

// Options controls shared filtering for ticket-backed stats queries.
type Options struct {
	Since           *time.Time
	Until           *time.Time
	Status          *models.Status
	Tier            string
	Type            string
	CreatedBy       string
	IncludeArchived bool
	IncludeVerified bool
}

// Report is the top-level stats payload consumed by future query and render code.
type Report struct {
	Overview     Overview
	CycleTime    CycleTime
	Throughput   Throughput
	ResourceBurn ResourceBurn
	Distribution Distribution
}

// Overview aggregates high-level ticket counts and duration summaries.
type Overview struct {
	Total       int
	Active      int
	Done        int
	Verified    int
	Archived    int
	CycleTime   DurationSummary
	LeadTime    DurationSummary
	Throughput  NumericSummary
	ResourceUse NumericSummary
}

// CycleTime aggregates completion-time metrics and supporting trend points.
type CycleTime struct {
	Summary DurationSummary
	Trend   []TimeDurationPoint
}

// Throughput tracks completed-ticket volume over time.
type Throughput struct {
	Total  int
	ByDay  []TimeCountPoint
	ByWeek []TimeCountPoint
}

// ResourceBurn tracks usage totals and optional breakdowns.
type ResourceBurn struct {
	Tokens   NumericSummary
	Tools    NumericSummary
	Duration DurationSummary
	Series   []TimeNumericPoint
	ByType   []LabeledSummary
}

// Distribution groups ticket counts by categorical fields.
type Distribution struct {
	Status []CountBucket
	Tier   []CountBucket
	Type   []CountBucket
}

// CountBucket is reusable for labeled count distributions.
type CountBucket struct {
	Label string
	Count int
}

// LabeledSummary carries summary values keyed by label.
type LabeledSummary struct {
	Label   string
	Summary NumericSummary
}

// TimeCountPoint is a dated count bucket for trend charts and sparklines.
type TimeCountPoint struct {
	Time  time.Time
	Count int
}

// TimeNumericPoint is a dated numeric bucket for trend charts and sparklines.
type TimeNumericPoint struct {
	Time  time.Time
	Value float64
}

// TimeDurationPoint is a dated duration bucket for trend charts and sparklines.
type TimeDurationPoint struct {
	Time     time.Time
	Duration time.Duration
}

// NumericSummary captures count, total, mean, and median for numeric metrics.
type NumericSummary struct {
	Count  int
	Total  float64
	Mean   float64
	Median float64
}

// DurationSummary captures count, total, mean, and median for durations.
type DurationSummary struct {
	Count  int
	Total  time.Duration
	Mean   time.Duration
	Median time.Duration
}

func filterArgs(opts Options) ([]string, []any) {
	where := []string{"t.deleted_at IS NULL"}
	args := make([]any, 0, 6)

	if opts.Status != nil {
		where = append(where, "t.status = ?")
		args = append(args, string(*opts.Status))
	} else if !opts.IncludeVerified {
		where = append(where, "t.status != 'VERIFIED'")
	}

	if !opts.IncludeArchived && (opts.Status == nil || *opts.Status != models.StatusArchived) {
		where = append(where, "t.status != 'ARCHIVED'")
	}

	if activityWhere, activityArgs := ticketActivityWhere(opts); activityWhere != "" {
		where = append(where, activityWhere)
		args = append(args, activityArgs...)
	}

	if opts.Tier != "" {
		where = append(where, "t.tier = ?")
		args = append(args, opts.Tier)
	}

	if opts.Type != "" {
		where = append(where, "t.main_type = ?")
		args = append(args, opts.Type)
	}

	if opts.CreatedBy != "" {
		where = append(where, "t.created_by = ?")
		args = append(args, opts.CreatedBy)
	}

	return where, args
}

func ticketActivityWhere(opts Options) (string, []any) {
	if opts.Since == nil && opts.Until == nil {
		return "", nil
	}

	var clauses []string
	var args []any

	updatedWhere, updatedArgs := timeRangeWhere("t.updated_at", opts)
	clauses = append(clauses, updatedWhere)
	args = append(args, updatedArgs...)

	logWhere, logArgs := timeRangeWhere("al.created_at", opts)
	clauses = append(clauses, "EXISTS (SELECT 1 FROM ticket_log al WHERE al.ticket_id = t.id AND al.deleted_at IS NULL AND "+logWhere+")")
	args = append(args, logArgs...)

	usageWhere, usageArgs := timeRangeWhere("au.created_at", opts)
	clauses = append(clauses, "EXISTS (SELECT 1 FROM ticket_usage au WHERE au.ticket_id = t.id AND au.deleted_at IS NULL AND "+usageWhere+")")
	args = append(args, usageArgs...)

	return "(" + strings.Join(clauses, " OR ") + ")", args
}

func timeRangeWhere(column string, opts Options) (string, []any) {
	var clauses []string
	var args []any
	if opts.Since != nil {
		clauses = append(clauses, column+" >= ?")
		args = append(args, *opts.Since)
	}
	if opts.Until != nil {
		clauses = append(clauses, column+" <= ?")
		args = append(args, *opts.Until)
	}
	return "(" + strings.Join(clauses, " AND ") + ")", args
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var total float64
	for _, value := range values {
		total += value
	}

	return total / float64(len(values))
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}

	return (sorted[mid-1] + sorted[mid]) / 2
}

func queryOverview(db *sql.DB, opts Options) (Overview, error) {
	var overview Overview

	where, args := filterArgs(opts)
	query := `
SELECT
	COUNT(*) AS total,
	COALESCE(SUM(CASE WHEN t.status IN ('TODO', 'PLANNING', 'IN_PROGRESS') THEN 1 ELSE 0 END), 0) AS active,
	COALESCE(SUM(CASE WHEN t.status = 'DONE' THEN 1 ELSE 0 END), 0) AS done,
	COALESCE(SUM(CASE WHEN t.status = 'VERIFIED' THEN 1 ELSE 0 END), 0) AS verified,
	COALESCE(SUM(CASE WHEN t.status = 'ARCHIVED' THEN 1 ELSE 0 END), 0) AS archived
FROM tickets t
WHERE ` + strings.Join(where, " AND ")

	if err := db.QueryRow(query, args...).Scan(
		&overview.Total,
		&overview.Active,
		&overview.Done,
		&overview.Verified,
		&overview.Archived,
	); err != nil {
		return Overview{}, err
	}

	cycleTime, err := queryTransitionSummary(db, opts, models.StatusDone)
	if err != nil {
		return Overview{}, err
	}
	leadTime, err := queryTransitionSummary(db, opts, models.StatusVerified)
	if err != nil {
		return Overview{}, err
	}

	overview.CycleTime = cycleTime
	overview.LeadTime = leadTime

	return overview, nil
}

func queryCycleTime(db *sql.DB, opts Options) (CycleTime, error) {
	rows, err := queryTransitionDurations(db, opts, models.StatusDone)
	if err != nil {
		return CycleTime{}, err
	}

	cycleTime := CycleTime{
		Summary: buildDurationSummary(durationValues(rows)),
		Trend:   make([]TimeDurationPoint, 0, len(rows)),
	}

	for _, row := range rows {
		cycleTime.Trend = append(cycleTime.Trend, TimeDurationPoint{
			Time:     row.CompletedAt,
			Duration: row.Duration,
		})
	}

	sort.Slice(cycleTime.Trend, func(i, j int) bool {
		return cycleTime.Trend[i].Time.Before(cycleTime.Trend[j].Time)
	})

	return cycleTime, nil
}

type transitionDurationRow struct {
	CompletedAt time.Time
	Duration    time.Duration
}

func parseSQLiteTime(value any) (time.Time, error) {
	switch v := value.(type) {
	case time.Time:
		return v, nil
	case string:
		return parseSQLiteTimeString(v)
	case []byte:
		return parseSQLiteTimeString(string(v))
	default:
		return time.Time{}, fmt.Errorf("unsupported time value %T", value)
	}
}

func parseSQLiteTimeString(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time value %q", value)
}

func queryTransitionSummary(db *sql.DB, opts Options, toState models.Status) (DurationSummary, error) {
	rows, err := queryTransitionDurations(db, opts, toState)
	if err != nil {
		return DurationSummary{}, err
	}

	return buildDurationSummary(durationValues(rows)), nil
}

func queryTransitionDurations(db *sql.DB, opts Options, toState models.Status) ([]transitionDurationRow, error) {
	where, args := filterArgs(opts)
	transitionWhere := []string{"tl.deleted_at IS NULL", "tl.kind = 'transition'", "tl.to_state = ?"}
	transitionArgs := []any{string(toState)}
	if rangeWhere, rangeArgs := timeRangeWhere("tl.created_at", opts); opts.Since != nil || opts.Until != nil {
		transitionWhere = append(transitionWhere, rangeWhere)
		transitionArgs = append(transitionArgs, rangeArgs...)
	}

	query := `
SELECT
	ft.created_at,
	first_transition.completed_at
FROM (
	SELECT t.id, t.created_at
	FROM tickets t
	WHERE ` + strings.Join(where, " AND ") + `
) AS ft
JOIN (
	SELECT tl.ticket_id, MIN(tl.created_at) AS completed_at
	FROM ticket_log tl
	WHERE ` + strings.Join(transitionWhere, " AND ") + `
	GROUP BY tl.ticket_id
) AS first_transition ON first_transition.ticket_id = ft.id`

	args = append(args, transitionArgs...)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []transitionDurationRow
	for rows.Next() {
		var createdAtRaw any
		var completedAtRaw any
		if err := rows.Scan(&createdAtRaw, &completedAtRaw); err != nil {
			return nil, err
		}

		createdAt, err := parseSQLiteTime(createdAtRaw)
		if err != nil {
			return nil, err
		}
		completedAt, err := parseSQLiteTime(completedAtRaw)
		if err != nil {
			return nil, err
		}

		result = append(result, transitionDurationRow{
			CompletedAt: completedAt,
			Duration:    completedAt.Sub(createdAt),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func durationValues(rows []transitionDurationRow) []time.Duration {
	values := make([]time.Duration, 0, len(rows))
	for _, row := range rows {
		values = append(values, row.Duration)
	}

	return values
}

func buildDurationSummary(values []time.Duration) DurationSummary {
	if len(values) == 0 {
		return DurationSummary{}
	}

	seconds := make([]float64, 0, len(values))
	var total time.Duration
	for _, value := range values {
		total += value
		seconds = append(seconds, value.Seconds())
	}

	return DurationSummary{
		Count:  len(values),
		Total:  total,
		Mean:   time.Duration(mean(seconds) * float64(time.Second)),
		Median: time.Duration(median(seconds) * float64(time.Second)),
	}
}

func filteredTicketsQuery(opts Options) (string, []any) {
	where, args := filterArgs(opts)
	query := `
SELECT
	t.id,
	t.status,
	t.tier,
	t.main_type
FROM tickets t
WHERE ` + strings.Join(where, " AND ")

	return query, args
}

func buildNumericSummary(values []float64) NumericSummary {
	if len(values) == 0 {
		return NumericSummary{}
	}

	var total float64
	for _, value := range values {
		total += value
	}

	return NumericSummary{
		Count:  len(values),
		Total:  total,
		Mean:   mean(values),
		Median: median(values),
	}
}

func queryThroughput(db *sql.DB, opts Options) (Throughput, error) {
	filteredQuery, args := filteredTicketsQuery(opts)
	transitionWhere := []string{"tl.deleted_at IS NULL", "tl.kind = 'transition'", "tl.to_state = ?"}
	transitionArgs := []any{string(models.StatusDone)}
	if rangeWhere, rangeArgs := timeRangeWhere("tl.created_at", opts); opts.Since != nil || opts.Until != nil {
		transitionWhere = append(transitionWhere, rangeWhere)
		transitionArgs = append(transitionArgs, rangeArgs...)
	}

	query := `
SELECT first_done.done_at
FROM (` + filteredQuery + `) AS ft
JOIN (
	SELECT tl.ticket_id, MIN(tl.created_at) AS done_at
	FROM ticket_log tl
	WHERE ` + strings.Join(transitionWhere, " AND ") + `
	GROUP BY tl.ticket_id
) AS first_done ON first_done.ticket_id = ft.id
ORDER BY first_done.done_at ASC`

	args = append(args, transitionArgs...)
	rows, err := db.Query(query, args...)
	if err != nil {
		return Throughput{}, err
	}
	defer rows.Close()

	throughput := Throughput{}
	dayCounts := make(map[time.Time]int)
	weekCounts := make(map[time.Time]int)

	for rows.Next() {
		var doneAtRaw any
		if err := rows.Scan(&doneAtRaw); err != nil {
			return Throughput{}, err
		}
		doneAt, err := parseSQLiteTime(doneAtRaw)
		if err != nil {
			return Throughput{}, err
		}

		throughput.Total++
		dayStart := truncateDay(doneAt)
		weekStart := weekStart(dayStart)
		dayCounts[dayStart]++
		weekCounts[weekStart]++
	}

	if err := rows.Err(); err != nil {
		return Throughput{}, err
	}

	throughput.ByDay = buildTimeCountPoints(dayCounts)
	throughput.ByWeek = buildTimeCountPoints(weekCounts)
	return throughput, nil
}

func queryResourceBurn(db *sql.DB, opts Options) (ResourceBurn, error) {
	filteredQuery, args := filteredTicketsQuery(opts)
	usageWhere := []string{"u.deleted_at IS NULL"}
	var usageArgs []any
	if rangeWhere, rangeArgs := timeRangeWhere("u.created_at", opts); opts.Since != nil || opts.Until != nil {
		usageWhere = append(usageWhere, rangeWhere)
		usageArgs = append(usageArgs, rangeArgs...)
	}

	query := `
SELECT
	ft.main_type,
	u.tokens,
	u.tools,
	u.duration_ms,
	u.created_at
FROM (` + filteredQuery + `) AS ft
JOIN ticket_usage u ON u.ticket_id = ft.id
WHERE ` + strings.Join(usageWhere, " AND ") + `
ORDER BY u.created_at ASC, u.id ASC`

	args = append(args, usageArgs...)

	rows, err := db.Query(query, args...)
	if err != nil {
		return ResourceBurn{}, err
	}
	defer rows.Close()

	resource := ResourceBurn{}
	tokenValues := make([]float64, 0)
	toolValues := make([]float64, 0)
	durationValues := make([]time.Duration, 0)
	byType := make(map[string][]float64)

	for rows.Next() {
		var label string
		var tokens int
		var tools int
		var durationMS int
		var createdAtRaw any
		if err := rows.Scan(&label, &tokens, &tools, &durationMS, &createdAtRaw); err != nil {
			return ResourceBurn{}, err
		}
		createdAt, err := parseSQLiteTime(createdAtRaw)
		if err != nil {
			return ResourceBurn{}, err
		}

		tokenValue := float64(tokens)
		toolValue := float64(tools)
		durationValue := time.Duration(durationMS) * time.Millisecond

		tokenValues = append(tokenValues, tokenValue)
		toolValues = append(toolValues, toolValue)
		durationValues = append(durationValues, durationValue)
		byType[label] = append(byType[label], tokenValue)
		resource.Series = append(resource.Series, TimeNumericPoint{
			Time:  createdAt,
			Value: tokenValue,
		})
	}

	if err := rows.Err(); err != nil {
		return ResourceBurn{}, err
	}

	resource.Tokens = buildNumericSummary(tokenValues)
	resource.Tools = buildNumericSummary(toolValues)
	resource.Duration = buildDurationSummary(durationValues)
	resource.ByType = make([]LabeledSummary, 0, len(byType))
	for label, values := range byType {
		resource.ByType = append(resource.ByType, LabeledSummary{
			Label:   label,
			Summary: buildNumericSummary(values),
		})
	}

	sort.Slice(resource.ByType, func(i, j int) bool {
		return resource.ByType[i].Label < resource.ByType[j].Label
	})

	return resource, nil
}

func queryDistribution(db *sql.DB, opts Options) (Distribution, error) {
	filteredQuery, args := filteredTicketsQuery(opts)
	query := `
SELECT status, tier, main_type
FROM (` + filteredQuery + `) AS ft`

	rows, err := db.Query(query, args...)
	if err != nil {
		return Distribution{}, err
	}
	defer rows.Close()

	statusCounts := make(map[string]int)
	tierCounts := make(map[string]int)
	typeCounts := make(map[string]int)

	for rows.Next() {
		var status string
		var tier string
		var mainType string
		if err := rows.Scan(&status, &tier, &mainType); err != nil {
			return Distribution{}, err
		}

		statusCounts[status]++
		tierCounts[tier]++
		typeCounts[mainType]++
	}

	if err := rows.Err(); err != nil {
		return Distribution{}, err
	}

	return Distribution{
		Status: buildCountBuckets(statusCounts),
		Tier:   buildCountBuckets(tierCounts),
		Type:   buildCountBuckets(typeCounts),
	}, nil
}

func Compute(db *sql.DB, opts Options) (Report, error) {
	overview, err := queryOverview(db, opts)
	if err != nil {
		return Report{}, err
	}

	cycleTime, err := queryCycleTime(db, opts)
	if err != nil {
		return Report{}, err
	}

	throughput, err := queryThroughput(db, opts)
	if err != nil {
		return Report{}, err
	}

	resourceBurn, err := queryResourceBurn(db, opts)
	if err != nil {
		return Report{}, err
	}

	distribution, err := queryDistribution(db, opts)
	if err != nil {
		return Report{}, err
	}

	overview.Throughput = throughputOverviewSummary(throughput.Total)
	overview.ResourceUse = resourceBurn.Tokens

	return Report{
		Overview:     overview,
		CycleTime:    cycleTime,
		Throughput:   throughput,
		ResourceBurn: resourceBurn,
		Distribution: distribution,
	}, nil
}

func truncateDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func weekStart(day time.Time) time.Time {
	offset := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -offset)
}

func buildTimeCountPoints(counts map[time.Time]int) []TimeCountPoint {
	points := make([]TimeCountPoint, 0, len(counts))
	for ts, count := range counts {
		points = append(points, TimeCountPoint{
			Time:  ts,
			Count: count,
		})
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].Time.Before(points[j].Time)
	})
	return points
}

func buildCountBuckets(counts map[string]int) []CountBucket {
	buckets := make([]CountBucket, 0, len(counts))
	for label, count := range counts {
		buckets = append(buckets, CountBucket{
			Label: label,
			Count: count,
		})
	}

	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Label < buckets[j].Label
	})
	return buckets
}

func throughputOverviewSummary(total int) NumericSummary {
	if total == 0 {
		return NumericSummary{}
	}

	value := float64(total)
	return NumericSummary{
		Count:  total,
		Total:  value,
		Mean:   value,
		Median: value,
	}
}
