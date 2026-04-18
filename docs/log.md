# tkt log

Record token and tool usage against a ticket.

## Usage

```
tkt log <id> --tokens <n> [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--tokens` | int | `0` | Number of tokens used (required, must be > 0) |
| `--tools` | int | `0` | Number of tool calls (optional) |
| `--duration` | int | `0` | Duration in seconds (optional) |
| `--agent` | string | `""` | Agent role (optional) |
| `--label` | string | `""` | Free annotation (optional) |

## Notes / Behaviour

- `--tokens` is required and must be greater than 0.
- All other flags are optional supplementary metadata.
- Log entries are visible in `tkt show` output under the token usage summary.
- Multiple log entries can be added to a single ticket over its lifetime.

## Examples

```bash
tkt log 5 --tokens 4200
tkt log 5 --tokens 8500 --tools 23 --duration 180 --agent implementer --label "refactor pass"
```
