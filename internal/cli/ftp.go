package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func newFTPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ftp [user@]host",
		Short: "FTP connection (delegates to system ftp)",
		Long:  `Connect to a host via FTP using the system ftp client.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			user, host := parseUserHost(args[0])
			ftpArgs := []string{}
			if user != "" {
				ftpArgs = append(ftpArgs, "-u", user)
			}
			ftpArgs = append(ftpArgs, host)

			c := exec.Command("ftp", ftpArgs...)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				return fmt.Errorf("ftp %s: %w", host, err)
			}
			return nil
		},
	}
}
