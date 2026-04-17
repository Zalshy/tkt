package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	ctxpkg "github.com/zalshy/tkt/internal/context"
	idb "github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/docs"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/role"
)

func addAdminTools(s *server.MCPServer, root string, db *sql.DB, sess *models.Session) {
	// tkt_add_context
	s.AddTool(
		mcplib.NewTool("tkt_add_context",
			mcplib.WithDescription("Add a new project context entry."),
			mcplib.WithString("title", mcplib.Required(), mcplib.Description("Context entry title")),
			mcplib.WithString("body", mcplib.Required(), mcplib.Description("Context entry body")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			title := req.GetString("title", "")
			if title == "" {
				return mcplib.NewToolResultError("title is required"), nil
			}
			body := req.GetString("body", "")
			if body == "" {
				return mcplib.NewToolResultError("body is required"), nil
			}

			entry, err := ctxpkg.Add(title, body, sess, db)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			return mcplib.NewToolResultText(fmt.Sprintf("Context #%d added: %s", entry.ID, entry.Title)), nil
		},
	)

	// tkt_update_context
	s.AddTool(
		mcplib.NewTool("tkt_update_context",
			mcplib.WithDescription("Update an existing project context entry."),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Context entry ID")),
			mcplib.WithString("title", mcplib.Required(), mcplib.Description("New title")),
			mcplib.WithString("body", mcplib.Required(), mcplib.Description("New body")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			idStr := req.GetString("id", "")
			if idStr == "" {
				return mcplib.NewToolResultError("id is required"), nil
			}
			title := req.GetString("title", "")
			if title == "" {
				return mcplib.NewToolResultError("title is required"), nil
			}
			body := req.GetString("body", "")
			if body == "" {
				return mcplib.NewToolResultError("body is required"), nil
			}

			n, err := strconv.Atoi(strings.TrimSpace(idStr))
			if err != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("invalid context id %q", idStr)), nil
			}

			if err := ctxpkg.Update(n, title, body, sess, db); err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			return mcplib.NewToolResultText(fmt.Sprintf("Context #%d updated", n)), nil
		},
	)

	// tkt_delete_context
	s.AddTool(
		mcplib.NewTool("tkt_delete_context",
			mcplib.WithDescription("Delete (soft-delete) a project context entry."),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Context entry ID")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			idStr := req.GetString("id", "")
			if idStr == "" {
				return mcplib.NewToolResultError("id is required"), nil
			}

			n, err := strconv.Atoi(strings.TrimSpace(idStr))
			if err != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("invalid context id %q", idStr)), nil
			}

			if err := ctxpkg.Delete(n, db); err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			return mcplib.NewToolResultText(fmt.Sprintf("Context #%d deleted", n)), nil
		},
	)

	// tkt_add_doc
	s.AddTool(
		mcplib.NewTool("tkt_add_doc",
			mcplib.WithDescription("Create a new document file."),
			mcplib.WithString("slug", mcplib.Required(), mcplib.Description("URL-safe slug (lowercase alphanumeric and hyphens)")),
			mcplib.WithString("title", mcplib.Required(), mcplib.Description("Document title")),
			mcplib.WithString("body", mcplib.Required(), mcplib.Description("Document body content")),
			mcplib.WithString("type", mcplib.Required(), mcplib.Description("Document type: analysis, plan, post-mortem, summary, or design")),
			mcplib.WithString("by", mcplib.Description("Author (optional, defaults to session role)")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			slug := req.GetString("slug", "")
			if slug == "" {
				return mcplib.NewToolResultError("slug is required"), nil
			}
			title := req.GetString("title", "")
			if title == "" {
				return mcplib.NewToolResultError("title is required"), nil
			}
			body := req.GetString("body", "")
			if body == "" {
				return mcplib.NewToolResultError("body is required"), nil
			}
			docType := req.GetString("type", "")
			if docType == "" {
				return mcplib.NewToolResultError("type is required"), nil
			}
			by := req.GetString("by", "")
			if by == "" {
				by = string(sess.Role)
			}

			if err := docs.ValidateSlug(slug); err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			id, err := docs.NextDocID(root)
			if err != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("next doc id: %v", err)), nil
			}

			content := fmt.Sprintf("# %s — %s\n\n**Type:** %s\n**Date:** %s\n**By:** %s\n\n---\n\n%s\n",
				id, title, docType, time.Now().Format("2006-01-02"), by, body)

			filename := id + "-" + slug + ".md"

			if err := os.MkdirAll(docs.DocsDir(root), 0o755); err != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("mkdir: %v", err)), nil
			}

			dest := filepath.Join(docs.DocsDir(root), filename)
			if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("write file: %v", err)), nil
			}

			return mcplib.NewToolResultText(fmt.Sprintf("created docs/%s", filename)), nil
		},
	)

	// tkt_archive_doc
	s.AddTool(
		mcplib.NewTool("tkt_archive_doc",
			mcplib.WithDescription("Move a document to the archived directory."),
			mcplib.WithString("id_or_slug", mcplib.Required(), mcplib.Description("Document ID or slug substring")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			idOrSlug := req.GetString("id_or_slug", "")
			if idOrSlug == "" {
				return mcplib.NewToolResultError("id_or_slug is required"), nil
			}

			src, err := docs.ResolveDoc(root, idOrSlug)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			archivedDir := docs.DocsArchivedDir(root)
			if err := os.MkdirAll(archivedDir, 0o755); err != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("mkdir archived: %v", err)), nil
			}

			dest := filepath.Join(archivedDir, filepath.Base(src))
			if err := os.Rename(src, dest); err != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("rename: %v", err)), nil
			}

			return mcplib.NewToolResultText(fmt.Sprintf("archived: docs/archived/%s", filepath.Base(src))), nil
		},
	)

	// tkt_create_role
	s.AddTool(
		mcplib.NewTool("tkt_create_role",
			mcplib.WithDescription("Create a new user-defined role."),
			mcplib.WithString("name", mcplib.Required(), mcplib.Description("Role name (lowercase alphanumeric/underscores)")),
			mcplib.WithString("like", mcplib.Required(), mcplib.Description("Base role: architect or implementer")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			name := req.GetString("name", "")
			if name == "" {
				return mcplib.NewToolResultError("name is required"), nil
			}
			like := req.GetString("like", "")
			if like == "" {
				return mcplib.NewToolResultError("like is required"), nil
			}

			if err := role.Create(name, like, db); err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			return mcplib.NewToolResultText(fmt.Sprintf("Role '%s' created (behaves like %s).", name, like)), nil
		},
	)

	// tkt_delete_role
	s.AddTool(
		mcplib.NewTool("tkt_delete_role",
			mcplib.WithDescription("Delete a user-defined role."),
			mcplib.WithString("name", mcplib.Required(), mcplib.Description("Role name to delete")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			name := req.GetString("name", "")
			if name == "" {
				return mcplib.NewToolResultError("name is required"), nil
			}

			if err := role.Delete(name, db); err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			return mcplib.NewToolResultText(fmt.Sprintf("Role '%s' deleted.", name)), nil
		},
	)

	// tkt_cleanup
	s.AddTool(
		mcplib.NewTool("tkt_cleanup",
			mcplib.WithDescription("Expire stale sessions (inactive for more than 4 hours)."),
			mcplib.WithBoolean("dry_run", mcplib.Description("Count stale sessions without expiring them")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			dryRun := req.GetBool("dry_run", false)

			n, err := idb.CleanupStaleSessions(db, dryRun)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			if dryRun {
				return mcplib.NewToolResultText(fmt.Sprintf("dry_run: %d stale session(s) would be expired", n)), nil
			}
			return mcplib.NewToolResultText(fmt.Sprintf("%d stale session(s) expired", n)), nil
		},
	)
}
