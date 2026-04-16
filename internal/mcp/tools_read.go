package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/zalshy/tkt/internal/docs"
	ilog "github.com/zalshy/tkt/internal/log"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/output"
	"github.com/zalshy/tkt/internal/role"
	"github.com/zalshy/tkt/internal/ticket"
	"github.com/zalshy/tkt/internal/usage"

	ctxpkg "github.com/zalshy/tkt/internal/context"
)

func addReadTools(s *server.MCPServer, root string, db *sql.DB) {
	// tkt_list_tickets
	s.AddTool(
		mcplib.NewTool("tkt_list_tickets",
			mcplib.WithDescription("List tickets. By default returns up to 10 non-VERIFIED tickets."),
			mcplib.WithString("status", mcplib.Description("Filter by status: TODO, PLANNING, PLANNED, IN_PROGRESS, DONE, VERIFIED, CANCELED, ARCHIVED")),
			mcplib.WithBoolean("all", mcplib.Description("Return all tickets without limit")),
			mcplib.WithBoolean("archived", mcplib.Description("Include ARCHIVED tickets")),
			mcplib.WithBoolean("verified", mcplib.Description("Include VERIFIED tickets")),
			mcplib.WithBoolean("ready", mcplib.Description("Only tickets with no unresolved dependencies")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			statusStr := req.GetString("status", "")
			all := req.GetBool("all", false)
			archived := req.GetBool("archived", false)
			verified := req.GetBool("verified", false)
			ready := req.GetBool("ready", false)

			opts := ticket.ListOptions{
				All:             all,
				IncludeArchived: archived,
				IncludeVerified: verified,
				Ready:           ready,
			}

			if statusStr != "" {
				st := models.Status(statusStr)
				opts.Status = &st
			}

			result, err := ticket.List(opts, db)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			if len(result.Tickets) == 0 {
				return mcplib.NewToolResultText("No tickets found."), nil
			}

			var sb strings.Builder
			for _, t := range result.Tickets {
				tier := ""
				if t.Tier != "" && t.Tier != "standard" {
					tier = "  [" + t.Tier + "]"
				}
				sb.WriteString(fmt.Sprintf("#%-5d  %-12s  %s%s\n", t.ID, t.Status, t.Title, tier))
			}
			if result.HasMore {
				sb.WriteString("\n(more results available — use all=true to see all)")
			}

			return mcplib.NewToolResultText(sb.String()), nil
		},
	)

	// tkt_show_ticket
	s.AddTool(
		mcplib.NewTool("tkt_show_ticket",
			mcplib.WithDescription("Show full details of a ticket including log, plan, and usage."),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Ticket ID (e.g. 42 or #42)")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			id := req.GetString("id", "")
			if id == "" {
				return mcplib.NewToolResultError("id is required"), nil
			}

			t, err := ticket.GetByID(id, db)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			entries, err := ilog.GetAll(ctx, id, db)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			usageEntries, err := usage.GetForTicket(ctx, t.ID, db)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			deps, err := ticket.GetDependencies(t.ID, db)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			out := output.RenderTicket(*t, entries, usageEntries)
			out += output.RenderDependencies(deps)
			return mcplib.NewToolResultText(out), nil
		},
	)

	// tkt_search_tickets
	s.AddTool(
		mcplib.NewTool("tkt_search_tickets",
			mcplib.WithDescription("Search tickets by title and/or description."),
			mcplib.WithString("query", mcplib.Required(), mcplib.Description("Search query")),
			mcplib.WithBoolean("all", mcplib.Description("Return all results without limit")),
			mcplib.WithString("status", mcplib.Description("Filter by status")),
			mcplib.WithBoolean("title_only", mcplib.Description("Search title only (not description)")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			query := req.GetString("query", "")
			if query == "" {
				return mcplib.NewToolResultError("query is required"), nil
			}
			all := req.GetBool("all", false)
			statusStr := req.GetString("status", "")
			titleOnly := req.GetBool("title_only", false)

			opts := ticket.ListOptions{
				All:       all,
				Query:     query,
				TitleOnly: titleOnly,
			}
			if statusStr != "" {
				st := models.Status(statusStr)
				opts.Status = &st
			}

			result, err := ticket.List(opts, db)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			if len(result.Tickets) == 0 {
				return mcplib.NewToolResultText("No tickets found matching query."), nil
			}

			var sb strings.Builder
			for _, t := range result.Tickets {
				sb.WriteString(fmt.Sprintf("#%-5d  %-12s  %s\n", t.ID, t.Status, t.Title))
			}
			if result.HasMore {
				sb.WriteString("\n(more results available — use all=true to see all)")
			}
			return mcplib.NewToolResultText(sb.String()), nil
		},
	)

	// tkt_batch
	s.AddTool(
		mcplib.NewTool("tkt_batch",
			mcplib.WithDescription("Show next N executable phases of tickets based on dependencies."),
			mcplib.WithNumber("n", mcplib.Description("Number of phases to return (default 6)")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			n := req.GetInt("n", 6)
			if n < 1 {
				n = 6
			}

			activeTickets, err := ticket.ListActive(db)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			if len(activeTickets) == 0 {
				return mcplib.NewToolResultText("No active tickets."), nil
			}

			edges, err := ticket.ListDependencyEdges(db)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			depCount := make(map[int64]int)
			unlockedBy := make(map[int64][]int64)
			for id := range activeTickets {
				depCount[id] = 0
			}
			for _, e := range edges {
				if e.DepStat == models.StatusVerified || e.DepStat == models.StatusCanceled {
					continue
				}
				if _, ok := activeTickets[e.TicketID]; !ok {
					continue
				}
				depCount[e.TicketID]++
				unlockedBy[e.DependsOn] = append(unlockedBy[e.DependsOn], e.TicketID)
			}

			var seed []int64
			for id, cnt := range depCount {
				if cnt == 0 {
					seed = append(seed, id)
				}
			}
			sort.Slice(seed, func(i, j int) bool { return seed[i] < seed[j] })

			if len(seed) == 0 {
				return mcplib.NewToolResultText("No unblocked tickets found."), nil
			}

			assigned := make(map[int64]bool)
			var phases [][]int64
			current := seed

			for len(phases) < n {
				if len(current) == 0 {
					break
				}
				phases = append(phases, current)
				for _, id := range current {
					assigned[id] = true
				}
				var next []int64
				for _, id := range current {
					for _, downstream := range unlockedBy[id] {
						depCount[downstream]--
						if depCount[downstream] == 0 && !assigned[downstream] {
							next = append(next, downstream)
						}
					}
				}
				sort.Slice(next, func(i, j int) bool { return next[i] < next[j] })
				current = next
			}

			var sb strings.Builder
			for phaseIdx, phase := range phases {
				sb.WriteString(fmt.Sprintf("Phase %d: ", phaseIdx+1))
				parts := make([]string, len(phase))
				for i, id := range phase {
					parts[i] = fmt.Sprintf("#%d (%s)", id, activeTickets[id])
				}
				sb.WriteString(strings.Join(parts, ", "))
				sb.WriteString("\n")
			}
			return mcplib.NewToolResultText(sb.String()), nil
		},
	)

	// tkt_list_context
	s.AddTool(
		mcplib.NewTool("tkt_list_context",
			mcplib.WithDescription("List all project context entries."),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			entries, err := ctxpkg.ReadAll(db)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			if len(entries) == 0 {
				return mcplib.NewToolResultText("No context entries."), nil
			}
			var sb strings.Builder
			for _, e := range entries {
				sb.WriteString(fmt.Sprintf("## Context #%d — %s\n\n%s\n\n", e.ID, e.Title, e.Body))
			}
			return mcplib.NewToolResultText(sb.String()), nil
		},
	)

	// tkt_list_docs
	s.AddTool(
		mcplib.NewTool("tkt_list_docs",
			mcplib.WithDescription("List documents."),
			mcplib.WithBoolean("archived", mcplib.Description("List archived documents")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			archived := req.GetBool("archived", false)
			var dir string
			if archived {
				dir = docs.DocsArchivedDir(root)
			} else {
				dir = docs.DocsDir(root)
			}

			if err := os.MkdirAll(dir, 0o755); err != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("mkdir: %v", err)), nil
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				if os.IsNotExist(err) {
					return mcplib.NewToolResultText("No documents found."), nil
				}
				return mcplib.NewToolResultError(err.Error()), nil
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("%-6s  %-40s  %-12s  %-12s  %s\n", "ID", "TITLE", "TYPE", "DATE", "BY"))
			count := 0
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				if filepath.Ext(name) != ".md" {
					continue
				}
				meta, err := docs.ParseDocMeta(filepath.Join(dir, name))
				if err != nil {
					continue
				}
				sb.WriteString(fmt.Sprintf("%-6s  %-40s  %-12s  %-12s  %s\n", meta.ID, meta.Title, meta.Type, meta.Date, meta.By))
				count++
			}
			if count == 0 {
				return mcplib.NewToolResultText("No documents found."), nil
			}
			return mcplib.NewToolResultText(sb.String()), nil
		},
	)

	// tkt_read_doc
	s.AddTool(
		mcplib.NewTool("tkt_read_doc",
			mcplib.WithDescription("Read the full content of a document by ID or slug."),
			mcplib.WithString("id_or_slug", mcplib.Required(), mcplib.Description("Document ID (e.g. 10) or slug substring")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			idOrSlug := req.GetString("id_or_slug", "")
			if idOrSlug == "" {
				return mcplib.NewToolResultError("id_or_slug is required"), nil
			}

			path, err := docs.ResolveDoc(root, idOrSlug)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			return mcplib.NewToolResultText(string(data)), nil
		},
	)

	// tkt_list_roles
	s.AddTool(
		mcplib.NewTool("tkt_list_roles",
			mcplib.WithDescription("List all roles."),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			roles, err := role.List(db)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			if len(roles) == 0 {
				return mcplib.NewToolResultText("No roles defined."), nil
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("%-20s  %-15s  %s\n", "NAME", "BASE ROLE", "BUILT-IN"))
			for _, r := range roles {
				builtin := "no"
				if r.IsBuiltin {
					builtin = "yes"
				}
				sb.WriteString(fmt.Sprintf("%-20s  %-15s  %s\n", r.Name, r.BaseRole, builtin))
			}
			return mcplib.NewToolResultText(sb.String()), nil
		},
	)

	// suppress unused import warning for strconv if not used elsewhere
	_ = strconv.Itoa
}
