package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"

	"github.com/emusal/alogin2/internal/model"
	internalssh "github.com/emusal/alogin2/internal/ssh"
	"github.com/emusal/alogin2/internal/tunnel"
	"github.com/emusal/alogin2/internal/tui"
	"github.com/spf13/cobra"
)

func newTunnelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tunnel",
		Short: "Manage and run SSH port-forward tunnels",
		Long: `Manage persistent SSH port-forward tunnels backed by tmux sessions.

Tunnel configurations are stored in the database. Each tunnel can be
started as a detached tmux session that maintains the SSH connection.

Examples:
  alogin tunnel add db-local --server db.prod --local-port 5432 --remote-host db.prod --remote-port 5432
  alogin tunnel start db-local
  alogin tunnel list
  alogin tunnel stop db-local`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUIAt(context.Background(), tui.StartAtTunnel)
		},
	}

	cmd.AddCommand(
		newTunnelListCmd(),
		newTunnelAddCmd(),
		newTunnelEditCmd(),
		newTunnelRmCmd(),
		newTunnelStartCmd(),
		newTunnelStopCmd(),
		newTunnelStatusCmd(),
		newTunnelRunCmd(),
	)
	return cmd
}

func newTunnelListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tunnel configurations",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			tunnels, err := database.Tunnels.ListAll(ctx)
			if err != nil {
				return err
			}

			if format == "json" {
				type tunnelJSON struct {
					ID         int64  `json:"id"`
					Name       string `json:"name"`
					Server     string `json:"server"`
					Direction  string `json:"direction"`
					LocalHost  string `json:"local_host"`
					LocalPort  int    `json:"local_port"`
					RemoteHost string `json:"remote_host"`
					RemotePort int    `json:"remote_port"`
					AutoGW     bool   `json:"auto_gw"`
					Running    bool   `json:"running"`
				}
				out := make([]tunnelJSON, 0, len(tunnels))
				for _, t := range tunnels {
					srv, _ := database.Servers.GetByID(ctx, t.ServerID)
					srvHost := fmt.Sprintf("id=%d", t.ServerID)
					if srv != nil {
						srvHost = srv.Host
					}
					out = append(out, tunnelJSON{
						ID: t.ID, Name: t.Name, Server: srvHost,
						Direction:  string(t.Direction),
						LocalHost:  t.LocalHost, LocalPort: t.LocalPort,
						RemoteHost: t.RemoteHost, RemotePort: t.RemotePort,
						AutoGW:  t.AutoGW,
						Running: tunnel.IsRunning(t.Name),
					})
				}
				return printJSON(out)
			}

			if len(tunnels) == 0 {
				fmt.Println("No tunnels configured. Use 'alogin tunnel add' to create one.")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tSERVER\tDIR\tLOCAL\tREMOTE\tSTATUS")
			for _, t := range tunnels {
				srv, _ := database.Servers.GetByID(ctx, t.ServerID)
				srvLabel := fmt.Sprintf("(id=%d)", t.ServerID)
				if srv != nil {
					srvLabel = srv.Host
				}
				status := "stopped"
				if tunnel.IsRunning(t.Name) {
					status = "running"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s:%d\t%s:%d\t%s\n",
					t.Name, srvLabel, string(t.Direction),
					t.LocalHost, t.LocalPort,
					t.RemoteHost, t.RemotePort,
					status,
				)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "output format: table|json")
	return cmd
}

func newTunnelAddCmd() *cobra.Command {
	var (
		serverHost string
		dir        string
		localHost  string
		localPort  int
		remoteHost string
		remotePort int
		autoGW     bool
	)
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a tunnel configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			name := args[0]
			if dir != "L" && dir != "R" {
				return fmt.Errorf("--dir must be L or R")
			}
			if localPort <= 0 || remotePort <= 0 {
				return fmt.Errorf("--local-port and --remote-port must be positive")
			}
			srv, err := database.Servers.GetByHost(ctx, serverHost, "")
			if err != nil || srv == nil {
				return fmt.Errorf("server %q not found in registry", serverHost)
			}
			t := &model.Tunnel{
				Name:       name,
				ServerID:   srv.ID,
				Direction:  model.TunnelDirection(dir),
				LocalHost:  localHost,
				LocalPort:  localPort,
				RemoteHost: remoteHost,
				RemotePort: remotePort,
				AutoGW:     autoGW,
			}
			if err := database.Tunnels.Create(ctx, t); err != nil {
				return fmt.Errorf("create tunnel: %w", err)
			}
			fmt.Printf("Tunnel %q added (id=%d).\n", t.Name, t.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&serverHost, "server", "", "Server hostname (required)")
	cmd.Flags().StringVar(&dir, "dir", "L", "Direction: L (local forward) or R (remote forward)")
	cmd.Flags().StringVar(&localHost, "local-host", "127.0.0.1", "Local listen address")
	cmd.Flags().IntVar(&localPort, "local-port", 0, "Local port (required)")
	cmd.Flags().StringVar(&remoteHost, "remote-host", "", "Remote host (required)")
	cmd.Flags().IntVar(&remotePort, "remote-port", 0, "Remote port (required)")
	cmd.Flags().BoolVar(&autoGW, "auto-gw", false, "Follow gateway chain from server registry")
	_ = cmd.MarkFlagRequired("server")
	_ = cmd.MarkFlagRequired("local-port")
	_ = cmd.MarkFlagRequired("remote-host")
	_ = cmd.MarkFlagRequired("remote-port")
	return cmd
}

func newTunnelEditCmd() *cobra.Command {
	var (
		serverHost string
		dir        string
		localHost  string
		localPort  int
		remoteHost string
		remotePort int
		autoGW     bool
	)
	cmd := &cobra.Command{
		Use:   "edit <name>",
		Short: "Edit a tunnel configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			t, err := database.Tunnels.GetByName(ctx, args[0])
			if err != nil || t == nil {
				return fmt.Errorf("tunnel %q not found", args[0])
			}
			if cmd.Flags().Changed("server") {
				srv, err := database.Servers.GetByHost(ctx, serverHost, "")
				if err != nil || srv == nil {
					return fmt.Errorf("server %q not found in registry", serverHost)
				}
				t.ServerID = srv.ID
			}
			if cmd.Flags().Changed("dir") {
				if dir != "L" && dir != "R" {
					return fmt.Errorf("--dir must be L or R")
				}
				t.Direction = model.TunnelDirection(dir)
			}
			if cmd.Flags().Changed("local-host") {
				t.LocalHost = localHost
			}
			if cmd.Flags().Changed("local-port") {
				t.LocalPort = localPort
			}
			if cmd.Flags().Changed("remote-host") {
				t.RemoteHost = remoteHost
			}
			if cmd.Flags().Changed("remote-port") {
				t.RemotePort = remotePort
			}
			if cmd.Flags().Changed("auto-gw") {
				t.AutoGW = autoGW
			}
			if err := database.Tunnels.Update(ctx, t); err != nil {
				return fmt.Errorf("update tunnel: %w", err)
			}
			fmt.Printf("Tunnel %q updated.\n", t.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&serverHost, "server", "", "Server hostname")
	cmd.Flags().StringVar(&dir, "dir", "", "Direction: L or R")
	cmd.Flags().StringVar(&localHost, "local-host", "", "Local listen address")
	cmd.Flags().IntVar(&localPort, "local-port", 0, "Local port")
	cmd.Flags().StringVar(&remoteHost, "remote-host", "", "Remote host")
	cmd.Flags().IntVar(&remotePort, "remote-port", 0, "Remote port")
	cmd.Flags().BoolVar(&autoGW, "auto-gw", false, "Follow gateway chain")
	return cmd
}

func newTunnelRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm <name>",
		Short:   "Remove a tunnel configuration",
		Aliases: []string{"delete", "del"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			t, err := database.Tunnels.GetByName(ctx, args[0])
			if err != nil || t == nil {
				return fmt.Errorf("tunnel %q not found", args[0])
			}
			// Stop the tunnel if it is running.
			if tunnel.IsRunning(t.Name) {
				if err := tunnel.Stop(t.Name); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not stop tunnel: %v\n", err)
				}
			}
			if err := database.Tunnels.Delete(ctx, t.ID); err != nil {
				return fmt.Errorf("delete tunnel: %w", err)
			}
			fmt.Printf("Tunnel %q removed.\n", t.Name)
			return nil
		},
	}
}

func newTunnelStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <name>",
		Short: "Start a tunnel in a detached tmux session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			t, err := database.Tunnels.GetByName(ctx, args[0])
			if err != nil || t == nil {
				return fmt.Errorf("tunnel %q not found", args[0])
			}
			binPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve binary path: %w", err)
			}
			if err := tunnel.Start(t.Name, binPath); err != nil {
				return err
			}
			fmt.Printf("Tunnel %q started (tmux session: %s).\n", t.Name, tunnel.SessionName(t.Name))
			return nil
		},
	}
}

func newTunnelStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <name>",
		Short: "Stop a running tunnel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			t, err := database.Tunnels.GetByName(ctx, args[0])
			if err != nil || t == nil {
				return fmt.Errorf("tunnel %q not found", args[0])
			}
			if err := tunnel.Stop(t.Name); err != nil {
				return err
			}
			fmt.Printf("Tunnel %q stopped.\n", t.Name)
			return nil
		},
	}
}

func newTunnelStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <name>",
		Short: "Show tunnel running status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			t, err := database.Tunnels.GetByName(ctx, args[0])
			if err != nil || t == nil {
				return fmt.Errorf("tunnel %q not found", args[0])
			}
			if tunnel.IsRunning(t.Name) {
				fmt.Printf("running  (session: %s)\n", tunnel.SessionName(t.Name))
			} else {
				fmt.Println("stopped")
			}
			return nil
		},
	}
}

// newTunnelRunCmd is the internal command invoked inside the tmux session.
// It maintains the SSH tunnel in the foreground until a signal is received.
func newTunnelRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "run <name>",
		Short:  "Run a tunnel in the foreground (called internally by tmux)",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			t, err := database.Tunnels.GetByName(ctx, args[0])
			if err != nil {
				return fmt.Errorf("lookup tunnel: %w", err)
			}
			if t == nil {
				return fmt.Errorf("tunnel %q not found", args[0])
			}

			srv, err := database.Servers.GetByID(ctx, t.ServerID)
			if err != nil || srv == nil {
				return fmt.Errorf("server id=%d not found", t.ServerID)
			}

			hops, err := buildHopChain(ctx, srv, srv.User, t.AutoGW)
			if err != nil {
				return fmt.Errorf("build hop chain: %w", err)
			}

			chain, err := internalssh.DialChain(hops)
			if err != nil {
				return fmt.Errorf("dial chain: %w", err)
			}
			defer chain.CloseAll()

			spec := internalssh.TunnelSpec{
				LocalHost:  t.LocalHost,
				LocalPort:  t.LocalPort,
				RemoteHost: t.RemoteHost,
				RemotePort: t.RemotePort,
			}

			client := chain.Terminal()
			if t.Direction == model.TunnelLocal {
				if err := client.ForwardLocal(ctx, spec); err != nil {
					return fmt.Errorf("setup local forward: %w", err)
				}
				fmt.Fprintf(os.Stderr, "[tunnel] %s: -L %s:%d:%s:%d active\n",
					t.Name, t.LocalHost, t.LocalPort, t.RemoteHost, t.RemotePort)
			} else {
				if err := client.ForwardRemote(ctx, spec); err != nil {
					return fmt.Errorf("setup remote forward: %w", err)
				}
				fmt.Fprintf(os.Stderr, "[tunnel] %s: -R %s:%d:%s:%d active\n",
					t.Name, t.RemoteHost, t.RemotePort, t.LocalHost, t.LocalPort)
			}

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
			<-sigChan
			fmt.Fprintf(os.Stderr, "[tunnel] %s: shutting down\n", t.Name)
			return nil
		},
	}
}
