package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/emusal/alogin2/internal/cluster"
	"github.com/emusal/alogin2/internal/model"
	"github.com/emusal/alogin2/internal/tui"
	"github.com/spf13/cobra"
)

func newClusterCmd() *cobra.Command {
	var mode string
	var tileX int
	var useGW bool

	cmd := &cobra.Command{
		Use:   "cluster [name-or-host]",
		Short: "Open cluster SSH sessions",
		Long: `Open simultaneous SSH sessions for all members of a cluster.
Run without arguments to open the interactive TUI cluster manager.

Mode options:
  tmux     - tmux split panes (cross-platform, default)
  iterm    - iTerm2 tabs/panes (macOS)
  terminal - Terminal.app windows (macOS)

Examples:
  alogin cluster
  alogin cluster prod-cluster
  alogin cluster prod-cluster --mode iterm
  alogin cluster prod-cluster --auto-gw`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if len(args) == 0 {
				return runTUIAt(ctx, tui.StartAtCluster)
			}
			name := args[0]

			cl, err := database.Clusters.GetByName(ctx, name)
			if err != nil || cl == nil {
				// Treat as a single host
				return fmt.Errorf("cluster %q not found", name)
			}

			var hosts []cluster.HostEntry
			for _, m := range cl.Members {
				srv, err := database.Servers.GetByID(ctx, m.ServerID)
				if err != nil || srv == nil {
					continue
				}
				user := srv.User
				if m.User != "" {
					user = m.User
				}
				pwd, _ := vlt.Get(vaultKey(srv))

				var hops []cluster.HopEntry
				if useGW && srv.GatewayID != nil {
					gwHops, _ := database.Gateways.HopsFor(ctx, srv.ID)
					for _, h := range gwHops {
						hopSrv, _ := database.Servers.GetByID(ctx, h.ServerID)
						if hopSrv != nil {
							hp, _ := vlt.Get(vaultKey(hopSrv))
							hops = append(hops, cluster.HopEntry{
								Host:     hopSrv.Host,
								Port:     hopSrv.EffectivePort(),
								User:     hopSrv.User,
								Password: hp,
							})
						}
					}
				}

				hosts = append(hosts, cluster.HostEntry{
					Host:     srv.Host,
					Port:     srv.EffectivePort(),
					User:     user,
					Password: pwd,
					Hops:     hops,
					UseGW:    useGW,
				})
			}

			if len(hosts) == 0 {
				return fmt.Errorf("no hosts in cluster %s", name)
			}

			binPath, _ := os.Executable()
			mgr := cluster.NewManager(mode, tileX, binPath)
			return mgr.Open(ctx, cl.Name, hosts)
		},
	}

	cmd.Flags().StringVar(&mode, "mode", "tmux", "session mode: tmux|iterm|terminal")
	cmd.Flags().IntVarP(&tileX, "tile-x", "x", 0, "number of columns for tiling (0=auto)")
	cmd.Flags().BoolVar(&useGW, "auto-gw", false, "route through gateways (like legacy 'cr')")
	cmd.AddCommand(newClusterAddCmd())
	cmd.AddCommand(newClusterListCmd())
	return cmd
}

func newClusterListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			clusters, err := database.Clusters.ListAll(ctx)
			if err != nil {
				return err
			}

			if format == "json" {
				type clusterJSON struct {
					ID          int64  `json:"id"`
					Name        string `json:"name"`
					MemberCount int    `json:"member_count"`
				}
				out := make([]clusterJSON, 0, len(clusters))
				for _, cl := range clusters {
					out = append(out, clusterJSON{ID: cl.ID, Name: cl.Name, MemberCount: len(cl.Members)})
				}
				return printJSON(out)
			}

			if len(clusters) == 0 {
				fmt.Println("No clusters registered.")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tMEMBERS")
			fmt.Fprintln(w, "----\t-------")
			for _, cl := range clusters {
				fmt.Fprintf(w, "%s\t%d\n", cl.Name, len(cl.Members))
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "output format: table|json")
	return cmd
}

func newClusterAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <host1> [host2...]",
		Short: "Add a new cluster with members",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			name := args[0]
			var members []model.ClusterMember

			for i, host := range args[1:] {
				srv, err := database.Servers.GetByHost(ctx, host, "")
				if err != nil || srv == nil {
					return fmt.Errorf("server %q not found in registry (you must add it first)", host)
				}
				members = append(members, model.ClusterMember{
					ServerID:    srv.ID,
					MemberOrder: i,
				})
			}

			if _, err := database.Clusters.Create(ctx, name, members); err != nil {
				return fmt.Errorf("failed to create cluster %q: %w", name, err)
			}

			fmt.Printf("Successfully created cluster %q with %d members.\n", name, len(members))
			return nil
		},
	}
}

