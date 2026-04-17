package cli

import "github.com/spf13/cobra"

// newServerGroupCmd returns the "server" group command.
// This is the canonical location for server registry management.
func newServerGroupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage servers in the registry",
		Long: `Manage the server registry: list, add, show, delete, and manage credentials.

Examples:
  alogin server list
  alogin server add --host web-01 --user admin
  alogin server show admin@web-01
  alogin server delete admin@web-01`,
	}
	cmd.AddCommand(
		newServerAddCmd(),
		newServerListCmd(),
		newServerShowCmd(),
		newServerDeleteCmd(),
		newServerPasswdCmd(),
		newServerGetPwdCmd(),
		newAliasCmd(),
	)
	return cmd
}
