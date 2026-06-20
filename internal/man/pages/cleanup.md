# tkt cleanup

Expire stale sessions (inactive for more than 4 hours) and run related maintenance.

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
- Active sessions with `last_active` older than 4 hours are expired (soft-deleted); expired sessions can no longer be used to advance tickets.
- Sessions already expired for more than 7 days are hard-deleted (purged) to free name slots; this purge only runs on a real (non-dry-run) invocation and is not reflected in the reported count, which counts newly-expired sessions only.
- Does not delete tickets; use `tkt advance <id> --to CANCELED` to cancel unwanted tickets.
- Prints "Nothing to clean up." when there are no stale sessions.

## Examples

```bash
tkt cleanup --dry-run
tkt cleanup
```
