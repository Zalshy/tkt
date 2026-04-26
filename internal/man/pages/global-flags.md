# tkt global flags

Persistent flags available on every tkt command.

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dir` | string | `""` | Override project root directory (default: cwd or nearest `.tkt/` parent) |

## Notes / Behaviour

- `--dir` must point to a directory that contains a `.tkt/` subdirectory, or be the directory where `tkt init` will be run.
- When `--dir` is omitted, tkt walks up from the current working directory looking for the nearest `.tkt/` parent.
- Useful in scripts or CI where the working directory is not the project root.

## Examples

```bash
tkt --dir /path/to/project list
tkt --dir /path/to/project advance 5 --note "done"
```
