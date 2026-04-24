# tkt comment

Add a comment to a ticket without changing its state.

## Usage

```
tkt comment <id> "<message>"
```

## Flags

No flags beyond the global `--dir`.

## Notes / Behaviour

- Comments are appended to the ticket log and are visible in `tkt show` output.
- State is not changed; use `tkt advance` to transition state.
- Use comments to communicate blockers, discoveries, or decisions without advancing the workflow.

## Examples

```bash
tkt comment 7 "Blocked on upstream API change in ticket 3"
tkt comment 12 "Found edge case with null user — handling in next commit"
```
