package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	ilog "github.com/zalshy/tkt/internal/log"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/state"
	"github.com/zalshy/tkt/internal/ticket"
	"github.com/zalshy/tkt/internal/usage"
)

func addWriteTools(s *server.MCPServer, root string, db *sql.DB, sess *models.Session) {
	// tkt_new_ticket
	s.AddTool(
		mcplib.NewTool("tkt_new_ticket",
			mcplib.WithDescription("Create a new ticket."),
			mcplib.WithString("title", mcplib.Required(), mcplib.Description("Ticket title")),
			mcplib.WithString("tier", mcplib.Description("Tier: critical, standard, or low (default: standard)")),
			mcplib.WithString("after", mcplib.Description("Comma-separated dependency ticket IDs (e.g. 5,7)")),
			mcplib.WithString("main_type", mcplib.Description("Ticket type label (optional, max 30 chars, e.g. feature, bugfix, refactor)")),
			mcplib.WithNumber("attention_level", mcplib.Description("Attention level 0–99 (optional; 0 = unset)")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			title := req.GetString("title", "")
			if title == "" {
				return mcplib.NewToolResultError("title is required"), nil
			}
			tier := req.GetString("tier", "standard")
			if tier == "" {
				tier = "standard"
			}

			mainType := req.GetString("main_type", "")
			attentionLevel := req.GetInt("attention_level", 0)
			t, err := ticket.Create(title, "", tier, sess, db, mainType, attentionLevel)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			result := fmt.Sprintf("Created #%d  %q\n", t.ID, t.Title)

			afterStr := req.GetString("after", "")
			if afterStr != "" {
				parts := strings.Split(afterStr, ",")
				var depIDs []int64
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if p == "" {
						continue
					}
					n, err := strconv.ParseInt(p, 10, 64)
					if err != nil {
						return mcplib.NewToolResultError(fmt.Sprintf("invalid dependency ID %q", p)), nil
					}
					depIDs = append(depIDs, n)
				}
				if len(depIDs) > 0 {
					if err := ticket.AddDependencies(t.ID, depIDs, db); err != nil {
						return mcplib.NewToolResultError(fmt.Sprintf("ticket created (#%d) but dependencies failed: %v", t.ID, err)), nil
					}
					strs := make([]string, len(depIDs))
					for i, id := range depIDs {
						strs[i] = fmt.Sprintf("#%d", id)
					}
					result += fmt.Sprintf("Depends on: %s\n", strings.Join(strs, ", "))
				}
			}

			return mcplib.NewToolResultText(result), nil
		},
	)

	// tkt_advance_ticket
	s.AddTool(
		mcplib.NewTool("tkt_advance_ticket",
			mcplib.WithDescription("Advance ticket(s) to the next state (or a specific state)."),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Ticket ID or comma-separated IDs")),
			mcplib.WithString("note", mcplib.Required(), mcplib.Description("Reason for advancing (required)")),
			mcplib.WithString("to", mcplib.Description("Target status (optional, defaults to natural next state)")),
			mcplib.WithBoolean("force", mcplib.Description("Bypass soft validation rules")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			idStr := req.GetString("id", "")
			if idStr == "" {
				return mcplib.NewToolResultError("id is required"), nil
			}
			note := req.GetString("note", "")
			if note == "" {
				return mcplib.NewToolResultError("note is required"), nil
			}
			toStr := req.GetString("to", "")
			force := req.GetBool("force", false)

			var targetStatus models.Status
			if toStr != "" {
				targetStatus = models.Status(toStr)
			}

			// Split comma-separated IDs.
			parts := strings.Split(idStr, ",")
			var errs []string
			var results []string

			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				t, err := ticket.GetByID(p, db)
				if err != nil {
					errs = append(errs, fmt.Sprintf("#%s: %v", p, err))
					continue
				}
				fromStatus := t.Status
				if err := state.Execute(p, targetStatus, note, sess, db, force); err != nil {
					errs = append(errs, fmt.Sprintf("#%s: %v", p, err))
					continue
				}
				to := toStr
				if to == "" {
					// Describe the natural transition
					next, _ := state.NextState(fromStatus)
					to = string(next)
				}
				results = append(results, fmt.Sprintf("#%d  %s → %s", t.ID, fromStatus, to))
			}

			var sb strings.Builder
			for _, r := range results {
				sb.WriteString(r + "\n")
			}
			if len(errs) > 0 {
				sb.WriteString("\nErrors:\n")
				for _, e := range errs {
					sb.WriteString("  " + e + "\n")
				}
			}
			if sb.Len() == 0 {
				return mcplib.NewToolResultError("no tickets processed"), nil
			}
			return mcplib.NewToolResultText(sb.String()), nil
		},
	)

	// tkt_add_comment
	s.AddTool(
		mcplib.NewTool("tkt_add_comment",
			mcplib.WithDescription("Add a comment/message to a ticket's log."),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Ticket ID or comma-separated IDs")),
			mcplib.WithString("body", mcplib.Required(), mcplib.Description("Comment text")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			idStr := req.GetString("id", "")
			if idStr == "" {
				return mcplib.NewToolResultError("id is required"), nil
			}
			body := req.GetString("body", "")
			if body == "" {
				return mcplib.NewToolResultError("body is required"), nil
			}

			parts := strings.Split(idStr, ",")
			var errs []string
			var results []string

			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				t, err := ticket.GetByID(p, db)
				if err != nil {
					errs = append(errs, fmt.Sprintf("#%s: %v", p, err))
					continue
				}
				if err := ilog.Append(ctx, t.ID, "message", body, nil, nil, sess, db); err != nil {
					errs = append(errs, fmt.Sprintf("#%d: %v", t.ID, err))
					continue
				}
				results = append(results, fmt.Sprintf("#%d  comment added", t.ID))
			}

			var sb strings.Builder
			for _, r := range results {
				sb.WriteString(r + "\n")
			}
			if len(errs) > 0 {
				sb.WriteString("\nErrors:\n")
				for _, e := range errs {
					sb.WriteString("  " + e + "\n")
				}
			}
			return mcplib.NewToolResultText(sb.String()), nil
		},
	)

	// tkt_submit_plan
	s.AddTool(
		mcplib.NewTool("tkt_submit_plan",
			mcplib.WithDescription("Submit a plan for a ticket (required before PLANNING→IN_PROGRESS)."),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Ticket ID")),
			mcplib.WithString("body", mcplib.Required(), mcplib.Description("Plan content")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			id := req.GetString("id", "")
			if id == "" {
				return mcplib.NewToolResultError("id is required"), nil
			}
			body := req.GetString("body", "")
			if body == "" {
				return mcplib.NewToolResultError("body is required"), nil
			}

			t, err := ticket.GetByID(id, db)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			if err := ilog.Append(ctx, t.ID, "plan", body, nil, nil, sess, db); err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			return mcplib.NewToolResultText(fmt.Sprintf("#%d  plan submitted", t.ID)), nil
		},
	)

	// tkt_add_depends
	s.AddTool(
		mcplib.NewTool("tkt_add_depends",
			mcplib.WithDescription("Add dependency edges to a ticket."),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Ticket ID")),
			mcplib.WithString("on", mcplib.Required(), mcplib.Description("Comma-separated IDs this ticket depends on")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			id := req.GetString("id", "")
			if id == "" {
				return mcplib.NewToolResultError("id is required"), nil
			}
			onStr := req.GetString("on", "")
			if onStr == "" {
				return mcplib.NewToolResultError("on is required"), nil
			}

			t, err := ticket.GetByID(id, db)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			parts := strings.Split(onStr, ",")
			var depIDs []int64
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				n, err := strconv.ParseInt(p, 10, 64)
				if err != nil {
					return mcplib.NewToolResultError(fmt.Sprintf("invalid dependency ID %q", p)), nil
				}
				depIDs = append(depIDs, n)
			}

			if err := ticket.AddDependencies(t.ID, depIDs, db); err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			strs := make([]string, len(depIDs))
			for i, d := range depIDs {
				strs[i] = fmt.Sprintf("#%d", d)
			}
			return mcplib.NewToolResultText(fmt.Sprintf("#%d now depends on: %s", t.ID, strings.Join(strs, ", "))), nil
		},
	)

	// tkt_set_tier
	s.AddTool(
		mcplib.NewTool("tkt_set_tier",
			mcplib.WithDescription("Set the tier of a ticket."),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Ticket ID")),
			mcplib.WithString("tier", mcplib.Required(), mcplib.Description("Tier: critical, standard, or low")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			id := req.GetString("id", "")
			if id == "" {
				return mcplib.NewToolResultError("id is required"), nil
			}
			tier := req.GetString("tier", "")
			if tier == "" {
				return mcplib.NewToolResultError("tier is required"), nil
			}

			t, err := ticket.SetTier(id, tier, db)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			return mcplib.NewToolResultText(fmt.Sprintf("#%d  tier set to %s", t.ID, t.Tier)), nil
		},
	)

	// tkt_archive_ticket
	s.AddTool(
		mcplib.NewTool("tkt_archive_ticket",
			mcplib.WithDescription("Archive one or more VERIFIED tickets."),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Ticket ID or comma-separated IDs")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			idStr := req.GetString("id", "")
			if idStr == "" {
				return mcplib.NewToolResultError("id is required"), nil
			}

			parts := strings.Split(idStr, ",")
			var errs []string
			var results []string

			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				t, err := ticket.GetByID(p, db)
				if err != nil {
					errs = append(errs, fmt.Sprintf("#%s: %v", p, err))
					continue
				}
				fromStatus := t.Status
				if err := state.Execute(p, models.StatusArchived, "archived via mcp", sess, db, false); err != nil {
					errs = append(errs, fmt.Sprintf("#%d: %v", t.ID, err))
					continue
				}
				results = append(results, fmt.Sprintf("#%d  %s → ARCHIVED", t.ID, fromStatus))
			}

			var sb strings.Builder
			for _, r := range results {
				sb.WriteString(r + "\n")
			}
			if len(errs) > 0 {
				sb.WriteString("\nErrors:\n")
				for _, e := range errs {
					sb.WriteString("  " + e + "\n")
				}
			}
			return mcplib.NewToolResultText(sb.String()), nil
		},
	)

	// tkt_log_usage
	s.AddTool(
		mcplib.NewTool("tkt_log_usage",
			mcplib.WithDescription("Record token/tool/duration usage against a ticket."),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Ticket ID")),
			mcplib.WithNumber("tokens", mcplib.Required(), mcplib.Description("Number of tokens used (must be > 0)")),
			mcplib.WithNumber("tools", mcplib.Description("Number of tool calls (optional)")),
			mcplib.WithNumber("duration", mcplib.Description("Duration in seconds (optional)")),
			mcplib.WithString("agent", mcplib.Description("Agent role (optional, defaults to session role)")),
			mcplib.WithString("label", mcplib.Description("Free annotation (optional)")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			id := req.GetString("id", "")
			if id == "" {
				return mcplib.NewToolResultError("id is required"), nil
			}
			tokens := req.GetInt("tokens", 0)
			if tokens <= 0 {
				return mcplib.NewToolResultError("tokens must be > 0"), nil
			}
			tools := req.GetInt("tools", 0)
			durationSecs := req.GetInt("duration", 0)
			agent := req.GetString("agent", "")
			if agent == "" {
				agent = string(sess.Role)
			}
			label := req.GetString("label", "")

			t, err := ticket.GetByID(id, db)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			if err := usage.Append(ctx, t.ID, sess.ID, tokens, tools, durationSecs*1000, agent, label, db); err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}

			parts := []string{fmt.Sprintf("%d tokens", tokens)}
			if tools > 0 {
				parts = append(parts, fmt.Sprintf("%d tools", tools))
			}
			if durationSecs > 0 {
				parts = append(parts, fmt.Sprintf("%ds", durationSecs))
			}

			return mcplib.NewToolResultText(fmt.Sprintf("#%d  logged %s — %s", t.ID, strings.Join(parts, ", "), agent)), nil
		},
	)

	// tkt_update_ticket
	s.AddTool(
		mcplib.NewTool("tkt_update_ticket",
			mcplib.WithDescription("Update main_type or attention_level of an existing ticket. At least one field required."),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Ticket ID")),
			mcplib.WithString("main_type", mcplib.Description("New type label (optional, max 30 chars)")),
			mcplib.WithNumber("attention", mcplib.Description("New attention level 0–99 (optional)")),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			id := req.GetString("id", "")
			if id == "" {
				return mcplib.NewToolResultError("id is required"), nil
			}

			rawType := req.GetString("main_type", "")
			var mainType *string
			if rawType != "" {
				mainType = &rawType
			}

			rawAttn := req.GetInt("attention", -1)
			var attentionLevel *int
			if rawAttn >= 0 {
				attentionLevel = &rawAttn
			}

			if mainType == nil && attentionLevel == nil {
				return mcplib.NewToolResultError("at least one of main_type or attention must be provided"), nil
			}

			t, err := ticket.Update(id, mainType, attentionLevel, db)
			if err != nil {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			return mcplib.NewToolResultText(fmt.Sprintf("#%d updated — type=%q attention=%d", t.ID, t.MainType, t.AttentionLevel)), nil
		},
	)

	// suppress unused import warning
	_ = root
}
