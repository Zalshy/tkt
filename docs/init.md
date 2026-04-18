# tkt init

Initialise a new tkt project in the current directory (or `--dir` target).

## Usage

```
tkt init [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--name` | string | `""` | Project name (default: directory basename) |

## Notes / Behaviour

- Creates a `.tkt/` directory in the target directory.
- Running `tkt init` in a directory that already has `.tkt/` will error.
- When `--name` is omitted the project name defaults to the basename of the directory.

## Examples

```bash
tkt init
tkt init --name "my-project"
tkt --dir /path/to/repo init --name "repo-name"
```
