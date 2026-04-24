# tkt batch

Show the next executable phases — groups of tickets that can be worked on in parallel.

## Usage

```
tkt batch [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--n` | int | `6` | Number of phases to display |

## Notes / Behaviour

- A phase is a set of tickets whose dependencies are all resolved (VERIFIED or absent).
- Tickets are grouped by dependency depth; phase 1 is immediately executable.
- Within a phase, tickets are ordered by tier (critical first) then attention level.
- `--n` controls how many phases ahead to show; increase when planning long-horizon work.

## Examples

```bash
tkt batch
tkt batch --n 3
tkt batch --n 10
```
