package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newAuthCmd returns the "auth" group command for credentials and routing.
func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage credentials and routing (gateways, aliases, vault)",
		Long: `Manage gateway routes, host aliases, and vault credentials.

Examples:
  alogin auth gateway list
  alogin auth gateway add corp-gw gw.corp.com
  alogin auth alias add prod admin@web-01
  alogin auth alias list`,
	}
	cmd.AddCommand(
		newGatewayCmd(),
		newAliasCmd(),
		newAuthVaultStubCmd(),
	)
	return cmd
}

// newAuthVaultStubCmd is a Phase 2 stub for vault operations.
func newAuthVaultStubCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "vault",
		Short: "Vault backend operations (coming in Phase 2)",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("auth vault: not yet implemented (Phase 2)")
			fmt.Println("Use the ALOGIN_VAULT_PASS environment variable to unlock the age vault.")
			return cmd.Help()
		},
	}
}
