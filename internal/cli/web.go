package cli

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/emusal/alogin2/internal/web"
	"github.com/spf13/cobra"
)

func newWebCmd() *cobra.Command {
	var port int
	var noBrowser bool

	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start the alogin Web UI server",
		Long: `Start the embedded HTTP server for the alogin Web UI.

Access the UI at http://localhost:8484 (default).
The Web UI provides:
  - Server management (add, edit, delete)
  - Browser-based SSH terminal (xterm.js)
  - Cluster and gateway management

Press Ctrl+C to stop the server.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			srv := web.NewServer(database, vlt, port, !noBrowser)
			return srv.Run(ctx)
		},
	}

	cmd.Flags().IntVar(&port, "port", 8484, "port to listen on")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "don't open browser automatically")
	return cmd
}
