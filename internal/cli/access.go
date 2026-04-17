package cli

import "github.com/spf13/cobra"

// newSSHGroupCmd returns the "ssh" group command for connectivity operations.
func newSSHGroupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Connect to remote hosts (SSH, SFTP, FTP, SSHFS, cluster)",
		Long: `Connectivity commands: SSH, SFTP, FTP, SSHFS mount, and cluster sessions.

Examples:
  alogin ssh connect admin@web-01
  alogin ssh sftp admin@web-01
  alogin ssh cluster prod-cluster
  alogin ssh cluster list`,
	}

	// Canonical SSH connect subcommand.
	sshCmd := newConnectCmd()
	sshCmd.Use = "connect [user@]host..."
	sshCmd.Short = "Connect via SSH"

	cmd.AddCommand(
		sshCmd,
		newSFTPCmd(),
		newFTPCmd(),
		newMountCmd(),
		newClusterCmd(),
		newPluginCmd(),
		newSessionCmd(),
	)
	return cmd
}
