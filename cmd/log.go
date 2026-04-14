package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/db"
	tktlog "github.com/zalshy/tkt/internal/log"
	"github.com/zalshy/tkt/internal/session"
	"github.com/zalshy/tkt/internal/ticket"
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

// usageBody is the JSON structure stored in ticket_log.body for kind="usage".
type usageBody struct {
	Tokens     int    `json:"tokens"`
	Tools      int    `json:"tools,omitempty"`
	DurationMs int    `json:"duration_ms,omitempty"`
	Agent      string `json:"agent,omitempty"`
	Label      string `json:"label,omitempty"`
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

	// Build JSON body.
	ub := usageBody{
		Tokens:     logTokens,
		Tools:      logTools,
		DurationMs: logDuration * 1000, // flag is seconds, store ms
		Agent:      logAgent,
		Label:      logLabel,
	}
	bodyBytes, err := json.Marshal(ub)
	if err != nil {
		return fmt.Errorf("log: marshal body: %w", err)
	}
	body := string(bodyBytes)

	// Split comma-separated IDs.
	rawIDs := strings.Split(args[0], ",")
	var errs []string
	out := cmd.OutOrStdout()

	for _, rawID := range rawIDs {
		rawID = strings.TrimSpace(rawID)
		if rawID == "" {
			continue
		}

		t, err := ticket.GetByID(rawID, database)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", rawID, err))
			continue
		}

		if err := tktlog.Append(t.ID, "usage", body, nil, nil, sess, database); err != nil {
			errs = append(errs, fmt.Sprintf("#%d: %v", t.ID, err))
			continue
		}

		// Build output line with thousands separators.
		parts := []string{formatInt(logTokens) + " tokens"}
		if logTools > 0 {
			parts = append(parts, fmt.Sprintf("%s tools", formatInt(logTools)))
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

// formatInt formats an integer with thousands separators (e.g. 78155 → "78,155").
func formatInt(n int) string {
	s := fmt.Sprintf("%d", n)
	if n < 0 {
		s = s[1:]
	}
	// Insert commas from right.
	result := []byte{}
	for i, c := range []byte(s) {
		pos := len(s) - i
		if i > 0 && pos%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, c)
	}
	if n < 0 {
		return "-" + string(result)
	}
	return string(result)
}
