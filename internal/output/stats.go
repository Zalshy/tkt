package output

import (
	"fmt"
	"math"
	"strings"
	"time"

	statsPkg "github.com/zalshy/tkt/internal/stats"
)

// RenderStats renders a stats report as deterministic plain terminal text.
func RenderStats(report statsPkg.Report) string {
	var b strings.Builder

	b.WriteString("Stats\n")
	b.WriteString("\nOverview\n")
	fmt.Fprintf(&b, "  Total: %s\n", FormatIntComma(report.Overview.Total))
	fmt.Fprintf(&b, "  Active: %s\n", FormatIntComma(report.Overview.Active))
	fmt.Fprintf(&b, "  Done: %s\n", FormatIntComma(report.Overview.Done))
	fmt.Fprintf(&b, "  Verified: %s\n", FormatIntComma(report.Overview.Verified))
	fmt.Fprintf(&b, "  Archived: %s\n", FormatIntComma(report.Overview.Archived))
	fmt.Fprintf(&b, "  Mean cycle time: %s\n", FormatDuration(report.Overview.CycleTime.Mean))
	fmt.Fprintf(&b, "  Mean lead time: %s\n", FormatDuration(report.Overview.LeadTime.Mean))
	fmt.Fprintf(&b, "  Throughput: %s\n", formatStatsFloat(report.Overview.Throughput.Total))
	fmt.Fprintf(&b, "  Tokens: %s\n", formatStatsFloat(report.Overview.ResourceUse.Total))

	b.WriteString("\nCycle Time\n")
	writeStatsDurationSummary(&b, report.CycleTime.Summary)
	fmt.Fprintf(&b, "  Trend: %s\n", durationSparkBar(report.CycleTime.Trend))

	b.WriteString("\nThroughput\n")
	fmt.Fprintf(&b, "  Total completed: %s\n", FormatIntComma(report.Throughput.Total))
	fmt.Fprintf(&b, "  Daily: %s\n", countSparkBar(report.Throughput.ByDay))
	fmt.Fprintf(&b, "  Weekly: %s\n", countSparkBar(report.Throughput.ByWeek))

	b.WriteString("\nResource Burn\n")
	b.WriteString("  Tokens\n")
	writeStatsNumericSummary(&b, report.ResourceBurn.Tokens, "    ")
	b.WriteString("  Tools\n")
	writeStatsNumericSummary(&b, report.ResourceBurn.Tools, "    ")
	b.WriteString("  Duration\n")
	writeStatsDurationSummaryWithIndent(&b, report.ResourceBurn.Duration, "    ")
	fmt.Fprintf(&b, "  Token trend: %s\n", numericSparkBar(report.ResourceBurn.Series))
	if len(report.ResourceBurn.ByType) > 0 {
		b.WriteString("  By type\n")
		for _, item := range report.ResourceBurn.ByType {
			label := item.Label
			if label == "" {
				label = "(none)"
			}
			fmt.Fprintf(&b, "    %s: total %s, mean %s\n", label, formatStatsFloat(item.Summary.Total), formatStatsFloat(item.Summary.Mean))
		}
	}

	b.WriteString("\nDistribution\n")
	writeStatsBuckets(&b, "Status", report.Distribution.Status)
	writeStatsBuckets(&b, "Tier", report.Distribution.Tier)
	writeStatsBuckets(&b, "Type", report.Distribution.Type)

	return b.String()
}

func writeStatsDurationSummary(b *strings.Builder, summary statsPkg.DurationSummary) {
	writeStatsDurationSummaryWithIndent(b, summary, "  ")
}

func writeStatsDurationSummaryWithIndent(b *strings.Builder, summary statsPkg.DurationSummary, indent string) {
	fmt.Fprintf(b, "%sCount: %s\n", indent, FormatIntComma(summary.Count))
	fmt.Fprintf(b, "%sTotal: %s\n", indent, FormatDuration(summary.Total))
	fmt.Fprintf(b, "%sMean: %s\n", indent, FormatDuration(summary.Mean))
	fmt.Fprintf(b, "%sMedian: %s\n", indent, FormatDuration(summary.Median))
}

func writeStatsNumericSummary(b *strings.Builder, summary statsPkg.NumericSummary, indent string) {
	fmt.Fprintf(b, "%sCount: %s\n", indent, FormatIntComma(summary.Count))
	fmt.Fprintf(b, "%sTotal: %s\n", indent, formatStatsFloat(summary.Total))
	fmt.Fprintf(b, "%sMean: %s\n", indent, formatStatsFloat(summary.Mean))
	fmt.Fprintf(b, "%sMedian: %s\n", indent, formatStatsFloat(summary.Median))
}

func writeStatsBuckets(b *strings.Builder, title string, buckets []statsPkg.CountBucket) {
	fmt.Fprintf(b, "  %s\n", title)
	if len(buckets) == 0 {
		b.WriteString("    (none)\n")
		return
	}
	for _, bucket := range buckets {
		label := bucket.Label
		if label == "" {
			label = "(none)"
		}
		fmt.Fprintf(b, "    %s: %s\n", label, FormatIntComma(bucket.Count))
	}
}

// FormatDuration formats a duration for stats output.
func FormatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	if d < 0 {
		return "-" + FormatDuration(-d)
	}

	seconds := int64(d.Round(time.Second).Seconds())
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}

	days := seconds / 86400
	seconds %= 86400
	hours := seconds / 3600
	seconds %= 3600
	minutes := seconds / 60
	seconds %= 60

	parts := make([]string, 0, 4)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 && days == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}
	return strings.Join(parts, " ")
}

// SparkBar renders integer values as a compact unicode sparkline.
func SparkBar(values []int) string {
	if len(values) == 0 {
		return "(none)"
	}

	maxValue := 0
	for _, value := range values {
		if value > maxValue {
			maxValue = value
		}
	}
	if maxValue == 0 {
		return strings.Repeat("▁", len(values))
	}

	blocks := []rune("▁▂▃▄▅▆▇█")
	var b strings.Builder
	for _, value := range values {
		idx := int(math.Ceil(float64(value)/float64(maxValue)*float64(len(blocks)))) - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		b.WriteRune(blocks[idx])
	}
	return b.String()
}

func countSparkBar(points []statsPkg.TimeCountPoint) string {
	values := make([]int, 0, len(points))
	for _, point := range points {
		values = append(values, point.Count)
	}
	return SparkBar(values)
}

func durationSparkBar(points []statsPkg.TimeDurationPoint) string {
	values := make([]int, 0, len(points))
	for _, point := range points {
		values = append(values, int(point.Duration.Seconds()))
	}
	return SparkBar(values)
}

func numericSparkBar(points []statsPkg.TimeNumericPoint) string {
	values := make([]int, 0, len(points))
	for _, point := range points {
		values = append(values, int(math.Round(point.Value)))
	}
	return SparkBar(values)
}

func formatStatsFloat(value float64) string {
	if value == 0 {
		return "0"
	}
	if math.Mod(value, 1) == 0 {
		return FormatIntComma(int(value))
	}
	return fmt.Sprintf("%.1f", value)
}
