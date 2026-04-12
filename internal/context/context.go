package context

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/zalshy/tkt/internal/models"
)

// ErrNotFound is returned when a context entry with the given ID does not exist
// (or has been soft-deleted).
var ErrNotFound = errors.New("context entry not found")

// Add inserts a new project_context entry and returns the fully-populated Context.
// Returns an error if title or body is empty.
func Add(title, body string, actor *models.Session, db *sql.DB) (*models.Context, error) {
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("context.Add: title must not be empty")
	}
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("context.Add: body must not be empty")
	}

	result, err := db.Exec(
		`INSERT INTO project_context (title, body, session_id) VALUES (?, ?, ?)`,
		title, body, actor.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("context.Add: insert: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("context.Add: last insert id: %w", err)
	}

	return Read(int(id), db)
}

// ReadAll returns all non-deleted project_context entries ordered by id ASC.
// Returns a non-nil empty slice if no entries exist.
func ReadAll(db *sql.DB) ([]models.Context, error) {
	rows, err := db.Query(
		`SELECT id, title, body, session_id, created_at, updated_at, deleted_at
		 FROM project_context
		 WHERE deleted_at IS NULL
		 ORDER BY id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("context.ReadAll: query: %w", err)
	}
	defer rows.Close()

	entries := make([]models.Context, 0)
	for rows.Next() {
		var c models.Context
		var deletedAt sql.NullTime
		if err := rows.Scan(
			&c.ID, &c.Title, &c.Body, &c.CreatedBy,
			&c.CreatedAt, &c.UpdatedAt, &deletedAt,
		); err != nil {
			return nil, fmt.Errorf("context.ReadAll: scan: %w", err)
		}
		if deletedAt.Valid {
			t := deletedAt.Time
			c.DeletedAt = &t
		}
		entries = append(entries, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("context.ReadAll: rows: %w", err)
	}

	return entries, nil
}

// Read fetches a single project_context entry by ID.
// Returns ErrNotFound if the entry does not exist or has been soft-deleted.
func Read(id int, db *sql.DB) (*models.Context, error) {
	var c models.Context
	var deletedAt sql.NullTime

	err := db.QueryRow(
		`SELECT id, title, body, session_id, created_at, updated_at, deleted_at
		 FROM project_context
		 WHERE id = ? AND deleted_at IS NULL`,
		id,
	).Scan(
		&c.ID, &c.Title, &c.Body, &c.CreatedBy,
		&c.CreatedAt, &c.UpdatedAt, &deletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: context #%d not found", ErrNotFound, id)
		}
		return nil, fmt.Errorf("context.Read: %w", err)
	}

	if deletedAt.Valid {
		t := deletedAt.Time
		c.DeletedAt = &t
	}

	return &c, nil
}

// Update replaces the title and body of a context entry and records the acting session.
// Returns an error if title or body is empty, or if the entry does not exist.
func Update(id int, title, body string, actor *models.Session, db *sql.DB) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("context.Update: title must not be empty")
	}
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("context.Update: body must not be empty")
	}

	result, err := db.Exec(
		`UPDATE project_context
		 SET title = ?, body = ?, session_id = ?, updated_at = datetime('now')
		 WHERE id = ? AND deleted_at IS NULL`,
		title, body, actor.ID, id,
	)
	if err != nil {
		return fmt.Errorf("context.Update: exec: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("context.Update: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: context #%d not found", ErrNotFound, id)
	}

	return nil
}

// Delete soft-deletes a context entry by setting deleted_at.
// Returns ErrNotFound if the entry does not exist or is already soft-deleted.
func Delete(id int, db *sql.DB) error {
	result, err := db.Exec(
		`UPDATE project_context SET deleted_at = datetime('now') WHERE id = ? AND deleted_at IS NULL`,
		id,
	)
	if err != nil {
		return fmt.Errorf("context.Delete: exec: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("context.Delete: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: context #%d not found", ErrNotFound, id)
	}

	return nil
}
