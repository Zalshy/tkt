package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/output"
	statsPkg "github.com/zalshy/tkt/internal/stats"
)

var (
	statsSince           string
	statsUntil           string
	statsWindow          string
	statsStatus          string
	statsTier            string
	statsType            string
	statsCreatedBy       string
	statsIncludeVerified bool
	statsIncludeArchived bool
	statsJSON            bool
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show project statistics",
	Args:  cobra.NoArgs,
	RunE:  runStats,
}

func init() {
	statsCmd.Flags().StringVar(&statsSince, "since", "", "include ticket activity on or after YYYY-MM-DD")
	statsCmd.Flags().StringVar(&statsUntil, "until", "", "include ticket activity on or before YYYY-MM-DD")
	statsCmd.Flags().StringVar(&statsWindow, "window", "", "include ticket activity in the last duration (e.g. 24h, 7d, 30d)")
	statsCmd.Flags().StringVar(&statsStatus, "status", "", "filter by status (TODO, PLANNING, IN_PROGRESS, DONE, VERIFIED, CANCELED, ARCHIVED)")
	statsCmd.Flags().StringVar(&statsTier, "tier", "", "filter by tier (critical, standard, low)")
	statsCmd.Flags().StringVar(&statsType, "type", "", "filter by main type")
	statsCmd.Flags().StringVar(&statsCreatedBy, "created-by", "", "filter by creator session name")
	statsCmd.Flags().BoolVar(&statsIncludeVerified, "verified", false, "include VERIFIED tickets")
	statsCmd.Flags().BoolVar(&statsIncludeArchived, "archived", false, "include ARCHIVED tickets")
	statsCmd.Flags().BoolVar(&statsJSON, "json", false, "output machine-readable JSON")
	rootCmd.AddCommand(statsCmd)
}

func runStats(cmd *cobra.Command, args []string) error {
	opts, err := statsOptionsFromFlags()
	if err != nil {
		return err
	}

	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("stats: open db: %w", err)
	}
	defer database.Close()

	report, err := statsPkg.Compute(database, opts)
	if err != nil {
		return fmt.Errorf("stats: compute: %w", err)
	}

	out := cmd.OutOrStdout()
	defaultScope := statsDefaultScopeActive()
	if statsJSON {
		payload := map[string]any{
			"default_scope": defaultScope,
			"report":        output.StatsReportJSON(report),
		}
		return output.WriteJSON(out, payload)
	}
	if defaultScope {
		fmt.Fprintln(out, "Scope: default last 24 hours, all ticket types and statuses")
		fmt.Fprintln(out)
	}
	fmt.Fprint(out, output.RenderStats(report))
	return nil
}

func statsOptionsFromFlags() (statsPkg.Options, error) {
	defaultScope := statsDefaultScopeActive()
	opts := statsPkg.Options{
		Tier:            statsTier,
		Type:            statsType,
		CreatedBy:       statsCreatedBy,
		IncludeVerified: statsIncludeVerified || defaultScope,
		IncludeArchived: statsIncludeArchived || defaultScope,
	}
	if defaultScope {
		since := time.Now().Add(-24 * time.Hour)
		opts.Since = &since
	}

	if statsWindow != "" {
		if statsSince != "" || statsUntil != "" {
			return statsPkg.Options{}, fmt.Errorf("stats: --window cannot be combined with --since or --until")
		}
		window, err := statsPkg.ParseWindow(statsWindow)
		if err != nil {
			return statsPkg.Options{}, fmt.Errorf("stats: %w", err)
		}
		since := time.Now().Add(-window)
		opts.Since = &since
	}

	if statsSince != "" {
		since, err := parseStatsDate(statsSince)
		if err != nil {
			return statsPkg.Options{}, fmt.Errorf("stats: invalid --since %q: use YYYY-MM-DD", statsSince)
		}
		opts.Since = &since
	}

	if statsUntil != "" {
		until, err := parseStatsDate(statsUntil)
		if err != nil {
			return statsPkg.Options{}, fmt.Errorf("stats: invalid --until %q: use YYYY-MM-DD", statsUntil)
		}
		until = until.Add(24*time.Hour - time.Nanosecond)
		opts.Until = &until
	}

	if opts.Since != nil && opts.Until != nil && opts.Since.After(*opts.Until) {
		return statsPkg.Options{}, fmt.Errorf("stats: --since must be before or equal to --until")
	}

	if statsStatus != "" {
		if !validStatuses[statsStatus] {
			return statsPkg.Options{}, fmt.Errorf("stats: invalid --status %q: must be one of TODO, PLANNING, IN_PROGRESS, DONE, VERIFIED, CANCELED, ARCHIVED", statsStatus)
		}
		status := models.Status(statsStatus)
		opts.Status = &status
	}

	if statsTier != "" && statsTier != "critical" && statsTier != "standard" && statsTier != "low" {
		return statsPkg.Options{}, fmt.Errorf("stats: invalid --tier %q: must be critical, standard, or low", statsTier)
	}

	return opts, nil
}

func parseStatsDate(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", value, time.UTC)
}

func statsDefaultScopeActive() bool {
	return statsSince == "" &&
		statsUntil == "" &&
		statsWindow == "" &&
		statsStatus == "" &&
		statsTier == "" &&
		statsType == "" &&
		statsCreatedBy == "" &&
		!statsIncludeVerified &&
		!statsIncludeArchived
}
