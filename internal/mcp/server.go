package mcp

import (
	"database/sql"

	"github.com/mark3labs/mcp-go/server"
	"github.com/zalshy/tkt/internal/models"
)

// NewServer creates the MCP server and registers all tools.
// sess may be nil when --readonly is set (no write tools registered).
func NewServer(root string, db *sql.DB, sess *models.Session) *server.MCPServer {
	s := server.NewMCPServer("tkt", "0.1.0",
		server.WithToolCapabilities(false),
	)

	// Read tools — always registered
	addReadTools(s, root, db)

	// Write + admin tools — only when sess is non-nil
	if sess != nil {
		addWriteTools(s, root, db, sess)
		addAdminTools(s, root, db, sess)
	}

	return s
}
