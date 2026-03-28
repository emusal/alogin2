package cli

import "github.com/spf13/cobra"

// newAccessCmd returns the "access" group command for connectivity operations.
func newAccessCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "access",
		Short: "Connect to remote hosts (SSH, SFTP, FTP, SSHFS, cluster)",
		Long: `Connectivity commands: SSH, SFTP, FTP, SSHFS mount, and cluster sessions.

Examples:
  alogin access ssh admin@web-01
  alogin access sftp admin@web-01
  alogin access cluster prod-cluster
  alogin access cluster list`,
	}

	// Canonical SSH subcommand — same implementation as the legacy 'connect' command.
	// Aliases t and r are preserved on this subcommand for muscle memory.
	sshCmd := newConnectCmd()
	sshCmd.Use = "ssh [user@]host..."
	sshCmd.Short = "Connect via SSH"

	cmd.AddCommand(
		sshCmd,
		newSFTPCmd(),
		newFTPCmd(),
		newMountCmd(),
		newClusterCmd(),
		newPluginCmd(),
	)
	return cmd
}
