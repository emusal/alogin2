package cli

import "github.com/spf13/cobra"

// newNetCmd returns the "net" group command for network management.
func newNetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "net",
		Short: "Manage network resources (hosts, tunnels, gateways, profiles)",
		Long: `Manage local hostname mappings, persistent SSH tunnels, gateway routes, and network profiles.

Examples:
  alogin net hosts list
  alogin net hosts add web-01 10.0.0.1
  alogin net tunnel list
  alogin net tunnel start db-local
  alogin net gateway add corp-gw gw.corp.com
  alogin net profile use home`,
	}
	cmd.AddCommand(
		newHostsCmd(),
		newTunnelCmd(),
		newGatewayCmd(),
		newProfileCmd(),
	)
	return cmd
}
