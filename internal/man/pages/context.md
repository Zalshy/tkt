# tkt context

Manage project context entries — short, imperative notes about non-obvious caveats.

## Usage

```
tkt context add "<title>" "<body>"
tkt context readall
tkt context read <id>
tkt context update <id> "<title>" "<body>"
tkt context delete <id>
```

## Flags

No flags beyond the global `--dir`. All arguments are positional.

## Subcommands

| Subcommand | Arguments | Description |
|------------|-----------|-------------|
| `add` | `"<title>"` `"<body>"` | Create a new context entry |
| `readall` | — | Print all context entries |
| `read` | `<id>` | Print a single context entry |
| `update` | `<id>` `"<title>"` `"<body>"` | Replace title and body of an existing entry |
| `delete` | `<id>` | Remove a context entry |

## Notes / Behaviour

- `add` takes exactly two positional string arguments: title and body. There are no flags for content.
- Context entries are meant to be short (≤ 3 lines), imperative, one caveat per entry.
- Run `tkt context readall` at the start of every session to pick up caveats that would not be obvious from reading source code alone.
- IDs are auto-assigned integers; use `tkt context readall` to find them.

## Examples

```bash
tkt context add "Init uses value receiver" "Init() uses a value receiver — mutations inside it are silently discarded."
tkt context readall
tkt context read 2
tkt context update 2 "Init uses value receiver" "Updated caveat text."
tkt context delete 2
```
