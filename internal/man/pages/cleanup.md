# tkt cleanup

Remove stale or orphaned data from the project store.

## Usage

```
tkt cleanup [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dry-run` | bool | `false` | Print what would be affected without writing any changes |

## Notes / Behaviour

- Always run with `--dry-run` first to inspect what will be removed.
- Cleanup targets orphaned references, stale session records, and similar internal inconsistencies.
- Active sessions with `last_active` older than 4 hours are expired.
- Expired sessions older than 7 days are purged.
- Does not delete tickets; use `tkt advance <id> --to CANCELED` to cancel unwanted tickets.

## Examples

```bash
tkt cleanup --dry-run
tkt cleanup
```
