package role

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/zalshy/tkt/internal/models"
)

// Sentinel errors returned by package functions.
var (
	ErrAlreadyExists = errors.New("role already exists")
	ErrBuiltIn       = errors.New("cannot modify a built-in role")
	ErrInUse         = errors.New("role is in use by one or more active sessions")
	ErrNotFound      = errors.New("role not found")
)

// validNameRe enforces the name pattern: starts with a lowercase letter,
// followed by lowercase letters, digits, or underscores.
var validNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Role maps to a row in the roles table.
type Role struct {
	Name      string
	BaseRole  string
	IsBuiltin bool
	CreatedAt time.Time
}

// Exists reports whether a role with the given name exists in the database.
func Exists(name string, db *sql.DB) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM roles WHERE name = ?`, name).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("role.Exists: %w", err)
	}
	return count > 0, nil
}

// Create inserts a new user-defined role.
//
// Validation order:
//  1. name length (2–64 chars)
//  2. name pattern ([a-z][a-z0-9_]*)
//  3. baseRole must be "architect" or "implementer"
//  4. built-in guard (name must not equal "architect" or "implementer")
//  5. duplicate check
//  6. INSERT
func Create(name, baseRole string, db *sql.DB) error {
	if len(name) < 2 || len(name) > 64 {
		return fmt.Errorf("role.Create: name must be 2–64 characters")
	}
	if !validNameRe.MatchString(name) {
		return fmt.Errorf("role.Create: name must match [a-z][a-z0-9_]*")
	}
	if baseRole != "architect" && baseRole != "implementer" {
		return fmt.Errorf("role.Create: baseRole must be \"architect\" or \"implementer\"")
	}
	if name == "architect" || name == "implementer" {
		return fmt.Errorf("role.Create: %w", ErrBuiltIn)
	}
	exists, err := Exists(name, db)
	if err != nil {
		return fmt.Errorf("role.Create: %w", err)
	}
	if exists {
		return fmt.Errorf("role.Create: %w", ErrAlreadyExists)
	}
	if _, err := db.Exec(`INSERT INTO roles (name, base_role) VALUES (?, ?)`, name, baseRole); err != nil {
		return fmt.Errorf("role.Create: insert: %w", err)
	}
	return nil
}

// List returns all roles ordered by name ASC.
// Returns nil (not an empty slice) when there are no roles.
func List(db *sql.DB) ([]Role, error) {
	rows, err := db.Query(`SELECT name, base_role, is_builtin, created_at FROM roles ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("role.List: query: %w", err)
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		var r Role
		var isBuiltin int
		if err := rows.Scan(&r.Name, &r.BaseRole, &isBuiltin, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("role.List: scan: %w", err)
		}
		r.IsBuiltin = isBuiltin != 0
		roles = append(roles, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("role.List: rows: %w", err)
	}
	return roles, nil
}

// Delete removes a user-defined role by name.
//
// Returns ErrNotFound if the role does not exist, ErrBuiltIn if it is a
// built-in role, or ErrInUse if an active (non-expired) session holds it.
func Delete(name string, db *sql.DB) error {
	var isBuiltin int
	err := db.QueryRow(`SELECT is_builtin FROM roles WHERE name = ?`, name).Scan(&isBuiltin)
	if err == sql.ErrNoRows {
		return fmt.Errorf("role.Delete: %w", ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("role.Delete: lookup: %w", err)
	}
	if isBuiltin == 1 {
		return fmt.Errorf("role.Delete: %w", ErrBuiltIn)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE role = ? AND expired_at IS NULL`, name).Scan(&count); err != nil {
		return fmt.Errorf("role.Delete: in-use check: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("role.Delete: %w", ErrInUse)
	}
	if _, err := db.Exec(`DELETE FROM roles WHERE name = ?`, name); err != nil {
		return fmt.Errorf("role.Delete: exec: %w", err)
	}
	return nil
}

// ResolveBase returns the base_role of the named role as a models.Role.
// Returns ErrNotFound if no role with that name exists.
func ResolveBase(name string, db *sql.DB) (models.Role, error) {
	var baseRole string
	err := db.QueryRow(`SELECT base_role FROM roles WHERE name = ?`, name).Scan(&baseRole)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("role.ResolveBase: %w", ErrNotFound)
	}
	if err != nil {
		return "", fmt.Errorf("role.ResolveBase: %w", err)
	}
	return models.Role(baseRole), nil
}
