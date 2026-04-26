# state-machine

Valid statuses:
- `TODO`
- `PLANNING`
- `IN_PROGRESS`
- `DONE`
- `VERIFIED`
- `CANCELED`
- `ARCHIVED`

Natural transitions:
- `TODO -> PLANNING`
- `PLANNING -> IN_PROGRESS`
- `IN_PROGRESS -> DONE`
- `DONE -> VERIFIED`
- `VERIFIED -> ARCHIVED`

Role/isolation rules:
- `PLANNING -> IN_PROGRESS` requires architect-effective role and different session from last transition/creator.
- `DONE -> VERIFIED` requires architect-effective role and different session from last transition.
- Implementer sessions can normally move implementation work to `DONE`.
- `--force` bypasses soft role/isolation checks and records a violation.

Plan guard:
- `PLANNING -> IN_PROGRESS` fails if no plan log entry exists.

There is no `PLANNED` status.
