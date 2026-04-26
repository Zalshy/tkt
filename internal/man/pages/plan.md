# tkt plan

Write or revise the plan for a ticket.

## Usage

```
tkt plan <id> [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--body` | string | `""` | Plan body for non-interactive use (skips `$EDITOR` when set) |
| `--stdin` | bool | `false` | Read plan body from stdin |
| `--file` | string | `""` | Read plan body from file at path |

## Notes / Behaviour

- `--body`, `--stdin`, and `--file` are mutually exclusive; providing more than one is an error.
- When none of `--body`, `--stdin`, or `--file` is set, tkt opens `$EDITOR` — this will hang in non-interactive (agent) contexts.
- Always use `--body` in scripts and agent workflows.
- A plan can be revised while the ticket is in PLANNING state; once the ticket advances past PLANNING the plan is locked.
- `PLANNING → IN_PROGRESS` will hard-error if no plan has been submitted.

## Examples

```bash
tkt plan 12 --body "Step 1: refactor X. Step 2: add tests."
echo "My plan" | tkt plan 12 --stdin
tkt plan 12 --file ./plan.txt
```
