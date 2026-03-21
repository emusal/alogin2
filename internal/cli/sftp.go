package cli

import (
	"context"
	"fmt"
	"path/filepath"

	internalssh "github.com/emusal/alogin2/internal/ssh"
	"github.com/spf13/cobra"
)

func newSFTPCmd() *cobra.Command {
	var putFile, getFile string

	cmd := &cobra.Command{
		Use:   "sftp [user@]host",
		Short: "SFTP file transfer",
		Long: `Connect to a host via SFTP.

Without flags, opens an interactive SFTP session (sftp prompt).
Use -p to upload a file, -g to download.

Examples:
  alogin sftp web-01                      # interactive sftp prompt
  alogin sftp web-01 -p ./deploy.tar.gz   # upload
  alogin sftp web-01 -g /var/log/app.log  # download`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			user, host := parseUserHost(args[0])

			srv, _ := database.Servers.GetByHost(ctx, host, user)
			if srv == nil {
				return fmt.Errorf("server %s not found in registry", host)
			}
			if user == "" {
				user = srv.User
			}

			hops, err := buildHopChain(ctx, srv, user, false)
			if err != nil {
				return err
			}

			// Interactive: hand off to the system sftp binary.
			if putFile == "" && getFile == "" {
				return internalssh.InteractiveSFTP(hops)
			}

			// File transfer: use the Go SFTP client via the established chain.
			chain, err := internalssh.DialChain(hops)
			if err != nil {
				return err
			}
			defer chain.CloseAll()

			sc, err := chain.Terminal().SFTPClient()
			if err != nil {
				return fmt.Errorf("sftp client: %w", err)
			}
			defer sc.Close()

			if putFile != "" {
				return sc.Upload(putFile, "./"+filepath.Base(putFile))
			}
			return sc.Download(getFile, "./"+filepath.Base(getFile))
		},
	}

	cmd.Flags().StringVarP(&putFile, "put", "p", "", "upload file to remote host")
	cmd.Flags().StringVarP(&getFile, "get", "g", "", "download file from remote host")
	return cmd
}
