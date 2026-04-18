# tkt advance

Move a ticket to the next state (or a specified target state).

## Usage

```
tkt advance <id> --note "<note>" [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--note` | string | `""` | Required note for the transition (must be non-empty) |
| `--to` | string | `""` | Target state: `TODO`, `PLANNING`, `IN_PROGRESS`, `DONE`, `VERIFIED`, `CANCELED`; default: natural next state |
| `--force` | bool | `false` | Override role/isolation checks (violation will be recorded in the log) |

## Notes / Behaviour

- `--note` is required on every call; an empty note causes a hard error.
- Ticket lifecycle: `TODO → PLANNING → IN_PROGRESS → DONE → VERIFIED → ARCHIVED`
- `PLANNING → IN_PROGRESS` requires a plan to be submitted first (`tkt plan`).
- `PLANNING → IN_PROGRESS` requires a different session than the one that wrote the plan (isolation rule).
- `DONE → VERIFIED` requires architect role.
- `--force` bypasses role and isolation checks but permanently records a violation in the audit log.
- `--to` can target any valid state; use with care since skipping states may violate workflow rules.
- Tickets can be moved to `CANCELED` from any state; `CANCELED → TODO` is the only reverse path without `--force`.

## Examples

```bash
tkt advance 5 --note "picking this up"
tkt advance 5 --note "implementation complete"
tkt advance 5 --to CANCELED --note "out of scope"
tkt advance 5 --to IN_PROGRESS --note "plan approved" --force
```
