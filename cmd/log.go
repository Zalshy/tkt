package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/output"
	"github.com/zalshy/tkt/internal/session"
	"github.com/zalshy/tkt/internal/ticket"
	"github.com/zalshy/tkt/internal/usage"
)

var (
	logTokens   int
	logTools    int
	logDuration int
	logAgent    string
	logLabel    string
)

var logCmd = &cobra.Command{
	Use:   "log <ticket-id[,id...]> --tokens N [--tools N] [--duration N] [--agent role] [--label text]",
	Short: "Record token/tool/duration usage against a ticket",
	Args:  cobra.ExactArgs(1),
	RunE:  runLog,
}

func init() {
	logCmd.Flags().IntVar(&logTokens, "tokens", 0, "number of tokens used (required, must be > 0)")
	logCmd.Flags().IntVar(&logTools, "tools", 0, "number of tool calls (optional)")
	logCmd.Flags().IntVar(&logDuration, "duration", 0, "duration in seconds (optional)")
	logCmd.Flags().StringVar(&logAgent, "agent", "", "agent role (optional)")
	logCmd.Flags().StringVar(&logLabel, "label", "", "free annotation (optional)")
	rootCmd.AddCommand(logCmd)
}

func runLog(cmd *cobra.Command, args []string) error {
	if logTokens <= 0 {
		return fmt.Errorf("--tokens is required and must be > 0")
	}

	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("log: open db: %w", err)
	}
	defer database.Close()

	sess, err := session.LoadActive(root, database)
	if err != nil {
		if errors.Is(err, session.ErrNoSession) {
			return fmt.Errorf(msgNoSession)
		}
		return fmt.Errorf("log: load session: %w", err)
	}

	// Split comma-separated IDs.
	rawIDs, err := parseIDs(args[0])
	if err != nil {
		return fmt.Errorf("log: %w", err)
	}
	var errs []string
	out := cmd.OutOrStdout()

	for _, rawID := range rawIDs {

		t, err := ticket.GetByID(rawID, database)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", rawID, err))
			continue
		}

		if err := usage.Append(
			context.Background(),
			t.ID,
			sess.ID,
			logTokens,
			logTools,
			logDuration*1000,
			logAgent,
			logLabel,
			database,
		); err != nil {
			errs = append(errs, fmt.Sprintf("#%d: %v", t.ID, err))
			continue
		}

		// Build output line with thousands separators.
		parts := []string{output.FormatIntComma(logTokens) + " tokens"}
		if logTools > 0 {
			parts = append(parts, fmt.Sprintf("%s tools", output.FormatIntComma(logTools)))
		}
		if logDuration > 0 {
			parts = append(parts, fmt.Sprintf("%ds", logDuration))
		}
		detail := strings.Join(parts, ", ")

		line := fmt.Sprintf("#%d  logged %s", t.ID, detail)
		if logAgent != "" {
			line += " — " + logAgent
		}
		fmt.Fprintln(out, line)
	}

	if len(errs) > 0 {
		return fmt.Errorf("log: %d error(s):\n  %s", len(errs), strings.Join(errs, "\n  "))
	}
	return nil
}

