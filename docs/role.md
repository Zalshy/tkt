# tkt role

Manage custom roles mapped to a built-in base role.

## Usage

```
tkt role create <name> --like <architect|implementer>
tkt role list
tkt role delete <name>
```

## Flags

### role create

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--like` | string | `""` | Base role to inherit permissions from: `architect` or `implementer` (required) |

### role list

No flags beyond the global `--dir`.

### role delete

No flags beyond the global `--dir`.

## Notes / Behaviour

- `--like` is required for `role create`; omitting it is an error.
- Custom role names must match `[a-z][a-z0-9_]*` (lowercase letter, then lowercase letters/digits/underscores — no hyphens), 2–64 characters.
- Custom roles inherit all permissions of their base role.
- Built-in roles `architect` and `implementer` cannot be deleted.
- A session started with a custom role is treated identically to a session with the base role for all isolation and permission checks.

## Examples

```bash
tkt role create security_expert --like architect
tkt role create ci_bot --like implementer
tkt role list
tkt role delete security_expert
```
