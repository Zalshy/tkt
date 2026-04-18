# tkt monitor

Live terminal UI for watching ticket activity in real time.

## Usage

```
tkt monitor
```

## Flags

No flags beyond the global `--dir`.

## Notes / Behaviour

- Opens a full-screen TUI that refreshes automatically as tickets change.
- Displays tickets across all active states with status, tier, and recent activity.
- Press `q` or `Ctrl-C` to exit.
- Intended for passive observation; ticket mutations must be performed via other commands.

## Examples

```bash
tkt monitor
tkt --dir /path/to/project monitor
```
