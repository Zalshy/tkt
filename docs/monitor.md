# tkt monitor

Live terminal UI for watching ticket activity in real time.

```
tkt monitor        # minimal mode (default)
tkt monitor side   # side panel
```

For full details run `tkt docs monitor`.

---

## Minimal mode (default)

A read-only 3-column kanban (TODO / PLANNING / DONE) that auto-refreshes every 3 seconds.
Navigate columns and cards with the arrow keys, open a ticket with `enter`, search with `/`.

## Side mode

A companion stats panel designed to run alongside minimal in a split pane, but works
standalone too. Shows live ticket stats, active sessions, a ticket activity feed, token
burn totals, and a velocity sparkline. Forced transitions are marked with ⚠ in the feed.
Quit with `q`.
