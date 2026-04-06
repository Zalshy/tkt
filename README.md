# tkt

A project-local CLI ticket system with role-based session isolation and a plan-first workflow. Built for human + AI agent collaboration.

## Install

```bash
go install github.com/zalshy/tkt@latest
```

Or build from source:

```bash
git clone https://github.com/zalshy/tkt
cd tkt
make build     # produces bin/tkt
make install   # installs to $GOPATH/bin
```

## Quick start

```bash
tkt init                          # initialise a project
tkt session --role architect      # declare your role
tkt new "Fix the login bug"       # create a ticket
tkt list                          # see what's open
tkt advance 1                     # move it forward
tkt show 1                        # inspect a ticket
```

## Roles

| Role | Can do |
|---|---|
| `architect` | Create tickets, write and approve plans, verify completed work |
| `implementer` | Pick up planned tickets, implement, submit for review |

Custom role names can be created and mapped to either built-in:

```bash
tkt role create security-expert --like architect
tkt session --role security-expert
```

> Role customisation will be expanded in future updates.

## Commands

```
tkt init                            Initialise a new project
tkt session                         Show active session
tkt session --role <role>           Start a new session
tkt session --end                   End the current session
tkt new "<title>"                   Create a ticket
tkt list                            List open tickets
tkt show <id>                       Show a ticket with full log
tkt advance <id>                    Move a ticket to the next state
tkt plan <id>                       Write or revise a ticket plan
tkt comment <id> "<msg>"            Add a comment to a ticket
tkt depends <id> --on <ids>         Declare ticket dependencies
tkt context readall                 Read all project context entries
tkt role create/list/delete         Manage custom roles
tkt doc add/list/read/archive       Manage documents
tkt monitor                         Read-only TUI dashboard
```

## Ticket lifecycle

```
TODO → PLANNING → IN_PROGRESS → DONE → VERIFIED
```

### The PLANNING step

PLANNING is the core of tkt's workflow — and what sets it apart from a simple task tracker.

When a ticket enters PLANNING, the architect writes a plan: exact files to touch, function signatures, edge cases, test strategy. No code is written yet. The plan is a contract, not a sketch.

Once written, the plan must be approved by a **different session** before implementation can begin. The state machine enforces this — the same session that wrote the plan cannot advance it to IN_PROGRESS. This is not just a process rule; it is structurally impossible to bypass without a recorded violation.

When the implementer picks up the ticket, the plan is **frozen**. Any deviation during implementation must be logged as a comment explaining why. The architect reviews the final code against the frozen plan at DONE — not against their memory of what they intended.

The result: every piece of work has a written, reviewed, timestamped specification that exists before a single line of code is written. The audit trail is complete and tamper-evident.

```
Implementer picks up + writes plan  →  Architect approves  →  Implementer executes  →  Architect verifies
           PLANNING                        IN_PROGRESS                DONE                  VERIFIED
```

### Full lifecycle

- An **implementer** advances TODO → PLANNING and writes the plan
- An **architect** reviews the plan — approval *is* the PLANNING → IN_PROGRESS transition (enforced by the state machine)
- The **implementer** executes the approved plan (IN_PROGRESS → DONE)
- The **architect** verifies the code against the frozen plan (DONE → VERIFIED)

Role and session isolation is enforced by the state machine — the same session that submitted a ticket cannot approve its own work.

## License

MIT
