package cli

import "github.com/spf13/cobra"

// newNetCmd returns the "net" group command for network management.
func newNetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "net",
		Short: "Manage network resources (hosts, tunnels)",
		Long: `Manage local hostname mappings and persistent SSH port-forward tunnels.

Examples:
  alogin net hosts list
  alogin net hosts add web-01 10.0.0.1
  alogin net tunnel list
  alogin net tunnel start db-local`,
	}
	cmd.AddCommand(
		newHostsCmd(),
		newTunnelCmd(),
	)
	return cmd
}
