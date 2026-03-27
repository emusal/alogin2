package cli

import (
	"context"
	"fmt"
	"os"
	"sync"
	"text/tabwriter"

	"github.com/emusal/alogin2/internal/cluster"
	"github.com/emusal/alogin2/internal/mcp"
	"github.com/emusal/alogin2/internal/model"
	"github.com/emusal/alogin2/internal/tui"
	"github.com/spf13/cobra"
)

func newClusterCmd() *cobra.Command {
	var mode string
	var tileX int
	var useGW bool
	var command string
	var format string

	cmd := &cobra.Command{
		Use:   "cluster [name-or-host]",
		Short: "Open cluster SSH sessions",
		Long: `Open simultaneous SSH sessions for all members of a cluster.
Run without arguments to open the interactive TUI cluster manager.

When --cmd is given, commands are executed on all members in parallel and
results are printed to stdout (no tmux session is opened).

Mode options (interactive sessions only):
  tmux     - tmux split panes (cross-platform, default)
  iterm    - iTerm2 tabs/panes (macOS)
  terminal - Terminal.app windows (macOS)

Examples:
  alogin cluster
  alogin cluster prod-cluster
  alogin cluster prod-cluster --mode iterm
  alogin cluster prod-cluster --auto-gw
  alogin cluster prod-cluster --cmd "uptime"
  alogin cluster prod-cluster --cmd "df -h" --format json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if len(args) == 0 {
				return runTUIAt(ctx, tui.StartAtCluster)
			}
			name := args[0]

			cl, err := database.Clusters.GetByName(ctx, name)
			if err != nil || cl == nil {
				return fmt.Errorf("cluster %q not found", name)
			}

			// --cmd: parallel exec, print results, no tmux
			if command != "" {
				return runClusterCmd(ctx, cl, command, useGW, format)
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
	cmd.Flags().StringVarP(&command, "cmd", "c", "", "command to run on each member (parallel exec, no tmux)")
	cmd.Flags().StringVar(&format, "format", "table", "output format when using --cmd: table|json")
	cmd.AddCommand(newClusterAddCmd())
	cmd.AddCommand(newClusterListCmd())
	return cmd
}

// clusterCmdResult is the result of running --cmd on one cluster member.
type clusterCmdResult struct {
	Host     string `json:"host"`
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// runClusterCmd executes a command on all cluster members in parallel and prints results.
func runClusterCmd(ctx context.Context, cl *model.Cluster, command string, useGW bool, format string) error {
	results := make([]clusterCmdResult, len(cl.Members))
	var wg sync.WaitGroup

	for i, m := range cl.Members {
		wg.Add(1)
		go func(idx int, member model.ClusterMember) {
			defer wg.Done()
			srv, err := database.Servers.GetByID(ctx, member.ServerID)
			if err != nil || srv == nil {
				results[idx] = clusterCmdResult{Host: fmt.Sprintf("id=%d", member.ServerID), Error: "server not found"}
				return
			}
			results[idx].Host = srv.Host

			cmdResults, err := mcp.ExecOnServer(ctx, database, vlt, mcp.ExecRequest{
				ServerID: srv.ID,
				Commands: []string{command},
				AutoGW:   useGW,
			})
			if err != nil {
				results[idx].Error = err.Error()
				return
			}
			if len(cmdResults) > 0 {
				results[idx].Output = cmdResults[0].Output
				results[idx].ExitCode = cmdResults[0].ExitCode
				if cmdResults[0].Error != "" {
					results[idx].Error = cmdResults[0].Error
				}
			}
		}(i, m)
	}
	wg.Wait()

	if format == "json" {
		return printJSON(results)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, r := range results {
		if r.Error != "" {
			fmt.Fprintf(w, "=== %s (error: %s) ===\n", r.Host, r.Error)
			continue
		}
		fmt.Fprintf(w, "=== %s (exit %d) ===\n", r.Host, r.ExitCode)
		fmt.Fprint(w, r.Output)
		if len(r.Output) > 0 && r.Output[len(r.Output)-1] != '\n' {
			fmt.Fprintln(w)
		}
	}
	return w.Flush()
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

