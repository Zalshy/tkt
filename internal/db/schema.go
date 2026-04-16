package db

// Named string constants for every DDL statement.
// No logic here — constants only.

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

const createTableTicketDependencies = `CREATE TABLE ticket_dependencies (
    ticket_id   INTEGER NOT NULL REFERENCES tickets(id),
    depends_on  INTEGER NOT NULL REFERENCES tickets(id),
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (ticket_id, depends_on),
    CHECK (ticket_id != depends_on)
)`

const createIndexTicketLogTicketID = `CREATE INDEX idx_ticket_log_ticket_id  ON ticket_log(ticket_id)`

const createIndexTicketLogKind = `CREATE INDEX idx_ticket_log_kind       ON ticket_log(kind)`

const createIndexTicketLogDeletedAt = `CREATE INDEX idx_ticket_log_deleted_at ON ticket_log(deleted_at)`

const createTableRoles = `CREATE TABLE roles (
    name       TEXT PRIMARY KEY,
    base_role  TEXT NOT NULL,
    is_builtin INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    CHECK (base_role IN ('architect', 'implementer'))
)`

const createIndexTicketsStatusDeletedAt = `CREATE INDEX idx_tickets_status_deleted_at ON tickets(status, deleted_at)`

const createIndexTicketLogTicketIDKind = `CREATE INDEX idx_ticket_log_ticket_id_kind ON ticket_log(ticket_id, kind)`

const createIndexTicketDependsDependsOn = `CREATE INDEX idx_ticket_dependencies_depends_on ON ticket_dependencies(depends_on)`

const createIndexSessionsExpiredAt = `CREATE INDEX idx_sessions_expired_at ON sessions(expired_at)`

const createTableTicketUsage = `
CREATE TABLE ticket_usage (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ticket_id   INTEGER NOT NULL REFERENCES tickets(id),
    session_id  TEXT    NOT NULL,
    tokens      INTEGER NOT NULL,
    tools       INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    agent       TEXT    NOT NULL DEFAULT '',
    label       TEXT    NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    deleted_at  DATETIME NULL
)`

const createIndexTicketUsageTicketIDDeletedAt = `CREATE INDEX idx_ticket_usage_ticket_id_deleted_at ON ticket_usage(ticket_id, deleted_at)`

const createTableTicketLogNew = `
CREATE TABLE ticket_log_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ticket_id   INTEGER NOT NULL REFERENCES tickets(id),
    session_id  TEXT    NOT NULL REFERENCES sessions(id),
    kind        TEXT    NOT NULL CHECK (kind IN ('transition', 'plan', 'message')),
    body        TEXT    NOT NULL,
    from_state  TEXT,
    to_state    TEXT,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    deleted_at  DATETIME NULL
)`

const createIndexTicketLogNewTicketID     = `CREATE INDEX idx_ticket_log_new_ticket_id        ON ticket_log_new(ticket_id)`
const createIndexTicketLogNewKind         = `CREATE INDEX idx_ticket_log_new_kind             ON ticket_log_new(kind)`
const createIndexTicketLogNewDeletedAt    = `CREATE INDEX idx_ticket_log_new_deleted_at       ON ticket_log_new(deleted_at)`
const createIndexTicketLogNewTicketIDKind = `CREATE INDEX idx_ticket_log_new_ticket_id_kind   ON ticket_log_new(ticket_id, kind)`

const createTableTicketDependenciesNew = `CREATE TABLE ticket_dependencies_new (
    ticket_id   INTEGER NOT NULL REFERENCES tickets(id),
    depends_on  INTEGER NOT NULL REFERENCES tickets(id),
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (ticket_id, depends_on),
    CHECK (ticket_id != depends_on)
)`

const createIndexTicketDependenciesNewDependsOn = `CREATE INDEX idx_ticket_dependencies_new_depends_on ON ticket_dependencies_new(depends_on)`
