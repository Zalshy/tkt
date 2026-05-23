# tkt session

Create a new role session with an auto-generated name, show the current session, or end the current session.

## Usage

```
tkt session [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--role` | string | `""` | Role for the new session (e.g. `architect` or `implementer`) |
| `--end` | bool | `false` | Mark the current session as expired |
| `--name` | string | `""` | Advanced: explicit session name (requires `--role`); lowercase alphanumeric + hyphens, max 32 chars |

## Notes / Behaviour

- `--role` and `--end` are mutually exclusive; providing both is an error.
- When called with no flags, prints the current active session.
- Normal usage is `tkt session --role <role>`; tkt generates the session name automatically.
- `--name` is optional advanced usage for deterministic automation, debugging, or rare cases that need a specific session name.
- `--name` requires `--role`; it cannot be used with `--end`.
- Explicit session names must be lowercase alphanumeric with hyphens, max 32 characters.
- Built-in roles are `architect` and `implementer`; custom roles can be created with `tkt role create`.
- Only one session is active at a time per project directory.

## Examples

```bash
tkt session                          # show active session
tkt session --role architect         # start a new architect session with generated name
tkt session --role implementer       # start a new implementer session with generated name
tkt session --end                    # expire the current session
```
