# tkt — Ticket System for AI Agents

tkt is a project-local CLI ticket system with role-based session isolation and
a plan-first workflow. This file tells you everything you need to operate it.

## Your first action in every session

Run this to check what exists and who you are:

    tkt session          # see your current role and session ID
    tkt list             # see the 10 most recently updated tickets
    tkt context readall  # read all project context entries

If you have no active session, declare one first:

    tkt session --role architect      # if you are reviewing/planning
    tkt session --role implementer    # if you are implementing

## Roles

There are three roles. Two are tkt session roles; one is a coordination role above the system.

**orchestrator** — you drive the board forward. You have no tkt session. You read the
board state, spawn architect and implementer agents, surface decisions to the user, and
advance tickets based on agent results. You are the only role that talks to the user.

**architect** — you create tickets, write and review plans, approve/reject
plans, and give final sign-off on completed work. You never implement code.

**implementer** — you pick up planned tickets, write plans, execute the work,
and submit tickets as done. You cannot approve your own work.

## Ticket lifecycle

Tickets move through these states in order:

    TODO → PLANNING → IN_PROGRESS → DONE → VERIFIED
    (any state except DONE/VERIFIED) → CANCELED → TODO (re-open)

Key rules:
- Only an **architect** can approve a plan (PLANNING → IN_PROGRESS)
- Only an **implementer** can submit work as done (IN_PROGRESS → DONE)
- Only an **architect** can verify completed work (DONE → VERIFIED)
- The session that submitted a plan cannot be the one that approves it
- The plan field is editable ONLY during PLANNING state — it freezes after approval

## Core workflow

### As an orchestrator

The orchestrator has no tkt session. It reads, coordinates, and records — it does not
create plans or write code.

    # Start every session by reading the board
    tkt list --all
    tkt context readall

    # Understand a ticket before acting on it
    tkt show #42
    tkt log #42

    # Advance a ticket on behalf of an agent result (e.g. auto-verify standard tier)
    tkt advance #42 --note "standard tier — implementer reported passing tests; auto-verified"

    # Record a decision or finding in the audit trail
    tkt comment #42 "Architect flagged plan — surfaced to user, awaiting decision"

**Orchestrator rules:**
- Never spawn an architect or implementer without telling the user what you are about to do and why
- Never act on an agent result silently — always report what the agent returned and what you are doing next
- Stop and ask the user before: priority decisions, architect criticisms, key technical decisions
- One implementer per ticket — never merge two tickets into one agent
- Standard and low tier: auto-verify if implementer reports passing tests
- Critical tier: always send to architect for code review before verifying

### As an architect

    # Create a ticket
    tkt new "Add rate limiting to API" --description "Protect /api/* routes"

    # Review a plan submitted by an implementer
    tkt show #42          # read the plan
    tkt log #42           # read full history

    # Approve the plan
    tkt advance #42 --note "Plan approved, good approach"

    # Reject the plan (sends it back to PLANNING)
    tkt advance #42 --to PLANNING --note "Plan needs to address token refresh edge case"

    # Verify completed work
    tkt advance #42 --note "Work matches plan, confirmed"

### As an implementer

    # Pick up a ticket
    tkt advance #42 --note "Starting work on this"
    # (moves it from TODO to PLANNING)

    # Write the plan
    tkt plan #42
    # (opens $EDITOR — write what you will do and how, then save)

    # Submit plan for review (architect must approve from a different session)
    tkt advance #42 --note "Plan ready for architect review"
    # (plan is now ready; architect runs tkt advance #42 --note "approved" to move to IN_PROGRESS)

    # After plan is approved by architect, submit work as done
    tkt advance #42 --note "Implementation complete, followed plan"
    # (moves it from IN_PROGRESS to DONE)

## Adding context along the way

    # Add a comment (any time, any state, any role)
    tkt comment #42 "Discovered a complexity: token refresh needs special handling"

    # View full history of a ticket
    tkt log #42

## Viewing state

    tkt list                          # last 10 tickets (most recently updated)
    tkt list --status PLANNING        # tickets where plan is being written/awaiting approval
    tkt list --status IN_PROGRESS     # tickets being implemented
    tkt list --limit 50               # last 50 tickets
    tkt list --all                    # every ticket (no limit)
    tkt list --sort id                # ordered by creation number (newest first)
    tkt show #42                  # full ticket detail with plan
    tkt log #42                   # full event + comment timeline

## Important rules for agents

1. **Always write a real plan.** The plan field should explain your approach,
   the key steps, any risks or uncertainties, and any decisions made. It is
   the primary artifact for review — not the code itself.

2. **Never self-approve.** You cannot approve transitions you submitted. If
   you need to override this, use --force, but this will be recorded as a
   violation in the audit log.

3. **Always provide --note.** Every `tkt advance` call requires --note. Write
   a meaningful note — it becomes part of the permanent audit trail.

4. **Check your role before acting.** Run `tkt session` at the start of every
   session. If your role is wrong for the task, run `tkt session --role <role>`
   to get a new session with the correct role.

5. **Do not implement during PLANNING.** While in PLANNING state, your only
   job is to write the plan. Do not touch implementation files until the
   ticket is IN_PROGRESS.

## Command quick reference

    tkt init                                  initialize project
    tkt session                               show current session
    tkt session --role <architect|implementer> start new session with role (orchestrator has no session)
    tkt new "<title>" [--description "<text>"] create ticket
    tkt list [--status <STATUS>] [--limit <n>] [--all] [--verified] [--sort <updated|id>]  list tickets
    tkt show <id>                             show ticket detail
    tkt plan <id>                             edit plan (PLANNING state only)
    tkt advance <id> --note "<note>"          advance to next state
    tkt advance <id> --to <STATE> --note "…"  advance to specific state
    tkt advance <id> --note "…" --force       override role check with warning
    tkt comment <id> "<body>"                 add comment
    tkt log <id>                              full history + comments
    tkt context add "<title>" "<body>"        add a context entry
    tkt context readall                       read all context entries (run at startup)
    tkt context read <id>                     read a single context entry
    tkt context update <id> "<title>" "<body>" update a context entry
    tkt context delete <id>                   delete a context entry
    tkt monitor                               read-only TUI dashboard
