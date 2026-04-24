package ticket

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zalshy/tkt/internal/models"
)

// ErrNotFound is returned when a ticket with the given ID does not exist
// (or has been soft-deleted).
var ErrNotFound = errors.New("ticket not found")

// ListOptions controls how List queries the database.
type ListOptions struct {
	Status          *models.Status // nil = all non-VERIFIED (unless IncludeVerified)
	Limit           int            // 0 = use default (10)
	All             bool           // true = no LIMIT clause; also disables soft-delete filter
	IncludeVerified bool
	IncludeArchived bool           // true = include ARCHIVED tickets in results
	ExcludeCanceled bool
	Sort            string // "updated" (default) or "id"
	Ready           bool   // true = only tickets with no unresolved dependencies
	Query           string // non-empty: LIKE '%query%' on title+description (or title only)
	TitleOnly       bool   // true: restrict query match to title only
}

// ListResult wraps the returned ticket slice with a pagination hint.
type ListResult struct {
	Tickets []models.Ticket
	HasMore bool
}

// Create inserts a new ticket and returns the fully-populated Ticket.
func Create(title, description, tier string, actor *models.Session, db *sql.DB, mainType string, attentionLevel int) (*models.Ticket, error) {
	if tier != "critical" && tier != "standard" && tier != "low" {
		return nil, fmt.Errorf("ticket.Create: invalid tier %q: must be critical, standard, or low", tier)
	}
	if len(mainType) > 30 {
		return nil, fmt.Errorf("main_type exceeds 30 character limit")
	}
	if attentionLevel < 0 || attentionLevel > 99 {
		return nil, fmt.Errorf("ticket.Create: attention_level must be in [0, 99]")
	}
	result, err := db.Exec(
		`INSERT INTO tickets (title, description, status, tier, created_by, main_type, attention_level)
		 VALUES (?, ?, 'TODO', ?, ?, ?, ?)`,
		title, description, tier, actor.ID, mainType, attentionLevel,
	)
	if err != nil {
		return nil, fmt.Errorf("ticket.Create: insert: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("ticket.Create: last insert id: %w", err)
	}

	return GetByID(strconv.FormatInt(id, 10), db)
}

// GetByID fetches a single ticket by its ID, which may be given as "42" or "#42".
// Returns ErrNotFound when the ticket does not exist or has been soft-deleted.
func GetByID(id string, db *sql.DB) (*models.Ticket, error) {
	stripped := strings.TrimPrefix(id, "#")
	n, err := strconv.Atoi(stripped)
	if err != nil {
		return nil, fmt.Errorf("%w: %q is not a valid ticket id", ErrNotFound, id)
	}

	var t models.Ticket
	var deletedAt sql.NullTime

	err = db.QueryRow(
		`SELECT id, title, description, status, tier, main_type, attention_level, created_by, created_at, updated_at, deleted_at
		 FROM tickets WHERE id = ? AND deleted_at IS NULL`,
		n,
	).Scan(
		&t.ID, &t.Title, &t.Description, &t.Status, &t.Tier, &t.MainType, &t.AttentionLevel,
		&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &deletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: ticket #%d not found", ErrNotFound, n)
		}
		return nil, fmt.Errorf("ticket.GetByID: %w", err)
	}

	if deletedAt.Valid {
		t.DeletedAt = ptr(deletedAt.Time)
	}

	return &t, nil
}

// List returns tickets matching opts plus a HasMore flag indicating whether more
// rows exist beyond the returned slice.
//
// The LIMIT+1 trick is used to detect HasMore without a second COUNT query (CONTEXT/005):
// fetch limit+1 rows; if len(rows) > limit, set HasMore=true and slice to [:limit].
func List(opts ListOptions, db *sql.DB) (ListResult, error) {
	// Build WHERE clauses.
	var where []string
	var args []any

	if !opts.All {
		where = append(where, "t.deleted_at IS NULL")
	}

	if opts.Status != nil {
		where = append(where, "t.status = ?")
		args = append(args, string(*opts.Status))
	} else if !opts.IncludeVerified {
		// When no specific status is requested, hide VERIFIED tickets by default.
		where = append(where, "t.status != 'VERIFIED'")
	}

	// ARCHIVED is hidden unconditionally unless explicitly requested via IncludeArchived
	// or by filtering directly on StatusArchived.
	if !opts.IncludeArchived && (opts.Status == nil || *opts.Status != models.StatusArchived) {
		where = append(where, "t.status != 'ARCHIVED'")
	}

	if opts.ExcludeCanceled {
		where = append(where, "t.status != 'CANCELED'")
	}

	if opts.Ready {
		where = append(where, `NOT EXISTS (
        SELECT 1
        FROM ticket_dependencies td
        JOIN tickets dep ON dep.id = td.depends_on
        WHERE td.ticket_id = t.id
          AND dep.status != 'VERIFIED'
          AND dep.deleted_at IS NULL
    )`)
	}

	if opts.Query != "" {
		q := "%" + opts.Query + "%"
		if opts.TitleOnly {
			where = append(where, "t.title LIKE ?")
			args = append(args, q)
		} else {
			where = append(where, "(t.title LIKE ? OR t.description LIKE ?)")
			args = append(args, q, q)
		}
	}

	// SECURITY: dynamic query assembly is safe — no user input reaches the SQL string.
	// - WHERE fragments appended above ("t.deleted_at IS NULL", "t.status = ?",
	//   "t.status != 'VERIFIED'", "t.status != 'CANCELED'", the NOT EXISTS subquery)
	//   are hardcoded string literals; never derived from user input.
	// - LIKE pattern values (opts.Query) are passed via ? placeholders, never interpolated.
	// - Hardcoded status literals in WHERE ('VERIFIED', 'CANCELED') are safe constants.
	// - ORDER BY is chosen from two hardcoded literals ("t.updated_at DESC",
	//   "t.id DESC") via the `opts.Sort == "id"` guard; user input cannot inject
	//   arbitrary SQL here.
	// - All user-supplied values (status filter, limit) are passed via ? placeholders,
	//   never interpolated into the query string.

	// ORDER BY
	orderBy := "t.updated_at DESC"
	if opts.Sort == "id" {
		orderBy = "t.id DESC"
	}

	// Build query string.
	query := "SELECT t.id, t.title, t.description, t.status, t.tier, t.main_type, t.attention_level, t.created_by, t.created_at, t.updated_at, t.deleted_at FROM tickets t"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY " + orderBy

	// LIMIT: fetch limit+1 to detect HasMore.
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}

	if !opts.All {
		query += " LIMIT ?"
		args = append(args, limit+1)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return ListResult{}, fmt.Errorf("ticket.List: query: %w", err)
	}
	defer rows.Close()

	var tickets []models.Ticket
	for rows.Next() {
		var t models.Ticket
		var deletedAt sql.NullTime
		if err := rows.Scan(
			&t.ID, &t.Title, &t.Description, &t.Status, &t.Tier, &t.MainType, &t.AttentionLevel,
			&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &deletedAt,
		); err != nil {
			return ListResult{}, fmt.Errorf("ticket.List: scan: %w", err)
		}
		if deletedAt.Valid {
			t.DeletedAt = ptr(deletedAt.Time)
		}
		tickets = append(tickets, t)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, fmt.Errorf("ticket.List: rows: %w", err)
	}

	hasMore := false
	if !opts.All && len(tickets) > limit {
		hasMore = true
		tickets = tickets[:limit]
	}

	return ListResult{Tickets: tickets, HasMore: hasMore}, nil
}

// GetDependencies returns all upstream tickets that ticketID directly or transitively
// depends on. Uses a recursive upstream CTE. Only ID, Title, and Status are populated;
// other fields are zero. Tickets soft-deleted at query time are excluded.
func GetDependencies(ticketID int64, db *sql.DB) ([]models.Ticket, error) {
	rows, err := db.Query(`
WITH RECURSIVE upstream(id, depth) AS (
    SELECT depends_on, 1
    FROM ticket_dependencies
    WHERE ticket_id = ?

    UNION ALL

    SELECT td.depends_on, u.depth + 1
    FROM ticket_dependencies td
    JOIN upstream u ON td.ticket_id = u.id
    WHERE u.depth < 1000
)
SELECT DISTINCT t.id, t.title, t.status, u.depth
FROM upstream u
JOIN tickets t ON t.id = u.id
WHERE t.deleted_at IS NULL
ORDER BY u.depth DESC
`, ticketID)
	if err != nil {
		return nil, fmt.Errorf("ticket.GetDependencies: %w", err)
	}
	defer rows.Close()

	var tickets []models.Ticket
	for rows.Next() {
		var t models.Ticket
		var depth int
		if err := rows.Scan(&t.ID, &t.Title, &t.Status, &depth); err != nil {
			return nil, fmt.Errorf("ticket.GetDependencies: scan: %w", err)
		}
		tickets = append(tickets, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ticket.GetDependencies: rows: %w", err)
	}
	return tickets, nil
}

// GetDependents returns all downstream tickets that directly or transitively depend on
// ticketID. Uses a recursive downstream CTE. Only ID, Title, and Status are populated;
// other fields are zero. Tickets soft-deleted at query time are excluded.
func GetDependents(ticketID int64, db *sql.DB) ([]models.Ticket, error) {
	rows, err := db.Query(`
WITH RECURSIVE downstream(id, depth) AS (
    SELECT ticket_id, 1
    FROM ticket_dependencies
    WHERE depends_on = ?

    UNION ALL

    SELECT td.ticket_id, d.depth + 1
    FROM ticket_dependencies td
    JOIN downstream d ON td.depends_on = d.id
    WHERE d.depth < 1000
)
SELECT DISTINCT t.id, t.title, t.status, d.depth
FROM downstream d
JOIN tickets t ON t.id = d.id
WHERE t.deleted_at IS NULL
ORDER BY d.depth, t.id
`, ticketID)
	if err != nil {
		return nil, fmt.Errorf("ticket.GetDependents: %w", err)
	}
	defer rows.Close()

	var tickets []models.Ticket
	for rows.Next() {
		var t models.Ticket
		var depth int
		if err := rows.Scan(&t.ID, &t.Title, &t.Status, &depth); err != nil {
			return nil, fmt.Errorf("ticket.GetDependents: scan: %w", err)
		}
		tickets = append(tickets, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ticket.GetDependents: rows: %w", err)
	}
	return tickets, nil
}

// IsReady returns true when ticketID has no unresolved dependencies — either it has none
// at all, or every dependency is VERIFIED. Soft-deleted dependency tickets are treated as
// resolved (orphaned edges are ignored). Returns (false, nil) if the ticket does not exist.
func IsReady(ticketID int64, db *sql.DB) (bool, error) {
	var count int
	err := db.QueryRow(`
    SELECT COUNT(*)
    FROM ticket_dependencies td
    JOIN tickets dep ON dep.id = td.depends_on
    WHERE td.ticket_id = ?
      AND dep.status != 'VERIFIED'
      AND dep.deleted_at IS NULL
`, ticketID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("ticket.IsReady: %w", err)
	}
	return count == 0, nil
}

// AddDependencies validates depIDs, checks for cycles, then inserts all edges
// inside a single transaction. Idempotent (INSERT OR IGNORE).
//
// Errors returned (exact strings matter — callers pattern-match on them):
//   - "a ticket cannot depend on itself" if any depID == ticketID
//   - "cycle detected — #N is already downstream of #M" if a cycle would form
//   - wraps ErrNotFound if ticketID or any depID does not exist
func AddDependencies(ticketID int64, depIDs []int64, db *sql.DB) error {
	for _, depID := range depIDs {
		if _, err := GetByID(strconv.FormatInt(depID, 10), db); err != nil {
			return err
		}
		if depID == ticketID {
			return fmt.Errorf("a ticket cannot depend on itself")
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("ticket.AddDependencies: begin tx: %w", err)
	}
	defer tx.Rollback() // no-op after successful Commit; covers all error paths

	// Cycle check inside the transaction — atomic with the INSERTs under SQLite write serialization.
	// Traverses upward: finds all tickets that (transitively) depend ON ticketID.
	// If any depID is in that set, adding ticketID→depID would create a cycle.
	rows, err := tx.Query(`
    WITH RECURSIVE downstream(id, depth) AS (
        SELECT ticket_id, 1 FROM ticket_dependencies WHERE depends_on = ?
        UNION ALL
        SELECT td.ticket_id, d.depth + 1 FROM ticket_dependencies td JOIN downstream d ON td.depends_on = d.id WHERE d.depth < 1000
    )
    SELECT id FROM downstream`, ticketID)
	if err != nil {
		return fmt.Errorf("ticket.AddDependencies: cycle check: %w", err)
	}
	downstreamSet := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("ticket.AddDependencies: cycle check scan: %w", err)
		}
		downstreamSet[id] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("ticket.AddDependencies: cycle check rows: %w", err)
	}
	for _, depID := range depIDs {
		if downstreamSet[depID] {
			return fmt.Errorf("cycle detected — #%d is already downstream of #%d", ticketID, depID)
		}
	}

	for _, depID := range depIDs {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO ticket_dependencies (ticket_id, depends_on) VALUES (?, ?)`,
			ticketID, depID,
		); err != nil {
			return fmt.Errorf("ticket.AddDependencies: insert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ticket.AddDependencies: commit: %w", err)
	}
	return nil
}

// DependencyEdge holds a single row from the batch dependency query.
type DependencyEdge struct {
	TicketID   int64
	DependsOn  int64
	TicketStat models.Status
	DepStat    models.Status
}

// ListActive returns a map of ticket ID → status for all tickets that are not
// VERIFIED, not CANCELED, and not soft-deleted.
func ListActive(db *sql.DB) (map[int64]models.Status, error) {
	rows, err := db.Query(`
		SELECT id, status FROM tickets
		WHERE status NOT IN ('VERIFIED', 'CANCELED', 'ARCHIVED')
		  AND deleted_at IS NULL
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("ticket.ListActive: query: %w", err)
	}
	defer rows.Close()

	result := make(map[int64]models.Status)
	for rows.Next() {
		var id int64
		var status models.Status
		if err := rows.Scan(&id, &status); err != nil {
			return nil, fmt.Errorf("ticket.ListActive: scan: %w", err)
		}
		result[id] = status
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ticket.ListActive: rows: %w", err)
	}
	return result, nil
}

// ListDependencyEdges returns all dependency edges for non-soft-deleted tickets,
// ordered by depends_on then ticket_id.
func ListDependencyEdges(db *sql.DB) ([]DependencyEdge, error) {
	rows, err := db.Query(`
		SELECT td.ticket_id, td.depends_on, t1.status AS ticket_status, t2.status AS dep_status
		FROM ticket_dependencies td
		JOIN tickets t1 ON t1.id = td.ticket_id
		JOIN tickets t2 ON t2.id = td.depends_on
		WHERE t1.deleted_at IS NULL AND t2.deleted_at IS NULL
		ORDER BY td.depends_on, td.ticket_id
	`)
	if err != nil {
		return nil, fmt.Errorf("ticket.ListDependencyEdges: query: %w", err)
	}
	defer rows.Close()

	var edges []DependencyEdge
	for rows.Next() {
		var e DependencyEdge
		if err := rows.Scan(&e.TicketID, &e.DependsOn, &e.TicketStat, &e.DepStat); err != nil {
			return nil, fmt.Errorf("ticket.ListDependencyEdges: scan: %w", err)
		}
		edges = append(edges, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ticket.ListDependencyEdges: rows: %w", err)
	}
	return edges, nil
}

// RemoveDependency deletes the dependency edge from ticketID to depID.
// Returns ErrNotFound when the edge does not exist.
func RemoveDependency(ticketID, depID int64, db *sql.DB) error {
	result, err := db.Exec(
		`DELETE FROM ticket_dependencies WHERE ticket_id = ? AND depends_on = ?`,
		ticketID, depID,
	)
	if err != nil {
		return fmt.Errorf("ticket.RemoveDependency: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("%w: dependency #%d → #%d not found", ErrNotFound, ticketID, depID)
	}
	return nil
}

// SetTier updates the tier of an existing ticket. Returns ErrNotFound if the
// ticket does not exist or has been soft-deleted.
func SetTier(id string, newTier string, db *sql.DB) (*models.Ticket, error) {
	t, err := GetByID(id, db)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(
		`UPDATE tickets SET tier = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		newTier, t.ID,
	); err != nil {
		return nil, fmt.Errorf("ticket.SetTier: update: %w", err)
	}

	return GetByID(id, db)
}

// Update sets main_type and/or attention_level on an existing ticket.
// At least one of mainType or attentionLevel must be non-nil.
// Returns ErrNotFound if the ticket does not exist or has been soft-deleted.
func Update(id string, mainType *string, attentionLevel *int, db *sql.DB) (*models.Ticket, error) {
	if mainType == nil && attentionLevel == nil {
		return nil, fmt.Errorf("ticket.Update: nothing to update")
	}
	if mainType != nil && len(*mainType) > 30 {
		return nil, fmt.Errorf("main_type exceeds 30 character limit")
	}
	if attentionLevel != nil && (*attentionLevel < 0 || *attentionLevel > 99) {
		return nil, fmt.Errorf("ticket.Update: attention_level must be in [0, 99]")
	}

	stripped := strings.TrimPrefix(id, "#")
	n, err := strconv.Atoi(stripped)
	if err != nil {
		return nil, fmt.Errorf("%w: %q is not a valid ticket id", ErrNotFound, id)
	}

	var setClauses []string
	var args []any
	if mainType != nil {
		setClauses = append(setClauses, "main_type = ?")
		args = append(args, *mainType)
	}
	if attentionLevel != nil {
		setClauses = append(setClauses, "attention_level = ?")
		args = append(args, *attentionLevel)
	}
	setClauses = append(setClauses, "updated_at = datetime('now')")
	args = append(args, n)

	query := "UPDATE tickets SET " + strings.Join(setClauses, ", ") + " WHERE id = ? AND deleted_at IS NULL"
	result, err := db.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("ticket.Update: exec: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("ticket.Update: rows affected: %w", err)
	}
	if affected == 0 {
		return nil, fmt.Errorf("%w: ticket #%d not found", ErrNotFound, n)
	}

	return GetByID(stripped, db)
}

// ptr returns a pointer to a time.Time value (helper for nullable scan).
func ptr(t time.Time) *time.Time { return &t }
