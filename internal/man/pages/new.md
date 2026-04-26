# tkt new

Create a new ticket.

## Usage

```
tkt new "<title>" [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--description` | string | `""` | Ticket description |
| `--description-file` | string | `""` | Read ticket description from file |
| `--description-stdin` | bool | `false` | Read ticket description from stdin |
| `--after` | string | `""` | Comma-separated dependency ticket IDs (e.g. `5,7`) |
| `--tier` | string | `standard` | Ticket tier: `critical`, `standard`, or `low` |
| `--type` | string | `""` | Ticket type label (e.g. `feature`, `bugfix`, `refactor`) |
| `--attention` | int | `0` | Attention level 0-99 (0 = unset) |

## Notes / Behaviour

- The title argument is required and must be quoted if it contains spaces.
- Use exactly one description source: `--description`, `--description-file`, or `--description-stdin`.
- Prefer file/stdin for multiline markdown so backticks, `$()`, and quotes are preserved by the shell.
- `--after` records the listed ticket IDs as dependencies; the new ticket will not appear as ready until those dependencies reach VERIFIED.
- `--tier` affects scheduling priority in `tkt batch`.
- `--attention` is a 0-99 integer hint for urgency; 0 means unset.
- New tickets start in TODO state.

## Examples

```bash
tkt new "Fix login bug"
tkt new "Add OAuth" --tier critical --type feature
tkt new "Refactor cache" --after 12,15 --description-file spec.md
tkt new "Markdown-heavy ticket" --description-stdin < ticket.md
tkt new "Hotfix deploy" --tier critical --attention 90
```
