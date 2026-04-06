package cmd

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	internalctx "github.com/zalshy/tkt/internal/context"
	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/output"
	"github.com/zalshy/tkt/internal/session"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Manage the project knowledge base",
}

var contextAddCmd = &cobra.Command{
	Use:   "add \"<title>\" \"<body>\"",
	Short: "Add a new context entry",
	Args:  cobra.ExactArgs(2),
	RunE:  runContextAdd,
}

var contextReadAllCmd = &cobra.Command{
	Use:   "readall",
	Short: "Print all context entries",
	Args:  cobra.NoArgs,
	RunE:  runContextReadAll,
}

var contextReadCmd = &cobra.Command{
	Use:   "read <id>",
	Short: "Print a single context entry",
	Args:  cobra.ExactArgs(1),
	RunE:  runContextRead,
}

var contextUpdateCmd = &cobra.Command{
	Use:   "update <id> \"<title>\" \"<body>\"",
	Short: "Update a context entry",
	Args:  cobra.ExactArgs(3),
	RunE:  runContextUpdate,
}

var contextDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a context entry",
	Args:  cobra.ExactArgs(1),
	RunE:  runContextDelete,
}

func init() {
	contextCmd.AddCommand(contextAddCmd)
	contextCmd.AddCommand(contextReadAllCmd)
	contextCmd.AddCommand(contextReadCmd)
	contextCmd.AddCommand(contextUpdateCmd)
	contextCmd.AddCommand(contextDeleteCmd)
	rootCmd.AddCommand(contextCmd)
}

func runContextAdd(cmd *cobra.Command, args []string) error {
	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("context add: open db: %w", err)
	}
	defer database.Close()

	sess, err := session.LoadActive(root, database)
	if err != nil {
		if errors.Is(err, session.ErrNoSession) {
			return fmt.Errorf("tkt context add requires an active session. Run: tkt session --role implementer")
		}
		return fmt.Errorf("context add: load session: %w", err)
	}

	c, err := internalctx.Add(args[0], args[1], sess, database)
	if err != nil {
		return fmt.Errorf("context add: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Context #%d added.\n", c.ID)
	return nil
}

func runContextReadAll(cmd *cobra.Command, args []string) error {
	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("context readall: open db: %w", err)
	}
	defer database.Close()

	entries, err := internalctx.ReadAll(database)
	if err != nil {
		return fmt.Errorf("context readall: %w", err)
	}

	fmt.Fprint(cmd.OutOrStdout(), output.RenderContextList(entries))
	return nil
}

func runContextRead(cmd *cobra.Command, args []string) error {
	id, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("context read: invalid id %q", args[0])
	}

	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("context read: open db: %w", err)
	}
	defer database.Close()

	c, err := internalctx.Read(id, database)
	if err != nil {
		if errors.Is(err, internalctx.ErrNotFound) {
			return fmt.Errorf("context #%d not found", id)
		}
		return fmt.Errorf("context read: %w", err)
	}

	fmt.Fprint(cmd.OutOrStdout(), output.RenderContextList([]models.Context{*c}))
	return nil
}

func runContextUpdate(cmd *cobra.Command, args []string) error {
	id, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("context update: invalid id %q", args[0])
	}

	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("context update: open db: %w", err)
	}
	defer database.Close()

	sess, err := session.LoadActive(root, database)
	if err != nil {
		if errors.Is(err, session.ErrNoSession) {
			return fmt.Errorf("tkt context update requires an active session. Run: tkt session --role implementer")
		}
		return fmt.Errorf("context update: load session: %w", err)
	}

	if err := internalctx.Update(id, args[1], args[2], sess, database); err != nil {
		if errors.Is(err, internalctx.ErrNotFound) {
			return fmt.Errorf("context #%d not found", id)
		}
		return fmt.Errorf("context update: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Context #%d updated.\n", id)
	return nil
}

func runContextDelete(cmd *cobra.Command, args []string) error {
	id, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("context delete: invalid id %q", args[0])
	}

	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("context delete: open db: %w", err)
	}
	defer database.Close()

	sess, err := session.LoadActive(root, database)
	if err != nil {
		if errors.Is(err, session.ErrNoSession) {
			return fmt.Errorf("tkt context delete requires an active session. Run: tkt session --role implementer")
		}
		return fmt.Errorf("context delete: load session: %w", err)
	}
	_ = sess // session required but not passed to Delete (soft-delete doesn't need actor)

	if err := internalctx.Delete(id, database); err != nil {
		if errors.Is(err, internalctx.ErrNotFound) {
			return fmt.Errorf("context #%d not found", id)
		}
		return fmt.Errorf("context delete: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Context #%d deleted.\n", id)
	return nil
}
