package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/emusal/alogin2/internal/tui"
	"github.com/spf13/cobra"
)

func newGatewayCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gateway",
		Short: "Manage gateway routes",
		Long:  "Manage gateway routes. Run without subcommand to open the interactive TUI.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUIAt(context.Background(), tui.StartAtGateway)
		},
	}
	cmd.AddCommand(newGatewayAddCmd(), newGatewayListCmd(), newGatewayShowCmd(), newGatewayDeleteCmd())
	return cmd
}

func newGatewayAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <hop1> [hop2 ...]",
		Short: "Add a gateway route",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			name := args[0]
			var hopIDs []int64
			for _, hopHost := range args[1:] {
				srv, err := database.Servers.GetByHost(ctx, hopHost, "")
				if err != nil || srv == nil {
					return fmt.Errorf("hop server %q not found in registry", hopHost)
				}
				hopIDs = append(hopIDs, srv.ID)
			}
			gw, err := database.Gateways.Create(ctx, name, hopIDs)
			if err != nil {
				return err
			}
			fmt.Printf("Added gateway %s (id=%d, %d hops)\n", gw.Name, gw.ID, len(gw.Hops))
			return nil
		},
	}
}

func newGatewayListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all gateway routes",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			gws, err := database.Gateways.ListAll(ctx)
			if err != nil {
				return err
			}

			if format == "json" {
				type gatewayJSON struct {
					ID   int64    `json:"id"`
					Name string   `json:"name"`
					Hops []string `json:"hops"`
				}
				out := make([]gatewayJSON, 0, len(gws))
				for _, gw := range gws {
					hops := make([]string, 0, len(gw.Hops))
					for _, h := range gw.Hops {
						srv, _ := database.Servers.GetByID(ctx, h.ServerID)
						if srv != nil {
							hops = append(hops, srv.Host)
						}
					}
					out = append(out, gatewayJSON{ID: gw.ID, Name: gw.Name, Hops: hops})
				}
				return printJSON(out)
			}

			if len(gws) == 0 {
				fmt.Println("No gateway routes.")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tHOPS")
			for _, gw := range gws {
				hops := make([]string, len(gw.Hops))
				for i, h := range gw.Hops {
					srv, _ := database.Servers.GetByID(ctx, h.ServerID)
					if srv != nil {
						hops[i] = srv.Host
					}
				}
				fmt.Fprintf(w, "%s\t%s\n", gw.Name, strings.Join(hops, " → "))
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "output format: table|json")
	return cmd
}

func newGatewayShowCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show gateway details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			gw, err := database.Gateways.GetByName(ctx, args[0])
			if err != nil || gw == nil {
				return fmt.Errorf("gateway %s not found", args[0])
			}

			if format == "json" {
				type hopJSON struct {
					Order int    `json:"order"`
					Host  string `json:"host"`
					User  string `json:"user"`
					Port  int    `json:"port"`
				}
				hops := make([]hopJSON, 0, len(gw.Hops))
				for i, h := range gw.Hops {
					srv, _ := database.Servers.GetByID(ctx, h.ServerID)
					if srv != nil {
						hops = append(hops, hopJSON{Order: i + 1, Host: srv.Host, User: srv.User, Port: srv.EffectivePort()})
					}
				}
				return printJSON(map[string]any{"id": gw.ID, "name": gw.Name, "hops": hops})
			}

			fmt.Printf("Name: %s\nHops:\n", gw.Name)
			for i, h := range gw.Hops {
				srv, _ := database.Servers.GetByID(ctx, h.ServerID)
				if srv != nil {
					fmt.Printf("  %d. %s@%s:%d\n", i+1, srv.User, srv.Host, srv.EffectivePort())
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "output format: table|json")
	return cmd
}

func newGatewayDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:  "delete <name>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			gw, err := database.Gateways.GetByName(ctx, args[0])
			if err != nil || gw == nil {
				return fmt.Errorf("gateway %s not found", args[0])
			}
			return database.Gateways.Delete(ctx, gw.ID)
		},
	}
}
