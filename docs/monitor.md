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
terminal pane, but works standalone too. Shows live stats, a session list, a ticket activity
feed, token burn totals, and a velocity sparkline.

**Layout:**

```
┌─────────────────────────────────────────────────┐
│  tkt monitoring                          17:12  │  ← full-width purple pill + cyan clock badge
├──────────────┬──────────────────┬───────────────┤
│  By Status   │  By Attention    │  By Type      │  ← 3 equal stat boxes
│  todo    ▌ 2 │  critical ▌▌  2  │  bug   ▌▌  2  │
│  planning▌ 1 │  high    ▌    1  │  feature▌▌ 2  │
│  in_prog…▌ 1 │  medium  ▌    1  │  chore  ▌  1  │
│  done    ▌ 1 │  low     ▌    1  │  docs   ▌  1  │
│  verified▌ 1 │  unset      0    │               │
├──────────────┴──────────────────┴───────────────┤
│  ░░░░░░░░░░▓▓▓░░░░░░░░░░░░░░░░░░░░░░░░          │  ← comet separator (animated)
├─────────────────┬───────────────────────────────┤
│  SESSIONS       │  TICKET ACTIVITY              │  ← left 1/3 / right 2/3
│  architect   3  │ ▶ alice-impl · #5 → done  2m  │
│  implementer 1  │   bob-arch  · #3 → verified 1h│
│  ───────────    │   carol-impl⚠· #1 → verified 2h│
│  alice-arch arch 11:10  │   ...                 │
│  bob        impl 09:55  │                       │
├─────────────────┼───────────────────────────────┤
│  TOKEN BURN     │  VELOCITY                 n/a │
│  total  369.5K  │  ─────────────────────────    │
│  arch   283.2K  │  -2h                     now  │
│  impl    86.3K  │                               │
├─────────────────┴───────────────────────────────┤
│  q quit                                         │
└─────────────────────────────────────────────────┘
```

### Header

A full-width purple pill containing the wordmark "tkt monitoring". A cyan clock badge is
pinned to the right edge showing `HH:MM`. The clock updates at the next minute boundary,
not every second.

### Stats row

Three equal-width boxes displayed side by side: **By Status**, **By Attention**, and
**By Type**. Each box has a centred title, a blank line, then label+bar+count rows. Bars
are built from `▌` characters, one per ticket, capped at the box width. The By Type box
shows the top 5 types by count. All three boxes exclude CANCELED and ARCHIVED tickets.

### Comet separator

A single animated row between the stats row and the bottom panels. A bright "comet"
ping-pongs left to right and back with a blended fading tail. Purely cosmetic, no
interaction.

### SESSIONS (left column, 1/3 width)

- Centred title "SESSIONS"
- Two count lines: `architect N` / `implementer N` (derived from visible session rows)
- A `───` divider
- Up to 5 rows: session name · role abbreviation (`arch` / `impl`) · start time `HH:MM`
- New sessions highlight amber for approximately 1.5 s on arrival

The monitor session itself uses role `monitor` and is excluded from these counts.

### TICKET ACTIVITY (right column, 2/3 width)

A feed of ticket state transitions and ticket creations, newest first.

```
TICKET ACTIVITY
▶ alice-impl · #5 → done         2m
  bob-arch   · #3 → verified     1h
  carol-impl⚠· #1 → verified     2h
```

- Sourced from `ticket_log WHERE kind='transition'` UNION ticket creations from `tickets`,
  ordered newest first, limit 15.
- Columns: marker (2 chars) · session name · `· #N →` · state · relative age.
- `▶` marks the most-recent entry; all others use two spaces.
- New entries get an amber background highlight for approximately 1.5 s on arrival. Entries
  appear on the next poll — there is no slide-in animation.
- **Forced transitions** render differently from normal rows:
  - The `▶` / `  ` marker is unchanged.
  - The session name renders in its normal per-session colour.
  - Immediately after the session name: a `⚠` symbol.
  - Everything from `·` onward (`· #N → state   age`) renders in amber/warning colour.
  - Example: `  carol-impl⚠· #1 → verified    2h ago`

### TOKEN BURN (bottom-left, full left-column width)

- Centred title "TOKEN BURN"
- Three rows: `total`, `arch`, `impl` with right-aligned token counts formatted as
  `369.5K` or `1.84M`
- Sourced from `ticket_usage`, excludes CANCELED and ARCHIVED tickets

### VELOCITY (bottom-right, full right-column width)

- Sparkline of ticket completions over the last approximately 2 hours, bucketed
- Shows `n/a` label when no completion data exists
- Time axis: `-2h` on the left, `now` on the right

### Keyboard shortcuts

| Key | Action |
|-----|--------|
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

### Visual test script

`scripts/visual-test.sh` is a local dev script (gitignored) that builds the binary and
records the GIF at three terminal sizes, then extracts frame 125 from each GIF as a PNG
for layout review:

```bash
bash scripts/visual-test.sh
# outputs:
#   /tmp/monitor-small.png   (~101×28 chars)
#   /tmp/monitor-medium.png  (~140×35 chars)
#   /tmp/monitor-large.png   (~180×45 chars)
```

| Size   | Pixels    | Approx cols×rows |
|--------|-----------|------------------|
| small  | 1010×560  | ~101×28          |
| medium | 1400×700  | ~140×35          |
| large  | 1800×900  | ~180×45          |

---

## Notes

- Both modes are read-only. Ticket mutations require other `tkt` commands.
- Both modes use the same 5-second poll interval and epoch-guarded async refresh.
- The clock in side mode shows `HH:MM` and updates at the next minute boundary — independent of the poll.
- Session counts (`arch`/`impl`) live exclusively in side mode. Minimal mode has no session info.
- `tkt --dir /path/to/project monitor side` works as expected.
- The two modes are designed to be run together in a split terminal pane, but each works
  standalone. `tkt monitor minimal` and `tkt monitor side` do not communicate with each
  other — they are two independent processes that each poll the database on their own
  5-second cycle, with no coordination or signalling between them.
