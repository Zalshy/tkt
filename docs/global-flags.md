# tkt global flags

Persistent flags available on every tkt command.

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dir` | string | `""` | Override project root directory (default: cwd or nearest `.tkt/` parent) |
| `--session` | string | `""` | Resolve the acting session by ID or name directly from the database, bypassing the `.tkt/session` file pointer for this invocation |

## Notes / Behaviour

- `--dir` must point to a directory that contains a `.tkt/` subdirectory, or be the directory where `tkt init` will be run.
- When `--dir` is omitted, tkt walks up from the current working directory looking for the nearest `.tkt/` parent.
- Useful in scripts or CI where the working directory is not the project root.
- `--session` accepts either a session's ULID or its generated name.
- `.tkt/session` is a single file pointer per project directory. When an orchestrator and the subagents it spawns all call tkt in the same directory, concurrent `tkt session --role X` calls race and silently overwrite each other's effective identity. `--session` lets a caller assert its identity explicitly per call instead of relying on that shared file state.
- If the value matches no session, or matches an expired session, the command fails with a clear error — it never silently falls back to the file pointer.
- When `--session` is omitted, behaviour is unchanged: the existing file-pointer-based active session is used.
- `--session` is distinct from `--as` (the orchestrator-only delegation flag on `tkt advance`): `--session` controls *which session resolves you*, bypassing the file; `--as` lets an already-resolved orchestrator session *act as* another role. The two can be combined.

## Examples

```bash
tkt --dir /path/to/project list
tkt --dir /path/to/project advance 5 --note "done"
tkt --session architect-7f2a advance 12 --note "approved plan"     # resolve by session name
tkt --session 01HXYZ... comment 12 "note from a specific session"  # resolve by session ULID
```
