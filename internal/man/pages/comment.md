# tkt comment

Add a comment to one or more tickets without changing state.

## Usage

```
tkt comment <id[,id...]> ["<body>"] [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--body` | string | `""` | Comment body |
| `--body-file` | string | `""` | Read comment body from file |
| `--body-stdin` | bool | `false` | Read comment body from stdin |

## Notes / Behaviour

- Positional body remains supported for compatibility: `tkt comment 7 "message"`.
- Use exactly one body source: positional body, `--body`, `--body-file`, or `--body-stdin`.
- Prefer file/stdin for multiline markdown so backticks, `$()`, and quotes are preserved by the shell.
- Comments are appended to the ticket log and are visible in `tkt show` output.
- State is not changed; use `tkt advance` to transition state.
- Multiple comma-separated IDs receive the same comment.

## Examples

```bash
tkt comment 7 "Blocked on upstream API change in ticket 3"
tkt comment 12 --body "Found edge case with null user"
tkt comment 12 --body-file notes.md
tkt comment 12 --body-stdin < notes.md
tkt comment 12,13 --body-file review-note.md
```
