# tkt doc

Manage long-form project documents (design notes, ADRs, post-mortems, etc.).

## Usage

```
tkt doc add <slug> [flags]
tkt doc list [flags]
tkt doc read <slug>
tkt doc archive <slug>
```

## Flags

### doc add

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--body` | string | `""` | Document content for non-interactive use (skips `$EDITOR` when set) |
| `--stdin` | bool | `false` | Read content from stdin |
| `--file` | string | `""` | Read content from file at path |

### doc list

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--archived` | bool | `false` | List archived documents |

### doc read

No flags beyond the global `--dir`.

### doc archive

No flags beyond the global `--dir`.

## Notes / Behaviour

- `doc add` requires a positional `<slug>` argument (e.g. `tkt doc add adr-001`). The slug is the document identifier.
- For `doc add`: `--body`, `--stdin`, and `--file` are mutually exclusive; providing more than one is an error.
- When none of `--body`, `--stdin`, or `--file` is set, `doc add` opens `$EDITOR` — this hangs in non-interactive contexts.
- `--archived` belongs only to `doc list`; it is not a flag on other subcommands.
- `doc archive` moves a document to an archived state; it will only appear in `doc list --archived` afterwards.

## Examples

```bash
tkt doc add adr-001 --body "Decision: use PostgreSQL for persistence."
echo "content" | tkt doc add adr-002 --stdin
tkt doc add design-notes --file ./notes.md
tkt doc list
tkt doc list --archived
tkt doc read adr-001
tkt doc archive adr-001
```
