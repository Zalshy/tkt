package session

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/project"
	rolepkg "github.com/zalshy/tkt/internal/role"
)

// LoadByName returns the active session with the given name.
// Returns ErrNoSession if no active (non-expired) session with that name exists.
func LoadByName(name string, db *sql.DB) (*models.Session, error) {
	var s models.Session
	err := db.QueryRow(
		`SELECT id, role, name, created_at, last_active
		 FROM sessions WHERE name = ? AND expired_at IS NULL`,
		name,
	).Scan(&s.ID, &s.Role, &s.Name, &s.CreatedAt, &s.LastActive)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, fmt.Errorf("LoadByName: %w", err)
	}
	base, err := rolepkg.ResolveBase(string(s.Role), db)
	if err != nil {
		return nil, fmt.Errorf("LoadByName: resolve base role: %w", err)
	}
	s.EffectiveRole = base
	return &s, nil
}

// insertSession inserts a new session row into the sessions table.
func insertSession(db *sql.DB, s *models.Session) error {
	_, err := db.Exec(
		`INSERT INTO sessions (id, role, name, created_at, last_active)
		 VALUES (?, ?, ?, datetime('now'), datetime('now'))`,
		s.ID, string(s.Role), s.Name,
	)
	if err != nil {
		return fmt.Errorf("insertSession: %w", err)
	}
	return nil
}

// End marks the current active session as expired by setting expired_at = NOW().
// Reads the session ID from the .tkt/session file. Returns ErrNoSession when
// no session file exists or the file is empty.
// Calling End on an already-expired session is idempotent — it simply updates expired_at again.
func End(root string, db *sql.DB) (sessionID string, err error) {
	sessionFile := project.SessionFile(root)

	data, err := os.ReadFile(sessionFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNoSession
		}
		return "", fmt.Errorf("End: read session file: %w", err)
	}

	id := strings.TrimSpace(string(data))
	if id == "" {
		return "", ErrNoSession
	}

	result, err := db.Exec(
		`UPDATE sessions SET expired_at = datetime('now') WHERE id = ? AND expired_at IS NULL`,
		id,
	)
	if err != nil {
		return "", fmt.Errorf("End: update session: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("End: rows affected: %w", err)
	}
	if n == 0 {
		var exists int
		_ = db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id = ?`, id).Scan(&exists)
		if exists > 0 {
			return id, nil // already expired — idempotent
		}
		return "", fmt.Errorf("End: session %q not found in database", id)
	}

	return id, nil
}

// updateLastActive sets last_active = NOW() for the session with the given ID.
func updateLastActive(db *sql.DB, id string) error {
	result, err := db.Exec(`UPDATE sessions SET last_active = datetime('now') WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("updateLastActive: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("updateLastActive: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("updateLastActive: session %q not found or already deleted", id)
	}
	return nil
}
