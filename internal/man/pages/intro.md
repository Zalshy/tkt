# intro

`tkt` is a project-local ticket CLI for human and AI-agent collaboration.

Each project stores data in `.tkt/`. Work is organized as tickets, sessions, logs, context entries, project documents, and optional MCP tools.

Common start:

```bash
tkt init
tkt session --role architect
tkt new "Implement feature" --description-file spec.md --type feature --attention 40
tkt list --all
```

`tkt man` pages are built into the binary. Project-specific long-form documents are managed separately with `tkt doc`.
