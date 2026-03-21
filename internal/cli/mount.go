package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	internalssh "github.com/emusal/alogin2/internal/ssh"
	"github.com/spf13/cobra"
)

func newMountCmd() *cobra.Command {
	var unmount bool

	cmd := &cobra.Command{
		Use:   "mount [user@]host[:remotePath] [localPath]",
		Short: "Mount remote filesystem via SSHFS",
		Long: `Mount a remote directory via SSHFS.

Examples:
  alogin mount web-01                          # mounts / at ~/mnt/web-01
  alogin mount web-01:/var/www ~/mnt/www       # mounts /var/www at ~/mnt/www
  alogin mount web-01 --unmount               # unmount`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Parse host:path
			target := args[0]
			remotePath := "/"
			userHost := target
			if idx := findColon(target); idx > 0 {
				userHost = target[:idx]
				remotePath = target[idx+1:]
			}

			user, host := parseUserHost(userHost)

			if unmount {
				localPath := defaultMountPath(host)
				if len(args) > 1 {
					localPath = args[1]
				}
				return internalssh.Unmount(localPath)
			}

			srv, _ := database.Servers.GetByHost(ctx, host, user)
			if srv == nil {
				return fmt.Errorf("server %s not found", host)
			}
			if user == "" {
				user = srv.User
			}

			localPath := defaultMountPath(host)
			if len(args) > 1 {
				localPath = args[1]
			}

			pwd, _ := vlt.Get(vaultKey(srv))
			hopCfg := internalssh.HopConfig{
				Host:     srv.Host,
				Port:     srv.EffectivePort(),
				User:     user,
				Password: pwd,
			}
			return internalssh.Mount(hopCfg, remotePath, localPath)
		},
	}

	cmd.Flags().BoolVar(&unmount, "unmount", false, "unmount the SSHFS volume")
	return cmd
}

func defaultMountPath(host string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "mnt", host)
}

// findColon finds the last colon in s that is followed by a '/' or a digit.
func findColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}
