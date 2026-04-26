# tkt advance

Move a ticket to the next state (or a specified target state).

## Usage

```
tkt advance <id> --note "<note>" [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--note` | string | `""` | Required note for real transitions (must be non-empty) |
| `--note-file` | string | `""` | Read transition note from file |
| `--note-stdin` | bool | `false` | Read transition note from stdin |
| `--to` | string | `""` | Target state: `TODO`, `PLANNING`, `IN_PROGRESS`, `DONE`, `VERIFIED`, `CANCELED`, `ARCHIVED`; default: natural next state |
| `--force` | bool | `false` | Override role/isolation checks (violation will be recorded in the log for real transitions) |
| `--dry-run` | bool | `false` | Check transition without changing state or writing a log row; note is optional |
| `--explain` | bool | `false` | Explain why transition is allowed or blocked without changing state or writing a log row; note is optional |

## Notes / Behaviour

- `--note` is required on real transitions; `--dry-run` and `--explain` do not require a note because they do not write a log row.
- Use exactly one note source: `--note`, `--note-file`, or `--note-stdin`.
- Prefer file/stdin for multiline markdown so backticks, `$()`, and quotes are preserved by the shell.
- Ticket lifecycle: `TODO → PLANNING → IN_PROGRESS → DONE → VERIFIED → ARCHIVED`
- `PLANNING → IN_PROGRESS` requires a plan to be submitted first (`tkt plan`).
- `PLANNING → IN_PROGRESS` requires a different session than the one that wrote the plan (isolation rule).
- `DONE → VERIFIED` requires architect role.
- `--dry-run` prints whether the transition would advance or be blocked; it never changes ticket status and never writes a transition log row.
- `--explain` prints current state, target state, allowed/blocked result, plan guard details when relevant, and manual hints (`tkt man advance`, `tkt man state-machine`).
- `--dry-run` and `--explain` cannot be used together.
- `--force` bypasses role and isolation checks but permanently records a violation in the audit log for real transitions. In dry-run/explain mode, it shows force effect without writing.
- `--to` can target any valid state; use with care since skipping states may violate workflow rules.
- Tickets can be moved to `CANCELED` from any state; `CANCELED → TODO` is the only reverse path without `--force`.

## Examples

```bash
tkt advance 5 --note "picking this up"
tkt advance 5 --note "implementation complete"
tkt advance 5 --note-file transition-note.md
tkt advance 5 --note-stdin < transition-note.md
tkt advance 5 --to CANCELED --note "out of scope"
tkt advance 5 --dry-run
tkt advance 5 --explain --to IN_PROGRESS
tkt advance 5 --to IN_PROGRESS --note "plan approved" --force
```
