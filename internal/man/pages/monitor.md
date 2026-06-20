# tkt monitor

Live terminal UI for watching ticket activity in real time.

```
tkt monitor        # minimal mode (default)
tkt monitor side   # side panel
```

For full details run `tkt man monitor`.

---

## Minimal mode (default)

A 3-column kanban (TODO / PLANNING / DONE) that auto-refreshes every 3 seconds (configurable via
project config). Navigate columns and cards with the arrow keys (or `h`/`j`/`k`/`l`), open a
ticket with `enter`, search with `/`. The one non-read-only action is `X`, which bulk-archives
VERIFIED tickets beyond a configured keep-window, behind a confirmation modal — it cannot change
ticket content or move a ticket to any state other than ARCHIVED from VERIFIED.

## Side mode

A companion stats panel designed to run alongside minimal in a split pane, but works
standalone too. Shows live ticket stats, active sessions, a ticket activity feed, token
burn totals, and a velocity sparkline. Forced transitions are marked with ⚠ in the feed.
Quit with `q` or Ctrl-C.
