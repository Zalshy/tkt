# tkt search

Full-text search across ticket titles and descriptions.

## Usage

```
tkt search <query> [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--title` | bool | `false` | Restrict search to title only |
| `--all` | bool | `false` | Include CANCELED and ARCHIVED tickets in results |
| `--status` | string | `""` | Filter results to a specific status |

## Notes / Behaviour

- `<query>` is a required positional argument; quote multi-word queries.
- By default searches both title and description; `--title` narrows to title only.
- By default excludes CANCELED and ARCHIVED tickets; `--all` includes them.
- `--status` and `--all` can be combined.

## Examples

```bash
tkt search "login bug"
tkt search oauth --title
tkt search cache --status IN_PROGRESS
tkt search refactor --all
```
