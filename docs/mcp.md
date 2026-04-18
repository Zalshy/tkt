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
- `--readonly` suppresses all write tools, exposing only read operations (list, show, search, context readall, etc.).
- The server runs until the host closes the connection.

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
