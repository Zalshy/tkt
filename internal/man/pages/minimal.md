# minimal

Compact operating guide for humans and LLM agents.

Read this first, then read specific pages with `tkt man <page>`.

Critical rules:
- Start a session before writes: `tkt session --role architect` or `tkt session --role implementer`.
- Multiple agents in the same project dir share one `.tkt/session` file pointer and can race. Use `--session <id-or-name>` on any command to resolve your identity explicitly instead of relying on the file pointer.
- Statuses are `TODO`, `PLANNING`, `IN_PROGRESS`, `DONE`, `VERIFIED`, `CANCELED`, `ARCHIVED`.
- No status named `PLANNED`.
- `tkt list` shows only 10 active non-verified tickets by default; use `tkt list --all` when auditing.
- `tkt plan` opens `$EDITOR` unless `--body`, `--stdin`, or `--file` is supplied.
- Prefer file/stdin flags for multiline markdown: `new --description-file`, `comment --body-file`, `advance --note-file`, `plan --file`.
- `tkt advance` requires a non-empty note for real transitions. Use `--note`, `--note-file`, or `--note-stdin`.
- Use `tkt advance --dry-run` or `--explain` to preflight transitions without writing status or log rows.
- Use `--json` on `list`, `show`, `batch`, and `stats` for machine-readable output.
- Use `tkt stats --window 24h` or `--window 7d` for relative activity windows.
- `PLANNING -> IN_PROGRESS` requires a submitted plan and an architect-effective session different from the plan submitter.
- `DONE -> VERIFIED` requires an architect-effective session different from the submitter.
- Use `--force` only to record an explicit violation.

Useful pages: `workflow`, `state-machine`, `new`, `plan`, `advance`, `list`, `show`, `stats`, `mcp`.
