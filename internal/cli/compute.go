package cli

import "github.com/spf13/cobra"

// newComputeCmd returns the "compute" group command.
// This is the canonical location for server registry management.
func newComputeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compute",
		Short: "Manage servers (compute resources)",
		Long: `Manage the server registry: list, add, show, delete, and manage credentials.

Examples:
  alogin compute list
  alogin compute add --host web-01 --user admin
  alogin compute show admin@web-01
  alogin compute delete admin@web-01`,
	}
	cmd.AddCommand(
		newServerAddCmd(),
		newServerListCmd(),
		newServerShowCmd(),
		newServerDeleteCmd(),
		newServerPasswdCmd(),
		newServerGetPwdCmd(),
	)
	return cmd
}
