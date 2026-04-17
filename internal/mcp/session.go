package mcp

import (
	"database/sql"
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
