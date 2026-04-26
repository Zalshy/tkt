# tkt show

Show full detail for a single ticket, including its plan, log, and token usage.

## Usage

```
tkt show <id> [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | bool | `false` | Output machine-readable JSON with ticket, log entries, usage, and dependencies |

## Notes / Behaviour

- `<id>` is the numeric ticket ID.
- Output includes: title, status, tier, type, description, plan (if submitted), full state-transition log, comments, and token usage summary.
- Dependencies and their current statuses are listed when present.
- `--json` outputs an object with `ticket`, `log_entries`, `usage`, and `dependencies`.

## Examples

```bash
tkt show 42
tkt show 42 --json
tkt --dir /path/to/project show 7
```
