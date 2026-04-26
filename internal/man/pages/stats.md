# tkt stats

Show project statistics and activity metrics.

## Usage

```
tkt stats [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--since` | string | `""` | Include ticket activity on or after `YYYY-MM-DD` |
| `--until` | string | `""` | Include ticket activity on or before `YYYY-MM-DD` |
| `--window` | string | `""` | Include ticket activity in the last duration, e.g. `24h`, `7d`, `30d` |
| `--status` | string | `""` | Filter by status (`TODO`, `PLANNING`, `IN_PROGRESS`, `DONE`, `VERIFIED`, `CANCELED`, `ARCHIVED`) |
| `--tier` | string | `""` | Filter by tier (`critical`, `standard`, `low`) |
| `--type` | string | `""` | Filter by ticket type (`main_type`) |
| `--created-by` | string | `""` | Filter by creator session name |
| `--verified` | bool | `false` | Include `VERIFIED` tickets when using explicit filters |
| `--archived` | bool | `false` | Include `ARCHIVED` tickets when using explicit filters |
| `--json` | bool | `false` | Output machine-readable JSON with `default_scope` and full report |

## Default scope

With no flags, `tkt stats` analyzes activity from the last 24 hours across all ticket types and statuses.

The output includes a scope line:

```text
Scope: default last 24 hours, all ticket types and statuses
```

## Activity time semantics

`--since` and `--until` filter by ticket activity time, not ticket creation time.

Activity includes:

- ticket `updated_at`
- ticket log entries (`ticket_log.created_at`)
- usage rows (`ticket_usage.created_at`)

Metric-specific sections also use activity time:

- cycle/lead time use transition log time
- throughput uses `DONE` transition time
- resource burn uses usage row time

## Output sections

`tkt stats` renders:

- `Overview` — ticket counts plus summary cycle/lead/resource values
- `Cycle Time` — completion duration summary and trend
- `Throughput` — completed tickets by day/week
- `Resource Burn` — tokens, tools, duration, token trend, and type breakdown
- `Distribution` — counts by status, tier, and type

## Notes / Behaviour

- If no flags are passed, `VERIFIED` and `ARCHIVED` tickets are included because the default is an all-status activity report.
- When explicit filters are passed, `VERIFIED` tickets are hidden unless `--verified` or `--status VERIFIED` is used.
- When explicit filters are passed, `ARCHIVED` tickets are hidden unless `--archived` or `--status ARCHIVED` is used.
- `--status ARCHIVED` works even without `--archived`.
- Dates must use `YYYY-MM-DD`.
- `--window` uses activity time since now minus duration. It supports Go durations like `24h` plus day suffixes like `7d` and `30d`.
- `--window` cannot be combined with `--since` or `--until`.
- `--since` must be before or equal to `--until`.

## Examples

```bash
# Last 24 hours of all ticket activity
tkt stats

# Activity since a date
tkt stats --since 2026-04-01

# Activity in a date range
tkt stats --since 2026-04-01 --until 2026-04-25

# Activity in a relative window
tkt stats --window 24h
tkt stats --window 7d

# DONE tickets only
tkt stats --status DONE

# Include verified tickets when filtering by type
tkt stats --type feature --verified

# Include archived tickets in filtered stats
tkt stats --archived --verified

# Machine-readable report
tkt stats --json
```
