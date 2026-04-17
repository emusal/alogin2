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
	"golang.org/x/term"
)


func newServerAddCmd() *cobra.Command {
	var proto, host, user, password, locale, gateway, authMethod, identityFile string
	var port int

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a server to the registry",
		Long: `Add a server to the registry.

The --gateway flag sets a per-server internal gateway route that is always applied
after the active profile gateway. Use this for servers that require an extra internal
jump hop beyond the profile gateway.

Full connection path: profile.gateway_hops → server.gateway_hops → server

Examples:
  # Server reachable directly from the profile gateway
  alogin server add --host db-01 --user admin

  # Server requiring an extra internal jump after the profile gateway
  alogin server add --host db-01 --user admin --gateway internal-jump

  # Key-only server with explicit identity file
  alogin server add --host db-01 --user admin --auth-method key --identity-file ~/.ssh/id_ed25519`,
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
			if authMethod == "" {
				authMethod = prompt(r, "Auth method [password/key] (default: password): ")
				if authMethod == "" {
					authMethod = "password"
				}
			}
			if authMethod == "password" && password == "" {
				password = promptSecret("Password (leave empty to use SSH key): ")
			}
			if authMethod == "key" && identityFile == "" {
				identityFile = prompt(r, "Identity file (leave empty to use default key): ")
			}

			srv := &model.Server{
				Protocol:     model.Protocol(proto),
				Host:         host,
				User:         user,
				Port:         port,
				Locale:       locale,
				AuthMethod:   authMethod,
				IdentityFile: identityFile,
			}

			if gateway != "" {
				gw, err := database.Gateways.GetByName(ctx, gateway)
				if err != nil || gw == nil {
					return fmt.Errorf("gateway route %q not found", gateway)
				}
				srv.GatewayRouteID = &gw.ID
			}

			if err := database.Servers.Create(ctx, srv, password); err != nil {
				return fmt.Errorf("add server: %w", err)
			}

			// Store password in vault; mask DB column on success
			if password != "" {
				if err := vlt.Set(vaultKey(srv), password); err == nil {
					_ = database.Servers.ClearPassword(ctx, srv.ID)
				}
			}

			fmt.Printf("Added: %s@%s (auth: %s)\n", user, host, authMethod)
			return nil
		},
	}

	cmd.Flags().StringVar(&proto, "proto", "", "protocol (ssh/sftp/ftp/sshfs)")
	cmd.Flags().StringVar(&host, "host", "", "hostname or IP")
	cmd.Flags().StringVar(&user, "user", "", "login user")
	cmd.Flags().StringVar(&password, "password", "", "password (insecure; prefer interactive prompt)")
	cmd.Flags().IntVar(&port, "port", 0, "port (0 = protocol default)")
	cmd.Flags().StringVar(&locale, "locale", "", "locale (e.g. ko_KR.eucKR)")
	cmd.Flags().StringVar(&gateway, "gateway", "", "internal gateway route name (applied after profile gateway)")
	cmd.Flags().StringVar(&authMethod, "auth-method", "", "authentication method: password|key (default: password)")
	cmd.Flags().StringVar(&identityFile, "identity-file", "", "path to SSH private key (used when --auth-method=key)")
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
					ID             int64  `json:"id"`
					Protocol       string `json:"protocol"`
					Host           string `json:"host"`
					User           string `json:"user"`
					Port           int    `json:"port"`
					Locale         string `json:"locale"`
					DeviceType     string `json:"device_type"`
					Note           string `json:"note"`
					GatewayRouteID *int64 `json:"gateway_route_id,omitempty"`
					AuthMethod     string `json:"auth_method"`
					IdentityFile   string `json:"identity_file,omitempty"`
				}
				out := make([]serverJSON, 0, len(servers))
				for _, s := range servers {
					out = append(out, serverJSON{
						ID: s.ID, Protocol: string(s.Protocol),
						Host: s.Host, User: s.User, Port: s.Port,
						Locale:         s.Locale,
						DeviceType:     string(s.DeviceType), Note: s.Note,
						GatewayRouteID: s.GatewayRouteID,
						AuthMethod:     s.AuthMethod,
						IdentityFile:   s.IdentityFile,
					})
				}
				return printJSON(out)
			}

			if len(servers) == 0 {
				fmt.Println("No servers registered.")
				return nil
			}

			// Pre-load all gateway routes for display (name lookup)
			gwCache := map[int64]string{}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tPROTO\tHOST\tUSER\tPORT\tLOCALE\tGATEWAY")
			fmt.Fprintln(w, "--\t-----\t----\t----\t----\t------\t-------")
			for _, s := range servers {
				port := "-"
				if s.Port > 0 {
					port = strconv.Itoa(s.Port)
				}
				gwName := "-"
				if s.GatewayRouteID != nil {
					if name, ok := gwCache[*s.GatewayRouteID]; ok {
						gwName = name
					} else {
						gw, err := database.Gateways.GetByID(context.Background(), *s.GatewayRouteID)
						if err == nil && gw != nil {
							gwCache[*s.GatewayRouteID] = gw.Name
							gwName = gw.Name
						}
					}
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
					s.ID, s.Protocol, s.Host, s.User, port, s.Locale, gwName)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "output format: table|json")
	return cmd
}

func newServerShowCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
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

			if format == "json" {
				_, pwdErr := vlt.Get(vaultKey(srv))
				hasPassword := pwdErr == nil
				return printJSON(map[string]any{
					"id": srv.ID, "protocol": string(srv.Protocol),
					"host": srv.Host, "user": srv.User,
					"port": srv.Port, "effective_port": srv.EffectivePort(),
					"locale": srv.Locale,
					"device_type":      string(srv.DeviceType), "note": srv.Note,
					"gateway_route_id": srv.GatewayRouteID,
					"has_password":     hasPassword,
					"auth_method":      srv.AuthMethod,
					"identity_file":    srv.IdentityFile,
					"policy_yaml":      srv.PolicyYAML,
					"system_prompt":    srv.SystemPrompt,
				})
			}

			fmt.Printf("ID:       %d\n", srv.ID)
			fmt.Printf("Protocol: %s\n", srv.Protocol)
			fmt.Printf("Host:     %s\n", srv.Host)
			fmt.Printf("User:     %s\n", srv.User)
			fmt.Printf("Port:     %d (effective: %d)\n", srv.Port, srv.EffectivePort())
			fmt.Printf("Locale:   %s\n", srv.Locale)

			if srv.GatewayRouteID != nil {
				gw, err := database.Gateways.GetByID(ctx, *srv.GatewayRouteID)
				if err == nil && gw != nil {
					fmt.Printf("Gateway:  %s (route id: %d)\n", gw.Name, gw.ID)
				} else {
					fmt.Printf("Gateway:  (route id: %d — not found)\n", *srv.GatewayRouteID)
				}
			} else {
				fmt.Printf("Gateway:  (none)\n")
			}

			fmt.Printf("Auth:     %s\n", srv.AuthMethod)
			if srv.IdentityFile != "" {
				fmt.Printf("Key:      %s\n", srv.IdentityFile)
			}
			pwd, err := vlt.Get(vaultKey(srv))
			if err == nil && pwd != "" {
				fmt.Printf("Password: ****\n")
			} else {
				fmt.Printf("Password: (key auth)\n")
			}

			if srv.Note != "" {
				fmt.Printf("Note:     %s\n", srv.Note)
			}
			if srv.PolicyYAML != "" {
				fmt.Printf("Policy:   (set — use 'alogin agent server-policy show %d' to view)\n", srv.ID)
			} else {
				fmt.Printf("Policy:   (none — using global agent-policy.yaml)\n")
			}
			if srv.SystemPrompt != "" {
				preview := srv.SystemPrompt
				if len(preview) > 80 {
					preview = preview[:77] + "..."
				}
				fmt.Printf("Prompt:   %s\n", preview)
			} else {
				fmt.Printf("Prompt:   (none)\n")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "output format: table|json")
	return cmd
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
			// Vault write succeeded — mask any plaintext in the DB column.
			_ = database.Servers.ClearPassword(ctx, srv.ID)
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
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		// fallback for non-terminal (pipe, test)
		r := bufio.NewReader(os.Stdin)
		line, _ := r.ReadString('\n')
		return strings.TrimRight(line, "\r\n")
	}
	return string(b)
}
