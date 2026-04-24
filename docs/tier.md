# tkt tier

Set the tier of a ticket.

## Usage

```
tkt tier <id> <tier>
```

## Flags

No flags beyond the global `--dir`.

## Notes / Behaviour

- `<tier>` is a positional argument; valid values are `critical`, `standard`, and `low`.
- Tier affects scheduling priority in `tkt batch`: critical tickets surface first.
- The default tier for new tickets is `standard` (set at creation with `tkt new --tier`).

## Examples

```bash
tkt tier 5 critical
tkt tier 12 low
tkt tier 8 standard
```
