# tkt show

Show full detail for a single ticket, including its plan, log, and token usage.

## Usage

```
tkt show <id>
```

## Flags

No flags beyond the global `--dir`.

## Notes / Behaviour

- `<id>` is the numeric ticket ID.
- Output includes: title, status, tier, type, description, plan (if submitted), full state-transition log, comments, and token usage summary.
- Dependencies and their current statuses are listed when present.

## Examples

```bash
tkt show 42
tkt --dir /path/to/project show 7
```
