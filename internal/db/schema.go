package db

// Named string constants for every DDL statement.
// No logic here — constants only.

const createTableTickets = `
CREATE TABLE tickets (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'TODO',
    created_by  TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    deleted_at  DATETIME NULL
)`

const createTableTicketLog = `
CREATE TABLE ticket_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ticket_id   INTEGER NOT NULL REFERENCES tickets(id),
    session_id  TEXT NOT NULL,
    kind        TEXT NOT NULL,
    body        TEXT NOT NULL,
    from_state  TEXT,
    to_state    TEXT,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    deleted_at  DATETIME NULL
)`

const createTableSessions = `
CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,
    role        TEXT NOT NULL,
    name        TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    last_active DATETIME NOT NULL DEFAULT (datetime('now')),
    expired_at  DATETIME NULL
)`

const createTableProjectContext = `
CREATE TABLE project_context (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL,
    session_id TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    deleted_at DATETIME NULL
)`

const createTableTicketDependencies = `CREATE TABLE ticket_dependencies (
    ticket_id   INTEGER NOT NULL REFERENCES tickets(id),
    depends_on  INTEGER NOT NULL REFERENCES tickets(id),
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (ticket_id, depends_on),
    CHECK (ticket_id != depends_on)
)`

const createIndexTicketsStatus = `
CREATE INDEX idx_tickets_status ON tickets(status)`

const createIndexTicketsDeletedAt = `
CREATE INDEX idx_tickets_deleted_at ON tickets(deleted_at)`

const createIndexTicketLogTicketID = `CREATE INDEX idx_ticket_log_ticket_id  ON ticket_log(ticket_id)`

const createIndexTicketLogKind = `CREATE INDEX idx_ticket_log_kind       ON ticket_log(kind)`

const createIndexTicketLogDeletedAt = `CREATE INDEX idx_ticket_log_deleted_at ON ticket_log(deleted_at)`

const createIndexProjectContextDeletedAt = `
CREATE INDEX idx_project_context_deleted_at ON project_context(deleted_at)`

const createTableRoles = `CREATE TABLE roles (
    name       TEXT PRIMARY KEY,
    base_role  TEXT NOT NULL,
    is_builtin INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    CHECK (base_role IN ('architect', 'implementer'))
)`
