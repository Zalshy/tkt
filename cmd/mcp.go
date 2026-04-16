package cmd

import (
	"fmt"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/db"
	mcppkg "github.com/zalshy/tkt/internal/mcp"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/session"
)

var (
	mcpRole     string
	mcpReadonly bool
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server (stdio transport)",
	RunE:  runMCP,
}

func init() {
	mcpCmd.Flags().StringVar(&mcpRole, "role", "implementer", "session role for write tools (architect|implementer)")
	mcpCmd.Flags().BoolVar(&mcpReadonly, "readonly", false, "register read-only tools only")
	rootCmd.AddCommand(mcpCmd)
}

func runMCP(cmd *cobra.Command, args []string) error {
	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("mcp: open db: %w", err)
	}
	defer database.Close()

	var mcpSess *models.Session
	if !mcpReadonly {
		mcpSess, err = mcppkg.EnsureMCPSession(database, mcpRole)
		if err != nil {
			return err
		}
		defer session.ExpireByID(mcpSess.ID, database)
	}

	s := mcppkg.NewServer(root, database, mcpSess)
	return server.ServeStdio(s)
}
