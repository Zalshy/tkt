# tkt session

Create a new role session, show the current session, or end the current session.

## Usage

```
tkt session [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--role` | string | `""` | Role for the new session (e.g. `architect` or `implementer`) |
| `--end` | bool | `false` | Mark the current session as expired |
| `--name` | string | `""` | Explicit session name (requires `--role`); lowercase alphanumeric + hyphens, max 32 chars |

## Notes / Behaviour

- `--role` and `--end` are mutually exclusive; providing both is an error.
- When called with no flags, prints the current active session.
- `--name` requires `--role`; it cannot be used with `--end`.
- Session names must be lowercase alphanumeric with hyphens, max 32 characters.
- Built-in roles are `architect` and `implementer`; custom roles can be created with `tkt role create`.
- Only one session is active at a time per project directory.

## Examples

```bash
tkt session                          # show active session
tkt session --role architect         # start a new architect session
tkt session --role implementer --name "impl-feature-x"
tkt session --end                    # expire the current session
```
