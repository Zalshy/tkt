# tkt update

Update ticket metadata (type label and attention level).

## Usage

```
tkt update <id> [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-t`, `--type` | string | `""` | Ticket type label (e.g. `feature`, `bugfix`, `refactor`) |
| `-a`, `--attention` | int | `0` | Attention level 0-99 (0 = unset) |

## Notes / Behaviour

- At least one of `--type` or `--attention` should be provided; calling with neither has no effect.
- `--attention` of `0` clears the attention level (unset).
- Type labels are free-form strings; no validation is performed.
- To update tier, use `tkt tier <id> <tier>` instead.

## Examples

```bash
tkt update 5 --type bugfix
tkt update 5 -a 80
tkt update 5 -t feature -a 50
```
