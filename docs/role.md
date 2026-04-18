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
- Custom role names follow the same constraints as session `--name`: lowercase alphanumeric + hyphens, max 32 chars.
- Custom roles inherit all permissions of their base role.
- Built-in roles `architect` and `implementer` cannot be deleted.
- A session started with a custom role is treated identically to a session with the base role for all isolation and permission checks.

## Examples

```bash
tkt role create security-expert --like architect
tkt role create ci-bot --like implementer
tkt role list
tkt role delete security-expert
```
