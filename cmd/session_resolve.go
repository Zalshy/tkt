package cmd

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/session"
)

// resolveSession is the single chokepoint every command uses to resolve the acting
// session, instead of calling session.LoadActive directly.
//
//   - When the --session flag is set (sessionOverride != ""), resolves the actor
//     directly from the DB by id-or-name via session.LoadByIDOrName, bypassing the
//     .tkt/session file pointer for this invocation. A value that matches no session,
//     or matches an expired one, hard-errors — it never silently falls back to the file.
//   - When the flag is absent, falls through unchanged to session.LoadActive(root, db) —
//     existing file-pointer behavior, zero change.
//
// Any new command that needs the acting session should call this instead of
// session.LoadActive directly, so --session support extends by convention.
func resolveSession(root string, db *sql.DB) (*models.Session, error) {
	if sessionOverride != "" {
		sess, err := session.LoadByIDOrName(sessionOverride, db)
		if err != nil {
			if errors.Is(err, session.ErrSessionNotFound) {
				return nil, fmt.Errorf("--session %q: no matching session (check id or name)", sessionOverride)
			}
			if errors.Is(err, session.ErrExpiredSession) {
				// Same user-facing message as the file-pointer expired-session path —
				// callers should not have to learn two error vocabularies.
				return nil, errors.New(msgExpiredSession)
			}
			return nil, fmt.Errorf("--session %q: %w", sessionOverride, err)
		}
		return sess, nil
	}
	return session.LoadActive(root, db)
}
