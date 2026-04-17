# tkt

A project-local CLI ticket system with role-based session isolation and a plan-first workflow. Built for human + AI agent collaboration.

tkt is **LLM-CLI agnostic** — it ships an MCP server over stdio so any MCP-compatible tool (Claude Code, Claude Desktop, Cursor, Zed, or your own agent) can drive it without glue code. The workflow, state machine, and audit trail are the same regardless of what's on the other end of the pipe.

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
tkt plan <id>                       Write or revise a ticket plan (opens $EDITOR)
tkt plan <id> --body/--stdin/--file Supply plan non-interactively
tkt comment <id> "<msg>"            Add a comment to a ticket
tkt depends <id> --on <ids>         Declare ticket dependencies
tkt tier <id> <tier>                Set ticket tier (critical|standard|low)
tkt context readall/add/update/delete  Manage project context entries
tkt role create/list/delete         Manage custom roles
tkt doc add/list/read/archive       Manage documents
tkt doc add <slug> --body/--stdin/--file  Create a document non-interactively
tkt search <query>                  Substring search across ticket titles and descriptions
tkt log <id> --tokens N             Record token/tool/duration usage against a ticket
tkt archive <id>                    Archive a VERIFIED ticket (terminal state)
tkt cleanup                         Expire stale sessions and run maintenance
tkt monitor                         Read-only TUI dashboard (auto-refreshes every 5s)
tkt mcp                             Start MCP server (stdio transport)
```

## Ticket lifecycle

```
TODO → PLANNING → IN_PROGRESS → DONE → VERIFIED → ARCHIVED
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

### Non-interactive input

`tkt plan` and `tkt doc add` both support `--body`, `--stdin`, and `--file` flags to skip `$EDITOR` — useful for agents and scripts:

```bash
tkt plan 42 --body "## Plan\n..."
cat plan.md | tkt plan 42 --stdin
tkt plan 42 --file ./plan.md

tkt doc add my-slug --body "# D001 — Title\n..."
cat doc.md | tkt doc add my-slug --stdin
tkt doc add my-slug --file ./doc.md
```

All three are mutually exclusive. An empty body is an error.

### Full lifecycle

- An **implementer** advances TODO → PLANNING and writes the plan
- An **architect** reviews the plan — approval *is* the PLANNING → IN_PROGRESS transition (enforced by the state machine)
- The **implementer** executes the approved plan (IN_PROGRESS → DONE)
- The **architect** verifies the code against the frozen plan (DONE → VERIFIED)

Role and session isolation is enforced by the state machine — the same session that submitted a ticket cannot approve its own work.

### ARCHIVED status

VERIFIED tickets can be archived to clear board noise. ARCHIVED is a terminal state — it cannot be undone. Archived tickets are hidden from `tkt list` and the TUI monitor by default.

```bash
tkt archive 42
tkt archive 42,43,44

tkt list --archived          # include ARCHIVED tickets
tkt list --status ARCHIVED   # show only ARCHIVED tickets
```

## Search

Substring search across ticket titles and descriptions:

```bash
tkt search kanban                        # search all tickets
tkt search kanban --title                # title only
tkt search kanban --status IN_PROGRESS   # filter by status
tkt search kanban --all                  # include all statuses
```

## Token logging

Record agent usage against a ticket. Visible in `tkt show` as a Token usage section:

```bash
tkt log 42 --tokens 78155 --tools 88 --duration 386 --agent implementer
tkt log 42 --tokens 12000 --label "planning pass"
```

All flags except `--tokens` are optional.

## TUI monitor

The TUI monitor (`tkt monitor`) shows a live kanban board. Key bindings:

- Arrow keys / `h` `j` `k` `l` — navigate tickets
- `X` — bulk-archive all VERIFIED tickets, keeping the 10 most recent (confirmation prompt)
- `q` / `Ctrl+C` — quit

The footer displays centered key hints. A session count line above the footer shows active sessions by role: `🧠 arch: N   ⚙️ impl: N`.

## MCP server

tkt ships a built-in MCP server over stdio, compatible with any MCP-capable LLM tool:

```
command:   tkt
args:      ["mcp"]
transport: stdio
```

Run `tkt init` in a project for the exact snippet to paste into your tool's config.

### Options

```bash
tkt mcp                        # default role: implementer
tkt mcp --role architect       # start session as architect
tkt mcp --readonly             # expose read-only tools only (no session required)
```

### Tools

The server exposes 24 tools covering the full tkt surface: creating and advancing tickets, writing plans, managing context entries, documents, roles, token logging, and board queries. All tools respect the same role and session isolation rules as the CLI.

## License

MIT
