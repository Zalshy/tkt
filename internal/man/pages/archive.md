# tkt archive

Archive a VERIFIED ticket, moving it out of the active view.

## Usage

```
tkt archive <id>
```

## Flags

No flags beyond the global `--dir`.

## Notes / Behaviour

- Only VERIFIED tickets can be archived; attempting to archive a ticket in another state is an error.
- Archived tickets are hidden from `tkt list` by default; use `tkt list --archived` to include them.
- Archiving is not deletion — data is preserved and still searchable via `tkt search --all`.
- The lifecycle path is: `... → VERIFIED → ARCHIVED`.

## Examples

```bash
tkt archive 42
tkt --dir /path/to/project archive 7
```
