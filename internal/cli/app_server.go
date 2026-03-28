package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/emusal/alogin2/internal/model"
	"github.com/spf13/cobra"
)

func newAppServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app-server",
		Short: "Manage app-server bindings (server + plugin shortcut)",
		Long: `Manage named bindings that pair a compute server with an application plugin.

An app-server binding gives a short name to the combination of a server and a
plugin, so you can connect with a single name instead of specifying both.

Examples:
  alogin app-server add --name prod-mysql --server target-mariadb --app mariadb
  alogin app-server list
  alogin app-server connect prod-mysql
  alogin app-server connect prod-mysql --cmd "SHOW DATABASES;"
  alogin app-server delete prod-mysql`,
	}
	cmd.AddCommand(
		newAppServerListCmd(),
		newAppServerAddCmd(),
		newAppServerShowCmd(),
		newAppServerDeleteCmd(),
		newAppServerConnectCmd(),
		newPluginCmd(),
	)
	return cmd
}

func newAppServerListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List app-server bindings",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			list, err := database.AppServers.ListAll(ctx)
			if err != nil {
				return fmt.Errorf("list app-servers: %w", err)
			}

			type row struct {
				ID          int64  `json:"id"`
				Name        string `json:"name"`
				ServerID    int64  `json:"server_id"`
				Server      string `json:"server"`
				PluginName  string `json:"plugin_name"`
				AutoGW      bool   `json:"auto_gw"`
				Description string `json:"description"`
			}

			rows := make([]row, 0, len(list))
			for _, as := range list {
				srvLabel := fmt.Sprintf("id=%d", as.ServerID)
				if srv, _ := database.Servers.GetByID(ctx, as.ServerID); srv != nil {
					srvLabel = srv.Host
				}
				autoGW := as.AutoGW
				rows = append(rows, row{
					ID:          as.ID,
					Name:        as.Name,
					ServerID:    as.ServerID,
					Server:      srvLabel,
					PluginName:  as.PluginName,
					AutoGW:      autoGW,
					Description: as.Description,
				})
			}

			if format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(rows)
			}

			if len(rows) == 0 {
				fmt.Println("No app-server bindings configured.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tSERVER\tPLUGIN\tAUTO-GW\tDESCRIPTION")
			for _, r := range rows {
				autoGW := "no"
				if r.AutoGW {
					autoGW = "yes"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					r.Name, r.Server, r.PluginName, autoGW, r.Description)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "output format: table|json")
	return cmd
}

func newAppServerAddCmd() *cobra.Command {
	var (
		name        string
		serverHost  string
		serverUser  string
		pluginName  string
		autoGW      bool
		description string
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add an app-server binding",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" || serverHost == "" || pluginName == "" {
				return fmt.Errorf("--name, --server, and --app are required")
			}
			ctx := context.Background()
			srv, err := database.Servers.GetByHost(ctx, serverHost, serverUser)
			if err != nil || srv == nil {
				return fmt.Errorf("server %q not found in registry", serverHost)
			}
			as := &model.AppServer{
				Name:        name,
				ServerID:    srv.ID,
				PluginName:  pluginName,
				AutoGW:      autoGW,
				Description: description,
			}
			if err := database.AppServers.Create(ctx, as); err != nil {
				return fmt.Errorf("create app-server: %w", err)
			}
			fmt.Printf("Created app-server %q → %s --app %s\n", name, srv.Host, pluginName)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "unique name for this binding (required)")
	cmd.Flags().StringVar(&serverHost, "server", "", "server hostname from the registry (required)")
	cmd.Flags().StringVar(&serverUser, "user", "", "server user (disambiguates when host has multiple users)")
	cmd.Flags().StringVar(&pluginName, "app", "", "plugin name, e.g. mariadb (required)")
	cmd.Flags().BoolVar(&autoGW, "auto-gw", false, "connect via gateway by default")
	cmd.Flags().StringVar(&description, "desc", "", "optional description")
	return cmd
}

func newAppServerShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show an app-server binding",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			as, err := database.AppServers.GetByName(ctx, args[0])
			if err != nil {
				return fmt.Errorf("get app-server: %w", err)
			}
			if as == nil {
				return fmt.Errorf("app-server %q not found", args[0])
			}
			srvLabel := fmt.Sprintf("id=%d", as.ServerID)
			if srv, _ := database.Servers.GetByID(ctx, as.ServerID); srv != nil {
				srvLabel = fmt.Sprintf("%s@%s", srv.User, srv.Host)
			}
			autoGW := "no"
			if as.AutoGW {
				autoGW = "yes"
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "Name:\t%s\n", as.Name)
			fmt.Fprintf(w, "Server:\t%s\n", srvLabel)
			fmt.Fprintf(w, "Plugin:\t%s\n", as.PluginName)
			fmt.Fprintf(w, "Auto-GW:\t%s\n", autoGW)
			fmt.Fprintf(w, "Description:\t%s\n", as.Description)
			fmt.Fprintf(w, "Created:\t%s\n", as.CreatedAt.Format("2006-01-02 15:04:05"))
			return w.Flush()
		},
	}
}

func newAppServerDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <name>",
		Aliases: []string{"rm", "del"},
		Short:   "Delete an app-server binding",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			as, err := database.AppServers.GetByName(ctx, args[0])
			if err != nil {
				return fmt.Errorf("get app-server: %w", err)
			}
			if as == nil {
				return fmt.Errorf("app-server %q not found", args[0])
			}
			if err := database.AppServers.Delete(ctx, as.ID); err != nil {
				return fmt.Errorf("delete app-server: %w", err)
			}
			fmt.Printf("Deleted app-server %q\n", args[0])
			return nil
		},
	}
}

func newAppServerConnectCmd() *cobra.Command {
	var cmdFlag string
	cmd := &cobra.Command{
		Use:   "connect <name>",
		Short: "Connect using an app-server binding",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			as, err := database.AppServers.GetByName(ctx, args[0])
			if err != nil {
				return fmt.Errorf("get app-server: %w", err)
			}
			if as == nil {
				return fmt.Errorf("app-server %q not found", args[0])
			}
			srv, err := database.Servers.GetByID(ctx, as.ServerID)
			if err != nil || srv == nil {
				return fmt.Errorf("server id=%d not found", as.ServerID)
			}
			opts := &model.ConnectOptions{
				AppName: as.PluginName,
				AutoGW:  as.AutoGW,
				Command: cmdFlag,
			}
			return doConnect(ctx, srv.User, srv.Host, opts)
		},
	}
	cmd.Flags().StringVarP(&cmdFlag, "cmd", "c", "", "command to run via plugin (e.g. \"SHOW DATABASES;\")")
	return cmd
}
