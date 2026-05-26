package session

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	dbpkg "github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/project"
	rolepkg "github.com/zalshy/tkt/internal/role"
	sqlite3 "modernc.org/sqlite"
	sqlite3lib "modernc.org/sqlite/lib"
)

// ErrNoSession is returned by LoadActive when the .tkt/session file is absent.
var ErrNoSession = errors.New("no active session")

// ErrExpiredSession is returned by LoadActive when the session's expired_at is set.
// This is a distinct sentinel from ErrNoSession — both appear in logs and must be
// distinguishable by humans and by errors.Is callers.
var ErrExpiredSession = errors.New("session has expired")

// LoadActive reads the active session from the .tkt/session file, looks it up in the
// DB, updates last_active, and returns the Session.
//
//   - Returns ErrNoSession when the session file does not exist.
//   - Returns ErrExpiredSession when expired_at is set on the session row.
//   - Returns a wrapped error (not ErrNoSession) when the file exists but the row is gone.
func LoadActive(root string, db *sql.DB) (*models.Session, error) {
	sessionFile := project.SessionFile(root)

	data, err := os.ReadFile(sessionFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNoSession
		}
		return nil, fmt.Errorf("LoadActive: read session file: %w", err)
	}

	id := strings.TrimSpace(string(data))
	if id == "" {
		return nil, ErrNoSession
	}

	var s models.Session
	var expiredAt sql.NullTime

	err = db.QueryRow(
		`SELECT id, role, name, created_at, last_active, expired_at
		 FROM sessions WHERE id = ?`,
		id,
	).Scan(&s.ID, &s.Role, &s.Name, &s.CreatedAt, &s.LastActive, &expiredAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("LoadActive: session %q not found in database (orphan file?)", id)
		}
		return nil, fmt.Errorf("LoadActive: query session: %w", err)
	}

	if expiredAt.Valid {
		t := expiredAt.Time
		s.ExpiredAt = &t
		return &s, ErrExpiredSession
	}

	if err := updateLastActive(db, id); err != nil {
		return nil, fmt.Errorf("LoadActive: %w", err)
	}

	// Refresh LastActive to reflect the update we just performed.
	s.LastActive = time.Now().UTC()

	base, err := rolepkg.ResolveBase(string(s.Role), db)
	if err != nil {
		return nil, fmt.Errorf("LoadActive: resolve base role: %w", err)
	}
	s.EffectiveRole = base

	return &s, nil
}

// Create generates a new ULID, inserts a row into the sessions table, and
// writes the bare session ULID to the .tkt/session file.
//
// name is the user-supplied session name (may be empty for random word). If non-empty,
// it must already have been validated by ValidateName.
func Create(role models.Role, name string, db *sql.DB, root string) (*models.Session, error) {
	if _, err := dbpkg.CleanupStaleSessions(db, false); err != nil {
		return nil, fmt.Errorf("Create: cleanup stale sessions: %w", err)
	}

	// Validate the role is registered before inserting.
	exists, err := rolepkg.Exists(string(role), db)
	if err != nil {
		return nil, fmt.Errorf("Create: check role: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("Create: role %q is not registered", role)
	}

	base, err := rolepkg.ResolveBase(string(role), db)
	if err != nil {
		return nil, fmt.Errorf("Create: resolve base role: %w", err)
	}

	s := models.Session{
		ID:            GenerateULID(),
		Role:          role,
		EffectiveRole: base,
	}

	if name != "" {
		s.Name = GenerateName(name)
		if err := insertSession(db, &s); err != nil {
			return nil, fmt.Errorf("Create: %w", err)
		}
	} else {
		baseName := GenerateName("")
		candidates, err := candidateNames(baseName)
		if err != nil {
			return nil, fmt.Errorf("Create: %w", err)
		}
		if err := insertWithNameFallback(&s, candidates, db, "Create"); err != nil {
			return nil, err
		}
	}

	// Write the bare ULID with no newline to .tkt/session.
	// LoadActive applies TrimSpace on read, but we intentionally write no trailing bytes.
	sessionFile := project.SessionFile(root)
	if err := os.WriteFile(sessionFile, []byte(s.ID), 0o644); err != nil {
		// Best-effort cleanup — delete the DB row we just inserted.
		_, _ = db.Exec(`DELETE FROM sessions WHERE id = ?`, s.ID)
		return nil, fmt.Errorf("Create: write session file: %w", err)
	}

	return &s, nil
}

// CreateSystem inserts a DB-only session row for a built-in system role (e.g. monitor)
// WITHOUT writing to the .tkt/session file. Used by tkt monitor to own its
// own session without displacing any user session.
func CreateSystem(role models.Role, db *sql.DB) (*models.Session, error) {
	if _, err := dbpkg.CleanupStaleSessions(db, false); err != nil {
		return nil, fmt.Errorf("CreateSystem: cleanup stale sessions: %w", err)
	}

	// Validate the role is registered before inserting.
	exists, err := rolepkg.Exists(string(role), db)
	if err != nil {
		return nil, fmt.Errorf("CreateSystem: check role: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("CreateSystem: role %q is not registered", role)
	}

	base, err := rolepkg.ResolveBase(string(role), db)
	if err != nil {
		return nil, fmt.Errorf("CreateSystem: resolve base role: %w", err)
	}

	s := models.Session{
		ID:            GenerateULID(),
		Role:          role,
		EffectiveRole: base,
	}
	baseName := GenerateName("")
	candidates, err := candidateNames(baseName)
	if err != nil {
		return nil, fmt.Errorf("CreateSystem: %w", err)
	}
	if err := insertWithNameFallback(&s, candidates, db, "CreateSystem"); err != nil {
		return nil, err
	}

	// Deliberately does NOT write a session file — system session is DB-only.
	return &s, nil
}

func candidateNames(base string) ([]string, error) {
	firstHex, err := randomHex4()
	if err != nil {
		return nil, err
	}
	secondHex, err := randomHex4()
	if err != nil {
		return nil, err
	}
	return []string{
		base + "-" + firstHex,
		base + "-" + secondHex,
	}, nil
}

func insertWithNameFallback(s *models.Session, candidates []string, db *sql.DB, op string) error {
	var lastErr error
	for _, candidate := range candidates {
		s.Name = candidate
		if err := insertSession(db, s); err != nil {
			lastErr = err
			if !isPKConstraintError(err) {
				return fmt.Errorf("%s: %w", op, err)
			}
			continue
		}
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("%s: %w", op, lastErr)
	}
	return fmt.Errorf("%s: no candidate session names generated", op)
}

// ExpireByID sets expired_at = datetime('now') for the session with the given ID.
// Returns nil if no row matched (idempotent).
func ExpireByID(id string, db *sql.DB) error {
	_, err := db.Exec(`UPDATE sessions SET expired_at = datetime('now') WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("ExpireByID: %w", err)
	}
	return nil
}

// isPKConstraintError reports whether err is a SQLite PRIMARY KEY / UNIQUE constraint violation.
// SQLite returns extended error codes (e.g. SQLITE_CONSTRAINT_PRIMARYKEY = 1555,
// SQLITE_CONSTRAINT_UNIQUE = 2067) — mask the upper bits to compare against the
// base SQLITE_CONSTRAINT code (19).
func isPKConstraintError(err error) bool {
	if err == nil {
		return false
	}
	var sqliteErr *sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code()&0xFF == sqlite3lib.SQLITE_CONSTRAINT
	}
	return false
}
