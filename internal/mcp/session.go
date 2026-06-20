package mcp

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/session"
)

// EnsureMCPSession creates a system session with the given role for MCP use.
// Does not write a .tkt/session file — DB-only session.
func EnsureMCPSession(db *sql.DB, role string) (*models.Session, error) {
	sess, err := session.CreateSystem(models.Role(role), db)
	if err != nil {
		return nil, fmt.Errorf("mcp: create session: %w", err)
	}
	return sess, nil
}

// resolveActingSession returns sess unchanged when sessionOverride is empty.
// When sessionOverride is non-empty, resolves directly via session.LoadByIDOrName,
// bypassing the fixed startup sess for this call only. Hard error on no-match or
// expired — never silently falls back to sess.
func resolveActingSession(sessionOverride string, sess *models.Session, db *sql.DB) (*models.Session, error) {
	if sessionOverride == "" {
		return sess, nil
	}
	resolved, err := session.LoadByIDOrName(sessionOverride, db)
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			return nil, fmt.Errorf("session %q: no matching session (check id or name)", sessionOverride)
		}
		if errors.Is(err, session.ErrExpiredSession) {
			return nil, fmt.Errorf("session %q: session has expired", sessionOverride)
		}
		return nil, fmt.Errorf("session %q: %w", sessionOverride, err)
	}
	return resolved, nil
}
