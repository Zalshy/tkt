package output

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/zalshy/tkt/internal/models"
	statsPkg "github.com/zalshy/tkt/internal/stats"
)

type JSONTicket struct {
	ID             int64   `json:"id"`
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	Status         string  `json:"status"`
	Tier           string  `json:"tier"`
	MainType       string  `json:"main_type"`
	AttentionLevel int     `json:"attention_level"`
	CreatedBy      string  `json:"created_by"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
	DeletedAt      *string `json:"deleted_at"`
}

type JSONLogEntry struct {
	ID          int64   `json:"id"`
	TicketID    int64   `json:"ticket_id"`
	SessionName string  `json:"session_name"`
	Kind        string  `json:"kind"`
	Body        string  `json:"body"`
	FromState   *string `json:"from_state"`
	ToState     *string `json:"to_state"`
	CreatedAt   string  `json:"created_at"`
	DeletedAt   *string `json:"deleted_at"`
}

type JSONUsageEntry struct {
	ID          int64   `json:"id"`
	TicketID    int64   `json:"ticket_id"`
	SessionName string  `json:"session_name"`
	Tokens      int     `json:"tokens"`
	Tools       int     `json:"tools"`
	DurationMs  int     `json:"duration_ms"`
	Agent       string  `json:"agent"`
	Label       string  `json:"label"`
	CreatedAt   string  `json:"created_at"`
	DeletedAt   *string `json:"deleted_at"`
}

func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	return nil
}

func TicketJSON(t models.Ticket) JSONTicket {
	return JSONTicket{
		ID:             t.ID,
		Title:          t.Title,
		Description:    t.Description,
		Status:         string(t.Status),
		Tier:           t.Tier,
		MainType:       t.MainType,
		AttentionLevel: t.AttentionLevel,
		CreatedBy:      t.CreatedBy,
		CreatedAt:      formatJSONTime(t.CreatedAt),
		UpdatedAt:      formatJSONTime(t.UpdatedAt),
		DeletedAt:      formatJSONTimePtr(t.DeletedAt),
	}
}

func TicketsJSON(tickets []models.Ticket) []JSONTicket {
	out := make([]JSONTicket, len(tickets))
	for i, t := range tickets {
		out[i] = TicketJSON(t)
	}
	return out
}

func LogEntryJSON(e models.LogEntry) JSONLogEntry {
	return JSONLogEntry{
		ID:          e.ID,
		TicketID:    e.TicketID,
		SessionName: e.SessionName,
		Kind:        e.Kind,
		Body:        e.Body,
		FromState:   statusPtrString(e.FromState),
		ToState:     statusPtrString(e.ToState),
		CreatedAt:   formatJSONTime(e.CreatedAt),
		DeletedAt:   formatJSONTimePtr(e.DeletedAt),
	}
}

func LogEntriesJSON(entries []models.LogEntry) []JSONLogEntry {
	out := make([]JSONLogEntry, len(entries))
	for i, e := range entries {
		out[i] = LogEntryJSON(e)
	}
	return out
}

func UsageEntryJSON(u models.UsageEntry) JSONUsageEntry {
	return JSONUsageEntry{
		ID:          u.ID,
		TicketID:    u.TicketID,
		SessionName: u.SessionName,
		Tokens:      u.Tokens,
		Tools:       u.Tools,
		DurationMs:  u.DurationMs,
		Agent:       u.Agent,
		Label:       u.Label,
		CreatedAt:   formatJSONTime(u.CreatedAt),
		DeletedAt:   formatJSONTimePtr(u.DeletedAt),
	}
}

func UsageEntriesJSON(entries []models.UsageEntry) []JSONUsageEntry {
	out := make([]JSONUsageEntry, len(entries))
	for i, u := range entries {
		out[i] = UsageEntryJSON(u)
	}
	return out
}

type JSONDurationSummary struct {
	Count    int    `json:"count"`
	TotalMs  int64  `json:"total_ms"`
	Total    string `json:"total"`
	MeanMs   int64  `json:"mean_ms"`
	Mean     string `json:"mean"`
	MedianMs int64  `json:"median_ms"`
	Median   string `json:"median"`
}

type JSONTimeDurationPoint struct {
	Time       string `json:"time"`
	DurationMs int64  `json:"duration_ms"`
	Duration   string `json:"duration"`
}

type JSONStatsReport struct {
	Overview     JSONOverview          `json:"overview"`
	CycleTime    JSONCycleTime         `json:"cycle_time"`
	Throughput   JSONThroughput        `json:"throughput"`
	ResourceBurn JSONResourceBurn      `json:"resource_burn"`
	Distribution statsPkg.Distribution `json:"distribution"`
}

type JSONOverview struct {
	Total       int                     `json:"total"`
	Active      int                     `json:"active"`
	Done        int                     `json:"done"`
	Verified    int                     `json:"verified"`
	Archived    int                     `json:"archived"`
	CycleTime   JSONDurationSummary     `json:"cycle_time"`
	LeadTime    JSONDurationSummary     `json:"lead_time"`
	Throughput  statsPkg.NumericSummary `json:"throughput"`
	ResourceUse statsPkg.NumericSummary `json:"resource_use"`
}

type JSONCycleTime struct {
	Summary JSONDurationSummary     `json:"summary"`
	Trend   []JSONTimeDurationPoint `json:"trend"`
}

type JSONThroughput struct {
	Total  int                  `json:"total"`
	ByDay  []JSONTimeCountPoint `json:"by_day"`
	ByWeek []JSONTimeCountPoint `json:"by_week"`
}

type JSONResourceBurn struct {
	Tokens   statsPkg.NumericSummary   `json:"tokens"`
	Tools    statsPkg.NumericSummary   `json:"tools"`
	Duration JSONDurationSummary       `json:"duration"`
	Series   []JSONTimeNumericPoint    `json:"series"`
	ByType   []statsPkg.LabeledSummary `json:"by_type"`
}

type JSONTimeCountPoint struct {
	Time  string `json:"time"`
	Count int    `json:"count"`
}

type JSONTimeNumericPoint struct {
	Time  string  `json:"time"`
	Value float64 `json:"value"`
}

func StatsReportJSON(report statsPkg.Report) JSONStatsReport {
	return JSONStatsReport{
		Overview:     OverviewJSON(report.Overview),
		CycleTime:    JSONCycleTime{Summary: DurationSummaryJSON(report.CycleTime.Summary), Trend: TimeDurationPointsJSON(report.CycleTime.Trend)},
		Throughput:   JSONThroughput{Total: report.Throughput.Total, ByDay: TimeCountPointsJSON(report.Throughput.ByDay), ByWeek: TimeCountPointsJSON(report.Throughput.ByWeek)},
		ResourceBurn: JSONResourceBurn{Tokens: report.ResourceBurn.Tokens, Tools: report.ResourceBurn.Tools, Duration: DurationSummaryJSON(report.ResourceBurn.Duration), Series: TimeNumericPointsJSON(report.ResourceBurn.Series), ByType: report.ResourceBurn.ByType},
		Distribution: report.Distribution,
	}
}

func OverviewJSON(o statsPkg.Overview) JSONOverview {
	return JSONOverview{
		Total:       o.Total,
		Active:      o.Active,
		Done:        o.Done,
		Verified:    o.Verified,
		Archived:    o.Archived,
		CycleTime:   DurationSummaryJSON(o.CycleTime),
		LeadTime:    DurationSummaryJSON(o.LeadTime),
		Throughput:  o.Throughput,
		ResourceUse: o.ResourceUse,
	}
}

func DurationSummaryJSON(s statsPkg.DurationSummary) JSONDurationSummary {
	return JSONDurationSummary{Count: s.Count, TotalMs: s.Total.Milliseconds(), Total: FormatDuration(s.Total), MeanMs: s.Mean.Milliseconds(), Mean: FormatDuration(s.Mean), MedianMs: s.Median.Milliseconds(), Median: FormatDuration(s.Median)}
}

func TimeDurationPointsJSON(points []statsPkg.TimeDurationPoint) []JSONTimeDurationPoint {
	out := make([]JSONTimeDurationPoint, len(points))
	for i, p := range points {
		out[i] = JSONTimeDurationPoint{Time: formatJSONTime(p.Time), DurationMs: p.Duration.Milliseconds(), Duration: FormatDuration(p.Duration)}
	}
	return out
}

func TimeCountPointsJSON(points []statsPkg.TimeCountPoint) []JSONTimeCountPoint {
	out := make([]JSONTimeCountPoint, len(points))
	for i, p := range points {
		out[i] = JSONTimeCountPoint{Time: formatJSONTime(p.Time), Count: p.Count}
	}
	return out
}

func TimeNumericPointsJSON(points []statsPkg.TimeNumericPoint) []JSONTimeNumericPoint {
	out := make([]JSONTimeNumericPoint, len(points))
	for i, p := range points {
		out[i] = JSONTimeNumericPoint{Time: formatJSONTime(p.Time), Value: p.Value}
	}
	return out
}

func statusPtrString(status *models.Status) *string {
	if status == nil {
		return nil
	}
	value := string(*status)
	return &value
}

func formatJSONTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func formatJSONTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	value := formatJSONTime(*t)
	return &value
}
