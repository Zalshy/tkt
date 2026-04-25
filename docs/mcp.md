# tkt mcp

Start the tkt MCP server over stdio, exposing tkt operations as MCP tools.

## Usage

```
tkt mcp [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--role` | string | `implementer` | Session role for write tools (`architect` or `implementer`) |
| `--readonly` | bool | `false` | Register read-only tools only |

## Notes / Behaviour

- The MCP server communicates over stdio; it is designed to be launched by an MCP-compatible host (Claude Code, Claude Desktop, Cursor, Zed, etc.).
- `--role` determines which write operations are permitted; defaults to `implementer`.
- `--readonly` suppresses all write/admin tools, exposing only read operations.
- The server runs until the host closes the connection.

## Tools

### Read tools

| Tool | Description |
|------|-------------|
| `tkt_list_tickets` | List tickets with status/ready/archive/verified filters |
| `tkt_show_ticket` | Show ticket detail including log, plan, dependencies, and usage |
| `tkt_search_tickets` | Search tickets by title/description |
| `tkt_batch` | Show dependency-unblocked executable phases |
| `tkt_stats` | Show activity-based project statistics |
| `tkt_list_context` | List project context entries |
| `tkt_list_docs` | List project documents |
| `tkt_read_doc` | Read a project document |
| `tkt_list_roles` | List roles |

### Write/admin tools

These are available unless `--readonly` is set:

| Tool | Description |
|------|-------------|
| `tkt_new_ticket` | Create a ticket |
| `tkt_advance_ticket` | Advance ticket state |
| `tkt_add_comment` | Add a ticket comment |
| `tkt_submit_plan` | Submit a ticket plan |
| `tkt_add_depends` | Add dependencies |
| `tkt_set_tier` | Set ticket tier |
| `tkt_archive_ticket` | Archive a verified ticket |
| `tkt_log_usage` | Record token/tool/duration usage |
| `tkt_update_ticket` | Update ticket type/attention |
| `tkt_add_context`, `tkt_update_context`, `tkt_delete_context` | Manage context entries |
| `tkt_add_doc`, `tkt_archive_doc` | Manage documents |
| `tkt_create_role`, `tkt_delete_role` | Manage roles |
| `tkt_cleanup` | Run cleanup |

### `tkt_stats` parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `since` | string | Include ticket activity on or after `YYYY-MM-DD` |
| `until` | string | Include ticket activity on or before `YYYY-MM-DD` |
| `status` | string | Filter by ticket status |
| `tier` | string | Filter by tier (`critical`, `standard`, `low`) |
| `type` | string | Filter by ticket type (`main_type`) |
| `created_by` | string | Filter by creator session name |
| `verified` | bool | Include `VERIFIED` tickets when using explicit filters |
| `archived` | bool | Include `ARCHIVED` tickets when using explicit filters |

With no parameters, `tkt_stats` uses the same default as `tkt stats`: last 24 hours of activity across all ticket types and statuses.

## Examples

```bash
# Typical host-managed invocation (configured in MCP host settings):
tkt mcp --role architect

# Read-only mode for untrusted contexts:
tkt mcp --readonly
```

### Example MCP host configuration

```json
{
  "mcpServers": {
    "tkt": {
      "command": "tkt",
      "args": ["mcp", "--role", "implementer"]
    }
  }
}
```
