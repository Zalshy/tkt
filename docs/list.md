# tkt list

List tickets with optional filters and sort order.

## Usage

```
tkt list [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--status` | string | `""` | Filter by status: `TODO`, `PLANNING`, `IN_PROGRESS`, `DONE`, `VERIFIED`, `CANCELED`, `ARCHIVED` |
| `--limit` | int | `10` | Maximum number of tickets to show |
| `--all` | bool | `false` | Show all tickets including CANCELED and soft-deleted |
| `--verified` | bool | `false` | Include VERIFIED tickets |
| `--archived` | bool | `false` | Include ARCHIVED tickets |
| `--sort` | string | `updated` | Sort order: `updated` or `id` |
| `--ready` | bool | `false` | Show only tickets with no unresolved dependencies |

## Notes / Behaviour

- Default output omits CANCELED, VERIFIED, and ARCHIVED tickets; use the relevant flag to include them.
- `--limit` defaults to 10; always pass `--all` (or a large `--limit`) when you need to see every ticket.
- `--all` overrides `--limit` and includes every ticket regardless of status.
- `--ready` and `--status` can be combined to narrow results further.
- Sort by `id` lists tickets in creation order; `updated` (default) lists most-recently-modified first.

## Examples

```bash
tkt list
tkt list --all
tkt list --status IN_PROGRESS
tkt list --ready --sort id
tkt list --verified --archived --limit 50
```
