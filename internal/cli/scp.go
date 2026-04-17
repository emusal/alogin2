package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	internalssh "github.com/emusal/alogin2/internal/ssh"
	"github.com/spf13/cobra"
)

// isLocalDir returns true if path exists and is a directory.
func isLocalDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// parseRemotePath splits "[user@]host:/path" into (user, host, path).
// The last ':' separates host from path so IPv6 addresses in brackets work.
func parseRemotePath(arg string) (user, host, path string, err error) {
	colonIdx := strings.LastIndex(arg, ":")
	if colonIdx < 0 {
		return "", "", "", fmt.Errorf("invalid remote path %q: expected [user@]host:/path", arg)
	}
	hostPart := arg[:colonIdx]
	path = arg[colonIdx+1:]
	if path == "" {
		path = "."
	}
	user, host = parseUserHost(hostPart)
	if host == "" {
		return "", "", "", fmt.Errorf("invalid remote path %q: missing host", arg)
	}
	return user, host, path, nil
}

// dialSFTP resolves host in the registry, builds the hop chain, and returns
// a connected SFTPClient. The caller must defer chain.CloseAll() and sc.Close().
func dialSFTP(ctx context.Context, user, host string) (*internalssh.ChainedClient, *internalssh.SFTPClient, error) {
	srv, _ := database.Servers.GetByHost(ctx, host, user)
	if srv == nil {
		return nil, nil, fmt.Errorf("server %s not found in registry", host)
	}
	if user == "" {
		user = srv.User
	}
	hops, err := buildHopChain(ctx, srv, user, "")
	if err != nil {
		return nil, nil, err
	}
	chain, err := internalssh.DialChain(hops)
	if err != nil {
		return nil, nil, err
	}
	sc, err := chain.Terminal().SFTPClient()
	if err != nil {
		chain.CloseAll()
		return nil, nil, fmt.Errorf("sftp client: %w", err)
	}
	return chain, sc, nil
}

func newScpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scp",
		Short: "Copy files to/from remote hosts via SFTP",
		Long: `Copy files between local and remote hosts using SFTP.

Source comes first, destination second — same convention as scp(1).

Upload:
  alogin scp push ./local/file.py web-01:/tmp/
  alogin scp push ./deploy.tar.gz admin@web-01:/opt/releases/

Download:
  alogin scp pull web-01:/var/log/app.log ./
  alogin scp pull admin@web-01:/etc/nginx/nginx.conf ./nginx.conf.bak`,
	}
	cmd.AddCommand(newScpPushCmd(), newScpPullCmd())
	return cmd
}

func newScpPushCmd() *cobra.Command {
	var recursive bool
	cmd := &cobra.Command{
		Use:   "push <local-path> <[user@]host:/remote-path>",
		Short: "Upload a local file or directory to a remote host",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			localPath := args[0]
			user, host, remotePath, err := parseRemotePath(args[1])
			if err != nil {
				return err
			}
			chain, sc, err := dialSFTP(ctx, user, host)
			if err != nil {
				return err
			}
			defer chain.CloseAll()
			defer sc.Close()

			if recursive || isLocalDir(localPath) {
				if !isLocalDir(localPath) {
					return fmt.Errorf("%s is not a directory (--recursive requires a directory source)", localPath)
				}
				// If remote path ends with '/', append the local dir name.
				if strings.HasSuffix(remotePath, "/") {
					remotePath = remotePath + filepath.Base(localPath)
				}
				return sc.UploadDir(localPath, remotePath)
			}
			// Single-file upload.
			if strings.HasSuffix(remotePath, "/") {
				remotePath = remotePath + filepath.Base(localPath)
			}
			return sc.Upload(localPath, remotePath)
		},
	}
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively upload a directory")
	return cmd
}

func newScpPullCmd() *cobra.Command {
	var recursive bool
	cmd := &cobra.Command{
		Use:   "pull <[user@]host:/remote-path> <local-path>",
		Short: "Download a remote file or directory to the local host",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			user, host, remotePath, err := parseRemotePath(args[0])
			if err != nil {
				return err
			}
			localPath := args[1]
			chain, sc, err := dialSFTP(ctx, user, host)
			if err != nil {
				return err
			}
			defer chain.CloseAll()
			defer sc.Close()

			if recursive {
				// If local path ends with '/', append the remote dir name.
				if strings.HasSuffix(localPath, "/") {
					localPath = localPath + filepath.Base(remotePath)
				}
				return sc.DownloadDir(remotePath, localPath)
			}
			// Single-file download.
			if strings.HasSuffix(localPath, "/") {
				localPath = localPath + filepath.Base(remotePath)
			}
			return sc.Download(remotePath, localPath)
		},
	}
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively download a directory")
	return cmd
}
