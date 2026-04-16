package cmd

import (
	"errors"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/role"
)

var roleLike string // --like flag for create subcommand

var roleCmd = &cobra.Command{
	Use:   "role",
	Short: "Manage roles",
}

var roleCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new role",
	Args:  cobra.ExactArgs(1),
	RunE:  runRoleCreate,
}

var roleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all roles",
	Args:  cobra.NoArgs,
	RunE:  runRoleList,
}

var roleDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a role",
	Args:  cobra.ExactArgs(1),
	RunE:  runRoleDelete,
}

func init() {
	roleCreateCmd.Flags().StringVar(&roleLike, "like", "", "base role: architect or implementer (required)")
	roleCreateCmd.MarkFlagRequired("like")

	roleCmd.AddCommand(roleCreateCmd)
	roleCmd.AddCommand(roleListCmd)
	roleCmd.AddCommand(roleDeleteCmd)
	rootCmd.AddCommand(roleCmd)
}

func runRoleCreate(cmd *cobra.Command, args []string) error {
	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("role create: open db: %w", err)
	}
	defer database.Close()

	if err := role.Create(args[0], roleLike, database); err != nil {
		return fmt.Errorf("role create: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Role '%s' created (behaves like %s).\n", args[0], roleLike)
	return nil
}

func runRoleList(cmd *cobra.Command, args []string) error {
	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("role list: open db: %w", err)
	}
	defer database.Close()

	roles, err := role.List(database)
	if err != nil {
		return fmt.Errorf("role list: %w", err)
	}

	if len(roles) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no roles)")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tBASE ROLE")
	for _, r := range roles {
		suffix := ""
		if r.IsBuiltin {
			suffix = "  (built-in)"
		}
		fmt.Fprintf(w, "%s\t%s%s\n", r.Name, r.BaseRole, suffix)
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("role list: flush: %w", err)
	}
	return nil
}

func runRoleDelete(cmd *cobra.Command, args []string) error {
	name := args[0]

	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("role delete: open db: %w", err)
	}
	defer database.Close()

	if err := role.Delete(name, database); err != nil {
		if errors.Is(err, role.ErrBuiltIn) {
			return fmt.Errorf("cannot delete built-in role '%s'", name)
		}
		if errors.Is(err, role.ErrInUse) {
			return fmt.Errorf("role '%s' is in use by one or more active sessions", name)
		}
		if errors.Is(err, role.ErrNotFound) {
			return fmt.Errorf("role '%s' not found", name)
		}
		return fmt.Errorf("role delete: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Role '%s' deleted.\n", name)
	return nil
}
