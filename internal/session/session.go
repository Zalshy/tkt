package session

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/project"
	rolepkg "github.com/zalshy/tkt/internal/role"
)

// ErrNoSession is returned by LoadActive when the .tkt/session file is absent.
var ErrNoSession = errors.New("no active session")

// ErrExpiredSession is returned by LoadActive when the session's expired_at is set.
// This is a distinct sentinel from ErrNoSession — both appear in logs and must be
// distinguishable by humans and by errors.Is callers.
var ErrExpiredSession = errors.New("session has expired")

// maxRetries is the number of times Create will retry on a PK collision before
// falling back to appending a hex suffix.
const maxRetries = 5

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
		return nil, ErrExpiredSession
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

// Create generates a new session ID, inserts a row into the sessions table, and
// writes the bare session ID to the .tkt/session file.
//
// name is the user-supplied session name (may be empty for random word). If non-empty,
// it must already have been validated by ValidateName.
//
// Collision strategy: attempt insertSession directly. On PRIMARY KEY constraint error,
// generate a new ID and retry up to maxRetries times. After maxRetries failures,
// append a randomHex4 suffix and do one final insert.
func Create(role models.Role, name string, db *sql.DB, root string) (*models.Session, error) {
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
		Role:          role,
		EffectiveRole: base,
	}

	// Insert-and-catch collision retry loop.
	// On PK constraint error, generate a fresh random word and retry.
	id := GenerateID(name)
	var insertErr error
	for i := 0; i < maxRetries; i++ {
		s.ID = id
		s.Name = id
		insertErr = insertSession(db, &s)
		if insertErr == nil {
			break
		}
		if !isPKConstraintError(insertErr) {
			return nil, fmt.Errorf("Create: %w", insertErr)
		}
		// Collision — only retry with random words (not user-supplied names).
		id = GenerateID("")
	}

	// After maxRetries failures, fall back to word + hex suffix.
	if insertErr != nil {
		if !isPKConstraintError(insertErr) {
			return nil, fmt.Errorf("Create: %w", insertErr)
		}
		id = GenerateID("") + "-" + randomHex4()
		s.ID = id
		s.Name = id
		if err := insertSession(db, &s); err != nil {
			return nil, fmt.Errorf("Create: %w", err)
		}
	}

	// Write the bare ID with no newline to .tkt/session.
	// LoadActive applies TrimSpace on read, but we intentionally write no trailing bytes.
	sessionFile := project.SessionFile(root)
	if err := os.WriteFile(sessionFile, []byte(id), 0o644); err != nil {
		// The DB row was already inserted — this orphan row is acceptable; tkt cleanup handles it.
		return nil, fmt.Errorf("Create: write session file: %w", err)
	}

	return &s, nil
}

// CreateSystem inserts a session row for a built-in system role (e.g. monitor)
// WITHOUT writing to the .tkt/session file. Used by tkt monitor to own its
// own session without displacing any user session.
// Collision retry strategy mirrors Create.
func CreateSystem(role models.Role, db *sql.DB) (*models.Session, error) {
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
		Role:          role,
		EffectiveRole: base,
	}

	id := GenerateID("")
	var insertErr error
	for i := 0; i < maxRetries; i++ {
		s.ID = id
		s.Name = id
		insertErr = insertSession(db, &s)
		if insertErr == nil {
			break
		}
		if !isPKConstraintError(insertErr) {
			return nil, fmt.Errorf("CreateSystem: %w", insertErr)
		}
		id = GenerateID("")
	}

	if insertErr != nil {
		if !isPKConstraintError(insertErr) {
			return nil, fmt.Errorf("CreateSystem: %w", insertErr)
		}
		id = GenerateID("") + "-" + randomHex4()
		s.ID = id
		s.Name = id
		if err := insertSession(db, &s); err != nil {
			return nil, fmt.Errorf("CreateSystem: %w", err)
		}
	}

	// Deliberately does NOT write a session file — system session is DB-only.
	return &s, nil
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
func isPKConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "PRIMARY KEY constraint failed")
}
