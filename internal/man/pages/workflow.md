# workflow

Default lifecycle:

```text
TODO -> PLANNING -> IN_PROGRESS -> DONE -> VERIFIED -> ARCHIVED
```

Recommended flow:
1. Create ticket with `tkt new`.
2. Move to planning with `tkt advance <id> --note ...`.
3. Write plan with `tkt plan <id> --file plan.md`.
4. A different architect-effective session approves with `tkt advance <id> --note ...`.
5. Implementer completes work and advances to `DONE`.
6. Architect verifies and advances to `VERIFIED`.
7. Archive verified tickets with `tkt archive <id>` when no longer active.

Use `tkt batch` to find dependency-ready tickets. Use `tkt context readall` for caveats agents must know.
