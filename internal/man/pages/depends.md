# tkt depends

Add or remove ticket dependencies.

## Usage

```
tkt depends <id> [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--on` | string | `""` | Comma-separated list of ticket IDs this ticket depends on |
| `--remove` | string | `""` | Ticket ID of the dependency to remove |

## Notes / Behaviour

- Exactly one of `--on` or `--remove` must be provided; using both is an error.
- A ticket with unresolved dependencies will not appear in `tkt list --ready` output.
- Dependencies are resolved when the depended-on ticket reaches VERIFIED.
- `--on` accepts multiple IDs separated by commas with no spaces (e.g. `5,7,12`).

## Examples

```bash
tkt depends 10 --on 5,7
tkt depends 10 --remove 5
```
