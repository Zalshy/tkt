# tkt man

Read built-in manual pages embedded in the `tkt` binary.

## Usage

```
tkt man
tkt man <page>
```

## Flags

No flags beyond the global `--dir`.

## Notes / Behaviour

- `tkt man` lists available built-in pages.
- `tkt man minimal` is a compact bootstrap guide for humans and LLM agents.
- `tkt man llm` aliases to `tkt man minimal`.
- Manual pages are embedded in the binary, so they work after `go install`.
- `tkt man` is separate from `tkt doc`: `tkt doc` manages project-local long-form documents in `.tkt/docs/`.
- Error messages for common workflow mistakes point to relevant man pages.

## Examples

```bash
tkt man
tkt man minimal
tkt man workflow
tkt man state-machine
tkt man advance
tkt man stats
tkt man llm
```
