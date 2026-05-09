# tkt monitor

Live terminal UI for watching ticket activity in real time.

## Usage

```
tkt monitor [mode]
```

`mode` is optional. Accepted values: `minimal` (default), `side`.

```bash
tkt monitor          # opens minimal mode
tkt monitor minimal  # same as above, explicit
tkt monitor side     # opens the companion side panel
```

## Flags

No flags beyond the global `--dir`.

---

## Minimal mode (default)

The board. A 3-column kanban that refreshes every 5 seconds. No ambient info — just tickets.

**Columns:**

| Column | Statuses shown |
|--------|---------------|
| TODO | TODO |
| PLANNING | PLANNING, IN_PROGRESS |
| DONE | DONE, VERIFIED |

CANCELED and ARCHIVED tickets are not shown.

**Layout:**

```
┌─────────────────────────────────────────────────────┐
│  tkt                                                │
├──────────────┬──────────────────┬───────────────────┤
│   TODO       │    PLANNING      │      DONE         │
│──────────────│──────────────────│───────────────────│
│  cards...    │  cards...        │  cards...         │
├──────────────┴──────────────────┴───────────────────┤
│  ← → navigate   / search   ? help   q quit          │
└─────────────────────────────────────────────────────┘
```

**Footer:** key hints only. No session counts, no stats.

**Keyboard shortcuts:**

| Key | Action |
|-----|--------|
| `←` `→` / `h` `l` | Switch column |
| `↑` `↓` / `j` `k` | Navigate cards |
| `enter` | Open ticket detail |
| `/` | Fuzzy search |
| `X` | Bulk-archive VERIFIED tickets |
| `?` | Toggle help |
| `q` / `Ctrl+C` | Quit |

---

## Side mode

A companion panel with no board. Designed to run alongside `tkt monitor minimal` in a split
terminal pane, but works standalone too. Shows live stats, a change feed, and session activity.

**Layout:**

```
┌─────────────────────────────────────────────────────┐
│  tkt side                               14:32:07   │
├─────────────────────────────────────────────────────┤
│  STATS                                              │
│                                                     │
│  by status                                          │
│  TODO        ████████░░░░░░░░  12                   │
│  PLANNING    ████░░░░░░░░░░░░   4                   │
│  IN PROGRESS ██░░░░░░░░░░░░░░   2                   │
│  DONE        ████████░░░░░░░░   8                   │
│  VERIFIED    ███░░░░░░░░░░░░░   4                   │
│                                                     │
│  by attention level    │  by main_type              │
│  critical   3          │  feature   12              │
│  high       8          │  bug        4              │
│  normal    15          │  chore      2              │
│  low        4          │  docs       2              │
├─────────────────────────────────────────────────────┤
│  TICKET CHANGES                                     │
│ ▶ alice-impl   · #T005 → IN_PROGRESS      2s       │
│   bob-arch     · #T008 → DONE             3m       │
│   alice-arch   · #T003 → PLANNING         1h       │
│   bob-impl     · #T001 → VERIFIED        14h       │
├─────────────────────────────────────────────────────┤
│  SESSIONS                                           │
│  alice-arch    started   14:28                      │
│  bob-impl      started   14:15                      │
│                                                     │
│  🧠 arch: 1   ⚙️  impl: 2                           │
└─────────────────────────────────────────────────────┘
```

### Header

Wordmark + live clock (`HH:MM:SS`) pinned to the top right. Clock updates every second
independently of the 5-second poll cycle.

### Stats

Three breakdowns, all excluding CANCELED and ARCHIVED tickets.

- **By status** — ticket counts across TODO, PLANNING, IN_PROGRESS, DONE, VERIFIED.
  Rendered as graphical bars using lipgloss. Exact visual form (bars, distribution, labels)
  determined at implementation time based on lipgloss capabilities.
- **By attention level** — ticket counts grouped by attention level field.
- **By main_type** — ticket counts grouped by main_type field.

The attention level and main_type breakdowns share a two-column row to preserve vertical space.

### Ticket changes feed

An htop-style scrollable table showing ticket state transitions, newest first.

```
TICKET CHANGES
▶ alice-impl   · #T005 → IN_PROGRESS      2s
  bob-arch     · #T008 → DONE             3m
  alice-arch   · #T003 → PLANNING         1h
  bob-impl     · #T001 → VERIFIED        14h
```

- Sourced from `ticket_log WHERE kind = 'transition'`, ordered `created_at DESC`.
- Columns: session name · ticket ID + new status · relative age (s → m → h → d).
- **New entry animation:** when a fresh transition is detected on poll, the new row slides
  in on arrival. Existing rows shift down.
- **New entry highlight:** fresh rows render with a vivid background color (primary or
  secondary from the color palette) that fades back to the default background over a short
  duration (~1–2s). Draws the eye immediately without requiring the user to watch the feed.
- The newest entry is marked with `▶`.
- Feed is scrollable with `j` / `k`.
- `enter` on a row opens the ticket detail modal.

### Sessions

A smaller section below the change feed. Same notification concept — new rows animate in
on arrival. Shows session start and end events with wall-clock timestamps.

```
SESSIONS
alice-arch    started   14:28
bob-impl      started   14:15
```

Followed by the live session count summary:

```
🧠 arch: 1   ⚙️  impl: 2
```

The monitor session itself uses role `monitor` and is excluded from these counts.

### Keyboard shortcuts

| Key | Action |
|-----|--------|
| `↑` `↓` / `j` `k` | Scroll active section |
| `tab` | Cycle focus: stats → ticket changes → sessions |
| `enter` | Open ticket detail (when ticket changes is focused) |
| `?` | Toggle help |
| `q` / `Ctrl+C` | Quit |

---

## Testing & visual feedback

### VHS setup (one-time, per machine)

VHS uses [go-rod](https://github.com/go-rod/rod) to drive a headless Chromium instance.
On Ubuntu with Chromium installed via snap, VHS crashes on launch because snap's crashpad
handler fails in headless environments:

```
chrome_crashpad_handler: --database is required
```

**Fix:** create a wrapper at `/usr/bin/chromium` that injects `--disable-crash-reporter`.
go-rod checks `/usr/bin/chromium` before `/usr/bin/chromium-browser`, so this wrapper
takes priority.

Run these three commands once:

```bash
echo '#!/bin/bash' | sudo tee /usr/bin/chromium
echo 'exec /usr/bin/chromium-browser --disable-crash-reporter "$@"' | sudo tee -a /usr/bin/chromium
sudo chmod +x /usr/bin/chromium
```

Verify it works:
```bash
/usr/bin/chromium --version   # should print Chromium version, no crashpad error
```

**Tape settings:** Do not use `Set Theme`, `Set FontSize`, or `Set FontFamily` in tapes.
With snap Chromium these cause a JS initialization error (`Cannot set properties of
undefined`) because xterm.js isn't fully ready when those settings are applied. Plain
`Set Width` / `Set Height` work fine. Use pixel dimensions (e.g. `1200 x 800`).

**Never run `vhs publish`** — all GIF output stays local (see tkt context #6).

### Running a tape

All tapes run from `.screenshots/fake-project/` against the local fake project DB.
Before recording, build a fresh binary into that directory:

```bash
go build -o .screenshots/fake-project/tkt .
cd .screenshots/fake-project
vhs monitor-side.tape
```

The tape sets `Env PATH` to include `.screenshots/fake-project/` so `./tkt` resolves
correctly inside the VHS shell session.

Output GIFs land at:
- `.screenshots/fake-project/monitor-side.tape` → `assets/monitor-side.gif`
- `.screenshots/fake-project/monitor.tape` → `assets/readme_example.gif`
- `monitor.tape` (root) → `assets/monitor.gif`

### Tapes

| Tape | Location | Output |
|------|----------|--------|
| `monitor.tape` | repo root | `assets/monitor.gif` |
| `monitor.tape` | `.screenshots/fake-project/` | `assets/readme_example.gif` |
| `monitor-side.tape` | `.screenshots/fake-project/` | `assets/monitor-side.gif` |

### Fake project

The fake project at `.screenshots/fake-project/` is the data source for all monitor tapes.
It is gitignored and safe to modify freely (see context #3).

For side mode to render meaningfully it needs richer data than minimal requires:

- Tickets spread across **all statuses** (already present)
- Tickets with **varied `attention_level` values** (critical / high / normal / low)
- Tickets with **varied `main_type` values** (feature / bug / chore / docs / etc.)
- Several rows in `ticket_log` with `kind = 'transition'` and recent `created_at`
  timestamps so the changes feed is not empty
- At least one active session row so the session counts are non-zero

To reseed the fake project DB:

```bash
go run scripts/seed_fake_project.go
```

### Animation testing limitation

The highlight-fade animation on new feed entries cannot be fully captured by a VHS tape
because it requires a live DB mutation while the monitor is running. Two approaches:

1. **Helper script** — use VHS `Exec` partway through the tape to run `tkt advance`
   against the fake project, triggering a real transition that side picks up on the next
   poll. `Sleep 6s` after the `Exec` to guarantee the poll fires and the animated entry
   is visible.
2. **Manual review** — animations are verified by hand during development; the tape
   covers layout only.

---

## Notes

- Both modes are read-only. Ticket mutations require other `tkt` commands.
- Both modes use the same 5-second poll interval and epoch-guarded async refresh.
- The clock in side mode updates every second on its own tick — independent of the poll.
- Session counts (`arch`/`impl`) live exclusively in side mode. Minimal mode has no session info.
- `tkt --dir /path/to/project monitor side` works as expected.
- The two modes are designed to be run together in a split terminal pane, but each works
  standalone. `tkt monitor minimal` and `tkt monitor side` do not communicate with each
  other — they are two independent processes that each poll the database on their own
  5-second cycle, with no coordination or signalling between them.
