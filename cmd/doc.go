package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/docs"
	"github.com/zalshy/tkt/internal/session"
)

var docCmd = &cobra.Command{
	Use:   "doc",
	Short: "Manage documents",
}

var docListArchived bool

var docListCmd = &cobra.Command{
	Use:   "list",
	Short: "List documents",
	Args:  cobra.NoArgs,
	RunE:  runDocList,
}

var docAddCmd = &cobra.Command{
	Use:   "add <slug>",
	Short: "Create a new document",
	Args:  cobra.ExactArgs(1),
	RunE:  runDocAdd,
}

var docReadCmd = &cobra.Command{
	Use:   "read <id|slug>",
	Short: "Print a document to stdout",
	Args:  cobra.ExactArgs(1),
	RunE:  runDocRead,
}

var docArchiveCmd = &cobra.Command{
	Use:   "archive <id|slug>",
	Short: "Move a document to archived/",
	Args:  cobra.ExactArgs(1),
	RunE:  runDocArchive,
}

func init() {
	docListCmd.Flags().BoolVar(&docListArchived, "archived", false, "list archived documents")
	docCmd.AddCommand(docListCmd, docAddCmd, docReadCmd, docArchiveCmd)
	rootCmd.AddCommand(docCmd)
}

func runDocList(cmd *cobra.Command, args []string) error {
	root, err := requireRoot()
	if err != nil {
		return err
	}

	var dir string
	if docListArchived {
		dir = docs.DocsArchivedDir(root)
	} else {
		dir = docs.DocsDir(root)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("doc list: mkdir: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("doc list: read dir: %w", err)
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tTYPE\tDATE\tBY")

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".md" {
			continue
		}
		if name == "archived" {
			continue
		}
		meta, err := docs.ParseDocMeta(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("doc list: parse %s: %w", name, err)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", meta.ID, meta.Title, meta.Type, meta.Date, meta.By)
	}

	return w.Flush()
}

func runDocAdd(cmd *cobra.Command, args []string) error {
	slug := args[0]

	if err := docs.ValidateSlug(slug); err != nil {
		return fmt.Errorf("doc add: %w", err)
	}

	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("doc add: open db: %w", err)
	}
	defer database.Close()

	sess, err := session.LoadActive(root, database)
	if err != nil {
		if errors.Is(err, session.ErrNoSession) {
			return fmt.Errorf("tkt doc add requires an active session. Run: tkt session --role implementer")
		}
		return fmt.Errorf("doc add: load session: %w", err)
	}

	id, err := docs.NextDocID(root)
	if err != nil {
		return fmt.Errorf("doc add: next id: %w", err)
	}

	template := fmt.Sprintf("# %s — \n\n**Type:** analysis | plan | post-mortem | summary | design\n**Date:** 2026-04-05\n**By:** %s\n\n---\n\n(body)\n", id, string(sess.Role))

	tmp, err := os.CreateTemp("", "tkt-doc-*.md")
	if err != nil {
		return fmt.Errorf("doc add: create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.WriteString(template); err != nil {
		tmp.Close()
		return fmt.Errorf("doc add: write template: %w", err)
	}
	tmp.Close()

	bin, extraArgs, err := resolveEditor(os.Getenv("EDITOR"))
	if err != nil {
		return fmt.Errorf("doc add: %w", err)
	}

	editorArgs := append(extraArgs, tmpPath)
	editorCmd := exec.Command(bin, editorArgs...)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("doc add: editor: %w", err)
	}

	content, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("doc add: read temp file: %w", err)
	}

	if string(content) == template {
		fmt.Fprintln(cmd.OutOrStdout(), "No changes made.")
		return nil
	}

	if err := os.MkdirAll(docs.DocsDir(root), 0o755); err != nil {
		return fmt.Errorf("doc add: mkdir: %w", err)
	}

	filename := id + "-" + slug + ".md"
	dest := filepath.Join(docs.DocsDir(root), filename)
	if err := os.WriteFile(dest, content, 0o644); err != nil {
		return fmt.Errorf("doc add: write file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "created docs/%s\n", filename)
	return nil
}

func runDocRead(cmd *cobra.Command, args []string) error {
	root, err := requireRoot()
	if err != nil {
		return err
	}

	path, err := docs.ResolveDoc(root, args[0])
	if err != nil {
		return fmt.Errorf("doc read: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("doc read: %w", err)
	}

	_, err = cmd.OutOrStdout().Write(data)
	return err
}

func runDocArchive(cmd *cobra.Command, args []string) error {
	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("doc archive: open db: %w", err)
	}
	defer database.Close()

	_, err = session.LoadActive(root, database)
	if err != nil {
		if errors.Is(err, session.ErrNoSession) {
			return fmt.Errorf("tkt doc archive requires an active session. Run: tkt session --role implementer")
		}
		return fmt.Errorf("doc archive: load session: %w", err)
	}

	src, err := docs.ResolveDoc(root, args[0])
	if err != nil {
		return fmt.Errorf("doc archive: %w", err)
	}

	archivedDir := docs.DocsArchivedDir(root)
	if err := os.MkdirAll(archivedDir, 0o755); err != nil {
		return fmt.Errorf("doc archive: mkdir: %w", err)
	}

	dest := filepath.Join(archivedDir, filepath.Base(src))
	if err := os.Rename(src, dest); err != nil {
		return fmt.Errorf("doc archive: rename: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "archived: docs/archived/%s\n", filepath.Base(src))
	return nil
}

