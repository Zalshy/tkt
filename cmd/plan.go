package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/log"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/session"
	"github.com/zalshy/tkt/internal/ticket"
)

var planCmd = &cobra.Command{
	Use:   "plan <ticket-id>",
	Short: "Write or revise the plan for a ticket",
	Args:  cobra.ExactArgs(1),
	RunE:  runPlan,
}

func init() {
	rootCmd.AddCommand(planCmd)
	planCmd.Flags().String("body", "", "plan body for non-interactive use (skips $EDITOR when set)")
	planCmd.Flags().Bool("stdin", false, "read plan body from stdin")
	planCmd.Flags().String("file", "", "read plan body from file at path")
	planCmd.MarkFlagsMutuallyExclusive("body", "stdin")
	planCmd.MarkFlagsMutuallyExclusive("body", "file")
	planCmd.MarkFlagsMutuallyExclusive("stdin", "file")
}

func runPlan(cmd *cobra.Command, args []string) error {
	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("plan: open db: %w", err)
	}
	defer database.Close()

	sess, err := session.LoadActive(root, database)
	if err != nil {
		if errors.Is(err, session.ErrNoSession) {
			return fmt.Errorf(msgNoSession)
		}
		return fmt.Errorf("plan: load session: %w", err)
	}

	t, err := ticket.GetByID(args[0], database)
	if err != nil {
		return err
	}

	if t.Status != models.StatusPlanning {
		return fmt.Errorf("cannot edit plan — ticket #%d is in %s state (plan is frozen once approved)", t.ID, t.Status)
	}

	planBody, err := readPlanBody(cmd)
	if err != nil {
		return err
	}

	// Non-empty planBody means a flag was supplied — save and return.
	// ("", nil) means no flag set; fall through to $EDITOR path below.
	if planBody != "" {
		if err := log.Append(context.Background(), t.ID, "plan", planBody, nil, nil, sess, database); err != nil {
			return fmt.Errorf("plan: save: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Plan updated for #%d\n", t.ID)
		return nil
	}

	entry, err := log.LatestPlan(context.Background(), strconv.FormatInt(t.ID, 10), database)
	if err != nil {
		return fmt.Errorf("plan: load existing plan: %w", err)
	}
	existingContent := ""
	if entry != nil {
		existingContent = entry.Body
	}

	tmpFile, err := os.CreateTemp("", "tkt-plan-*.md")
	if err != nil {
		return fmt.Errorf("plan: create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(existingContent); err != nil {
		tmpFile.Close()
		return fmt.Errorf("plan: write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("plan: close temp file: %w", err)
	}

	bin, extraArgs, err := resolveEditor(os.Getenv("EDITOR"))
	if err != nil {
		return err
	}

	editorArgs := append(extraArgs, tmpFile.Name())
	editorCmd := exec.Command(bin, editorArgs...)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("plan: editor exited with error: %w", err)
	}

	newContent, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("plan: read temp file: %w", err)
	}

	if bytes.Equal([]byte(existingContent), newContent) {
		fmt.Fprintln(cmd.OutOrStdout(), "No changes made.")
		return nil
	}

	if err := log.Append(context.Background(), t.ID, "plan", string(newContent), nil, nil, sess, database); err != nil {
		return fmt.Errorf("plan: save: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Plan updated for #%d\n", t.ID)
	return nil
}

// readPlanBody reads plan content from whichever of --body, --stdin, or --file was supplied.
// Returns ("", nil) when no flag is set — caller should fall through to $EDITOR.
func readPlanBody(cmd *cobra.Command) (string, error) {
	body, err := cmd.Flags().GetString("body")
	if err != nil {
		return "", fmt.Errorf("plan: get --body flag: %w", err)
	}
	if body = strings.TrimSpace(body); body != "" {
		return body, nil
	}

	useStdin, err := cmd.Flags().GetBool("stdin")
	if err != nil {
		return "", fmt.Errorf("plan: get --stdin flag: %w", err)
	}
	if useStdin {
		raw, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", fmt.Errorf("plan: read stdin: %w", err)
		}
		body = strings.TrimSpace(string(raw))
		if body == "" {
			return "", fmt.Errorf("plan: body is empty")
		}
		return body, nil
	}

	filePath, err := cmd.Flags().GetString("file")
	if err != nil {
		return "", fmt.Errorf("plan: get --file flag: %w", err)
	}
	if filePath != "" {
		raw, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("plan: read file: %w", err)
		}
		body = strings.TrimSpace(string(raw))
		if body == "" {
			return "", fmt.Errorf("plan: body is empty")
		}
		return body, nil
	}

	// No flag set — signal caller to fall through to $EDITOR.
	return "", nil
}

// resolveEditor picks an editor binary and any extra args from the $EDITOR env value.
// It tries the $EDITOR value first, then falls back to nano and vi in order.
// Returns an error if no usable editor is found.
func resolveEditor(envValue string) (bin string, extraArgs []string, err error) {
	type candidate struct {
		bin  string
		args []string
	}

	var candidates []candidate

	fields := strings.Fields(envValue)
	if len(fields) > 0 {
		candidates = append(candidates, candidate{fields[0], fields[1:]})
	}
	candidates = append(candidates,
		candidate{"nano", nil},
		candidate{"vi", nil},
	)

	for _, c := range candidates {
		if path, lookErr := exec.LookPath(c.bin); lookErr == nil {
			return path, c.args, nil
		}
	}

	return "", nil, fmt.Errorf("no editor found — set $EDITOR or install nano or vi")
}
