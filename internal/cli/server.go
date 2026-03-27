package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/emusal/alogin2/internal/model"
	"github.com/spf13/cobra"
)

func newServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage servers in the registry",
	}
	cmd.AddCommand(
		newServerAddCmd(),
		newServerListCmd(),
		newServerShowCmd(),
		newServerDeleteCmd(),
		newServerPasswdCmd(),
		newServerGetPwdCmd(),
	)
	return cmd
}

func newServerAddCmd() *cobra.Command {
	var proto, host, user, password, gateway, locale string
	var port int

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a server to the registry",
		Long: `Add a server to the registry.

The --gateway flag sets the gateway route used when connecting with 'r' (--auto-gw).
Connecting with 't' (direct) ignores the gateway and connects straight to the host.

Examples:
  # Direct-only server (t web-01 connects directly)
  alogin server add --host web-01 --user admin

  # Server reachable only via a gateway (r web-01 goes gw → web-01)
  alogin server add --host web-01 --user admin --gateway corp-gw

  # Explicit multi-hop with t (no gateway needed in registry):
  # t gw-01 web-01   →  connects gw-01 then web-01`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			r := bufio.NewReader(os.Stdin)

			if proto == "" {
				proto = prompt(r, "Protocol [ssh]: ")
				if proto == "" {
					proto = "ssh"
				}
			}
			if host == "" {
				host = prompt(r, "Host: ")
			}
			if user == "" {
				user = prompt(r, "User: ")
			}
			if password == "" {
				password = promptSecret("Password (leave empty to use SSH key): ")
			}

			srv := &model.Server{
				Protocol: model.Protocol(proto),
				Host:     host,
				User:     user,
				Port:     port,
				Locale:   locale,
			}

			if gateway != "" {
				// Accept named route (gateway_list) or server hostname (server_list.gateway).
				if gw, err := database.Gateways.GetByName(ctx, gateway); err == nil && gw != nil {
					srv.GatewayID = &gw.ID
				} else if gwSrv, err := database.Servers.GetByHost(ctx, gateway, ""); err == nil && gwSrv != nil {
					srv.GatewayServerID = &gwSrv.ID
				} else {
					return fmt.Errorf("gateway %q not found as a named route or server", gateway)
				}
			}

			if err := database.Servers.Create(ctx, srv, password); err != nil {
				return fmt.Errorf("add server: %w", err)
			}

			// Store password in vault
			if password != "" && cfg.KeychainUse {
				_ = vlt.Set(vaultKey(srv), password)
			}

			fmt.Printf("Added: %s@%s\n", user, host)
			return nil
		},
	}

	cmd.Flags().StringVar(&proto, "proto", "", "protocol (ssh/sftp/ftp/sshfs)")
	cmd.Flags().StringVar(&host, "host", "", "hostname or IP")
	cmd.Flags().StringVar(&user, "user", "", "login user")
	cmd.Flags().StringVar(&password, "password", "", "password (insecure; prefer interactive prompt)")
	cmd.Flags().IntVar(&port, "port", 0, "port (0 = protocol default)")
	cmd.Flags().StringVar(&gateway, "gateway", "", "gateway route name")
	cmd.Flags().StringVar(&locale, "locale", "", "locale (e.g. ko_KR.eucKR)")
	return cmd
}

func newServerListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			servers, err := database.Servers.ListAll(ctx)
			if err != nil {
				return err
			}

			if format == "json" {
				type serverJSON struct {
					ID         int64  `json:"id"`
					Protocol   string `json:"protocol"`
					Host       string `json:"host"`
					User       string `json:"user"`
					Port       int    `json:"port"`
					Gateway    string `json:"gateway"`
					Locale     string `json:"locale"`
					DeviceType string `json:"device_type"`
					Note       string `json:"note"`
				}
				out := make([]serverJSON, 0, len(servers))
				for _, s := range servers {
					gw := ""
					if s.GatewayID != nil {
						r, _ := database.Gateways.GetByID(ctx, *s.GatewayID)
						if r != nil {
							gw = r.Name
						}
					}
					out = append(out, serverJSON{
						ID: s.ID, Protocol: string(s.Protocol),
						Host: s.Host, User: s.User, Port: s.Port,
						Gateway: gw, Locale: s.Locale,
						DeviceType: string(s.DeviceType), Note: s.Note,
					})
				}
				return printJSON(out)
			}

			if len(servers) == 0 {
				fmt.Println("No servers registered.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tPROTO\tHOST\tUSER\tPORT\tGATEWAY\tLOCALE")
			fmt.Fprintln(w, "--\t-----\t----\t----\t----\t-------\t------")
			for _, s := range servers {
				gw := "-"
				if s.GatewayID != nil {
					r, _ := database.Gateways.GetByID(ctx, *s.GatewayID)
					if r != nil {
						gw = r.Name
					}
				}
				port := "-"
				if s.Port > 0 {
					port = strconv.Itoa(s.Port)
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
					s.ID, s.Protocol, s.Host, s.User, port, gw, s.Locale)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "output format: table|json")
	return cmd
}

func newServerShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [user@]host",
		Short: "Show server details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			user, host := parseUserHost(args[0])
			srv, err := database.Servers.GetByHost(ctx, host, user)
			if err != nil || srv == nil {
				return fmt.Errorf("server %s not found", args[0])
			}

			fmt.Printf("ID:       %d\n", srv.ID)
			fmt.Printf("Protocol: %s\n", srv.Protocol)
			fmt.Printf("Host:     %s\n", srv.Host)
			fmt.Printf("User:     %s\n", srv.User)
			fmt.Printf("Port:     %d (effective: %d)\n", srv.Port, srv.EffectivePort())
			fmt.Printf("Locale:   %s\n", srv.Locale)

			if srv.GatewayID != nil {
				r, _ := database.Gateways.GetByID(ctx, *srv.GatewayID)
				if r != nil {
					hops := make([]string, len(r.Hops))
					for i, h := range r.Hops {
						hopSrv, _ := database.Servers.GetByID(ctx, h.ServerID)
						if hopSrv != nil {
							hops[i] = hopSrv.Host
						}
					}
					fmt.Printf("Gateway:  %s (%s → %s)\n", r.Name,
						strings.Join(hops, " → "), srv.Host)
				}
			} else if srv.GatewayServerID != nil {
				gwSrv, _ := database.Servers.GetByID(ctx, *srv.GatewayServerID)
				if gwSrv != nil {
					fmt.Printf("Gateway:  %s@%s (via server)\n", gwSrv.User, gwSrv.Host)
				}
			}

			pwd, err := vlt.Get(vaultKey(srv))
			if err == nil && pwd != "" {
				fmt.Printf("Password: ****\n")
			} else {
				fmt.Printf("Password: (key auth)\n")
			}
			return nil
		},
	}
}

func newServerDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [user@]host",
		Short: "Delete a server from the registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			user, host := parseUserHost(args[0])
			srv, err := database.Servers.GetByHost(ctx, host, user)
			if err != nil || srv == nil {
				return fmt.Errorf("server %s not found", args[0])
			}

			r := bufio.NewReader(os.Stdin)
			answer := prompt(r, fmt.Sprintf("Delete %s@%s? [y/N]: ", srv.User, srv.Host))
			if strings.ToLower(strings.TrimSpace(answer)) != "y" {
				fmt.Println("Cancelled.")
				return nil
			}

			_ = vlt.Delete(vaultKey(srv))
			return database.Servers.Delete(ctx, srv.ID)
		},
	}
}

func newServerGetPwdCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "getpwd [user@]host",
		Short: "Show the stored password for a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			user, host := parseUserHost(args[0])
			srv, err := database.Servers.GetByHost(ctx, host, user)
			if err != nil || srv == nil {
				return fmt.Errorf("server %s not found", args[0])
			}

			pwd, err := vlt.Get(vaultKey(srv))
			if err != nil || pwd == "" {
				fmt.Printf("%s@%s: no password stored (key auth)\n", srv.User, srv.Host)
				return nil
			}
			fmt.Printf("%s@%s: %s\n", srv.User, srv.Host, pwd)
			return nil
		},
	}
}

func newServerPasswdCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "passwd [user@]host",
		Short: "Change the password for a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			user, host := parseUserHost(args[0])
			srv, err := database.Servers.GetByHost(ctx, host, user)
			if err != nil || srv == nil {
				return fmt.Errorf("server %s not found", args[0])
			}

			newPwd := promptSecret(fmt.Sprintf("New password for %s@%s: ", srv.User, srv.Host))
			if err := vlt.Set(vaultKey(srv), newPwd); err != nil {
				// fallback: store in DB plaintext column
				return database.Servers.Update(ctx, srv, newPwd)
			}
			fmt.Println("Password updated.")
			return nil
		},
	}
}

// --- helpers ---

func prompt(r *bufio.Reader, label string) string {
	fmt.Print(label)
	line, _ := r.ReadString('\n')
	return strings.TrimRight(line, "\r\n")
}

func promptSecret(label string) string {
	fmt.Print(label)
	// For Phase 1: simple stdin read (Phase 2: use term.ReadPassword)
	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	return strings.TrimRight(line, "\r\n")
}
