package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/emusal/alogin2/internal/model"
	internalssh "github.com/emusal/alogin2/internal/ssh"
	"github.com/spf13/cobra"
)

func newConnectCmd() *cobra.Command {
	var (
		autoGW  bool
		dryRun  bool
		command string
		tunnelL []string
		tunnelR []string
		appName string
	)

	cmd := &cobra.Command{
		Use:     "connect [user@]host...",
		Aliases: []string{"t", "r"},
		Short:   "Connect to a host via SSH",
		Long: `Connect to a host via SSH.

If no host is provided, opens the interactive TUI host selector.

Single host (t — direct, ignores gateway setting):
  alogin connect web-01
  alogin connect admin@web-01

Single host via gateway (r — follows the gateway defined in the registry):
  alogin connect web-01 --auto-gw

Explicit multi-hop (each host is an SSH jump, resolved from the registry):
  alogin connect gw-01 web-01
  alogin connect gw-01 gw-02 web-01

Port forwarding (-L local, -R remote):
  alogin connect web-01 -L 8080:localhost:80       # full spec
  alogin connect web-01 -L 2222:22                 # shorthand: local:2222 → dest:22
  alogin connect web-01 --auto-gw -L 2222:22       # works through gateway too
  alogin connect web-01 -R 2222:127.0.0.1:22       # remote→local reverse tunnel

Other options:
  alogin connect web-01 --cmd "tail -f /var/log/app.log"`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			opts := &model.ConnectOptions{
				Command: command,
				AutoGW:  autoGW,
				DryRun:  dryRun,
				TunnelL: tunnelL,
				TunnelR: tunnelR,
				AppName: appName,
			}

			// No host → launch TUI
			if len(args) == 0 {
				return runConnectTUI(ctx, opts)
			}

			// Multiple hosts → explicit multi-hop chain (like v1 `t host1 host2 dest`)
			if len(args) > 1 {
				return doConnectChain(ctx, args, opts)
			}

			// Single host
			user, host := parseUserHost(args[0])
			return doConnect(ctx, user, host, opts)
		},
	}

	cmd.Flags().BoolVar(&autoGW, "auto-gw", false, "auto-detect gateway route (like legacy 'r' command)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print connection route without connecting")
	cmd.Flags().StringVarP(&command, "cmd", "c", "", "command to run after login")
	cmd.Flags().StringArrayVarP(&tunnelL, "local-forward", "L", nil, "local port forward: PORT | LPORT:RPORT | LPORT:host:RPORT | lhost:LPORT:host:RPORT")
	cmd.Flags().StringArrayVarP(&tunnelR, "remote-forward", "R", nil, "remote port forward (SSH -R): RPORT:lhost:LPORT | rhost:RPORT:lhost:LPORT")
	cmd.Flags().StringVar(&appName, "app", "", "application plugin to launch after connecting (e.g. mariadb)")

	return cmd
}

func doConnect(ctx context.Context, user, host string, opts *model.ConnectOptions) error {
	// Resolve alias → real host
	if alias, err := database.Aliases.GetByName(ctx, host); err == nil && alias != nil {
		srv, _ := database.Servers.GetByID(ctx, alias.ServerID)
		if srv != nil {
			if alias.User != "" {
				user = alias.User
			} else if user == "" {
				user = srv.User
			}
			host = srv.Host
		}
	}

	// Look up server in registry
	srv, err := database.Servers.GetByHost(ctx, host, user)
	if err != nil {
		return fmt.Errorf("lookup server %s: %w", host, err)
	}
	if srv == nil {
		// Not in registry — try direct connection with provided credentials
		return connectDirect(user, host, 22, opts)
	}
	if user == "" {
		user = srv.User
	}

	// Build hop chain
	hops, err := buildHopChain(ctx, srv, user, opts.AutoGW)
	if err != nil {
		return err
	}

	if opts.DryRun {
		printRoute(hops)
		return nil
	}

	// Set locale environment
	env := map[string]string{}
	if srv.Locale != "" && srv.Locale != "-" {
		env["LC_ALL"] = srv.Locale
		env["LANG"] = srv.Locale
	} else if cfg.Lang != "" {
		env["LC_ALL"] = cfg.Lang
	}

	shellOpts := internalssh.ShellOptions{
		Command: opts.Command,
		Env:     env,
	}

	// Single-hop or multi-hop: try direct-tcpip (ProxyJump) first.
	// If an intermediate hop refuses TCP forwarding, fall back to the v1
	// shell-chain method (runs "ssh" inside the shell of each hop — no
	// AllowTcpForwarding required on proxy servers).
	chain, err := internalssh.DialChain(hops)
	if err != nil {
		var eofErr *internalssh.ErrDialViaEOF
		if errors.As(err, &eofErr) {
			if len(opts.TunnelL)+len(opts.TunnelR) > 0 {
				fmt.Fprintf(os.Stderr, "Warning: shell-chain fallback does not support port forwarding; tunnels will not be set up\n")
			}
			fmt.Fprintf(os.Stderr, "Note: direct-tcpip unavailable (%s), retrying via shell-chain\n", eofErr.ProxyAddr)
			return internalssh.ShellChain(hops, shellOpts)
		}
		return err
	}
	defer chain.CloseAll()

	client := chain.Terminal()
	targetHost := hops[len(hops)-1].Host

	// Set up port tunnels (non-blocking).
	// Works for both single-hop and multi-hop (gateway) chains — the tunnel
	// is established on the terminal client, which already carries the full
	// ProxyJump chain internally.
	for _, spec := range parseTunnelSpecs(opts.TunnelL, targetHost) {
		if err := client.ForwardLocal(ctx, spec); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: tunnel -L failed: %v\n", err)
		}
	}
	for _, spec := range parseTunnelSpecs(opts.TunnelR, targetHost) {
		if err := client.ForwardRemote(ctx, spec); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: tunnel -R failed: %v\n", err)
		}
	}

	if opts.AppName != "" {
		return runPlugin(ctx, opts.AppName, client, srv, opts.Command)
	}
	return client.Shell(shellOpts)
}

// buildHopChain constructs the SSH hop chain for a single destination.
//
//   - autoGW=false (t): connect directly, ignoring any gateway stored in the registry.
//   - autoGW=true  (r): follow the gateway chain stored in the registry, mirroring v1
//     get_gateway_list semantics:
//     1. gateway_id set  → named route from gateway_routes (mirrors gateway_list)
//     2. gateway_server_id set → recursive server chain (mirrors server_list.gateway)
func buildHopChain(ctx context.Context, srv *model.Server, user string, autoGW bool) ([]internalssh.HopConfig, error) {
	var hops []internalssh.HopConfig

	if autoGW {
		// Case 1: named gateway route (mirrors v1 gateway_list lookup).
		if srv.GatewayID != nil {
			gwHops, err := database.Gateways.HopsFor(ctx, srv.ID)
			if err != nil {
				return nil, fmt.Errorf("gateway hops: %w", err)
			}
			for _, h := range gwHops {
				hopSrv, err := database.Servers.GetByID(ctx, h.ServerID)
				if err != nil || hopSrv == nil {
					return nil, fmt.Errorf("gateway hop server %d not found", h.ServerID)
				}
				pwd, _ := vlt.Get(vaultKey(hopSrv))
				hops = append(hops, internalssh.HopConfig{
					Host:     resolveHost(ctx, hopSrv.Host),
					Port:     hopSrv.EffectivePort(),
					User:     hopSrv.User,
					Password: pwd,
				})
			}
		} else if srv.GatewayServerID != nil {
			// Case 2: recursive server chain (mirrors v1 server_list.gateway).
			chain, err := resolveGatewayChain(ctx, srv)
			if err != nil {
				return nil, err
			}
			hops = append(hops, chain...)
		}
	}

	// Destination hop
	pwd, _ := vlt.Get(vaultKey(srv))
	hops = append(hops, internalssh.HopConfig{
		Host:     resolveHost(ctx, srv.Host),
		Port:     srv.EffectivePort(),
		User:     user,
		Password: pwd,
	})

	return hops, nil
}

// resolveGatewayChain follows gateway_server_id links recursively to build the hop
// prefix, mirroring v1's get_gateway_list behaviour.
// Returns hops in order [outermost-gw ... innermost-gw] (destination is appended by caller).
func resolveGatewayChain(ctx context.Context, dest *model.Server) ([]internalssh.HopConfig, error) {
	var chain []internalssh.HopConfig
	visited := map[int64]bool{dest.ID: true}

	cur := dest
	for cur.GatewayServerID != nil {
		gwSrv, err := database.Servers.GetByID(ctx, *cur.GatewayServerID)
		if err != nil || gwSrv == nil {
			return nil, fmt.Errorf("gateway server %d not found", *cur.GatewayServerID)
		}
		if visited[gwSrv.ID] {
			return nil, fmt.Errorf("gateway loop detected at server %s", gwSrv.Host)
		}
		visited[gwSrv.ID] = true

		pwd, _ := vlt.Get(vaultKey(gwSrv))
		// Prepend so the outermost gateway is first.
		chain = append([]internalssh.HopConfig{{
			Host:     resolveHost(ctx, gwSrv.Host),
			Port:     gwSrv.EffectivePort(),
			User:     gwSrv.User,
			Password: pwd,
		}}, chain...)

		cur = gwSrv
	}

	// If the outermost server in the GatewayServerID chain itself has a named
	// gateway route (GatewayID), prepend those hops so the full path is resolved.
	// Example: deep-target →(GatewayServerID) middle →(GatewayID) bastion_gw
	//   produces: [bastion_host, middle, deep-target]
	if cur.GatewayID != nil {
		gwHops, err := database.Gateways.HopsFor(ctx, cur.ID)
		if err != nil {
			return nil, fmt.Errorf("gateway hops for %s: %w", cur.Host, err)
		}
		var prefix []internalssh.HopConfig
		for _, h := range gwHops {
			hopSrv, err := database.Servers.GetByID(ctx, h.ServerID)
			if err != nil || hopSrv == nil {
				return nil, fmt.Errorf("gateway hop server %d not found", h.ServerID)
			}
			pwd, _ := vlt.Get(vaultKey(hopSrv))
			prefix = append(prefix, internalssh.HopConfig{
				Host:     resolveHost(ctx, hopSrv.Host),
				Port:     hopSrv.EffectivePort(),
				User:     hopSrv.User,
				Password: pwd,
			})
		}
		chain = append(prefix, chain...)
	}

	return chain, nil
}

// doConnectChain handles explicit multi-hop: `t gw1 gw2 dest`.
// Each argument is looked up in the registry in order; together they form the
// ProxyJump chain — identical to v1 `t host1 host2 dest` behaviour.
func doConnectChain(ctx context.Context, hostArgs []string, opts *model.ConnectOptions) error {
	var hops []internalssh.HopConfig

	for _, arg := range hostArgs {
		user, host := parseUserHost(arg)

		// Resolve alias first
		if alias, err := database.Aliases.GetByName(ctx, host); err == nil && alias != nil {
			srv, _ := database.Servers.GetByID(ctx, alias.ServerID)
			if srv != nil {
				if alias.User != "" {
					user = alias.User
				} else if user == "" {
					user = srv.User
				}
				host = srv.Host
			}
		}

		srv, err := database.Servers.GetByHost(ctx, host, user)
		if err != nil || srv == nil {
			// Not in registry — use bare credentials (key auth, system user)
			if user == "" {
				user = os.Getenv("USER")
			}
			hops = append(hops, internalssh.HopConfig{Host: resolveHost(ctx, host), Port: 22, User: user})
			continue
		}
		if user == "" {
			user = srv.User
		}
		pwd, _ := vlt.Get(vaultKey(srv))
		hops = append(hops, internalssh.HopConfig{
			Host:     resolveHost(ctx, srv.Host),
			Port:     srv.EffectivePort(),
			User:     user,
			Password: pwd,
		})
	}

	if opts.DryRun {
		printRoute(hops)
		return nil
	}

	shellOpts := internalssh.ShellOptions{Command: opts.Command}

	chain, err := internalssh.DialChain(hops)
	if err != nil {
		var eofErr *internalssh.ErrDialViaEOF
		if errors.As(err, &eofErr) {
			fmt.Fprintf(os.Stderr, "Note: direct-tcpip unavailable (%s), retrying via shell-chain\n", eofErr.ProxyAddr)
			return internalssh.ShellChain(hops, shellOpts)
		}
		return err
	}
	defer chain.CloseAll()

	return chain.Terminal().Shell(shellOpts)
}

func connectDirect(user, host string, port int, opts *model.ConnectOptions) error {
	if user == "" {
		user = os.Getenv("USER")
	}
	hops := []internalssh.HopConfig{{Host: host, Port: port, User: user}}

	chain, err := internalssh.DialChain(hops)
	if err != nil {
		return err
	}
	defer chain.CloseAll()

	return chain.Terminal().Shell(internalssh.ShellOptions{Command: opts.Command})
}

func runConnectTUI(ctx context.Context, opts *model.ConnectOptions) error {
	return runConnectTUIFull(ctx, opts)
}

func printRoute(hops []internalssh.HopConfig) {
	fmt.Println("Connection route:")
	for i, h := range hops {
		prefix := "  →"
		if i == 0 {
			prefix = "  ○"
		}
		if i == len(hops)-1 {
			prefix = "  ●"
		}
		fmt.Printf("%s %s@%s:%d\n", prefix, h.User, h.Host, h.Port)
	}
}

// resolveHost checks the local hosts table before falling back to DNS.
// It is called when constructing HopConfig.Host values so that custom
// hostname→IP mappings are applied transparently at connection time.
func resolveHost(ctx context.Context, hostname string) string {
	if database == nil {
		return hostname
	}
	return database.Hosts.Resolve(ctx, hostname)
}

func vaultKey(srv *model.Server) string {
	return srv.User + "@" + srv.Host
}

func parseUserHost(arg string) (user, host string) {
	if idx := strings.Index(arg, "@"); idx >= 0 {
		return arg[:idx], arg[idx+1:]
	}
	return "", arg
}

// parseTunnelSpecs parses port-forward specs with defaultRemoteHost used when
// the remote host is not explicit. Supported formats:
//
//	PORT                          → 127.0.0.1:PORT → defaultRemoteHost:PORT
//	localPort:remotePort          → 127.0.0.1:LPORT → defaultRemoteHost:RPORT
//	localPort:remoteHost:remotePort
//	localHost:localPort:remoteHost:remotePort
func parseTunnelSpecs(specs []string, defaultRemoteHost string) []internalssh.TunnelSpec {
	var result []internalssh.TunnelSpec
	for _, spec := range specs {
		parts := strings.Split(spec, ":")
		var ts internalssh.TunnelSpec
		switch len(parts) {
		case 1: // PORT — same port on both sides, remote host = destination
			fmt.Sscan(parts[0], &ts.LocalPort)
			ts.LocalHost = "127.0.0.1"
			ts.RemoteHost = defaultRemoteHost
			ts.RemotePort = ts.LocalPort
		case 2: // localPort:remotePort — remote host = destination
			fmt.Sscan(parts[0], &ts.LocalPort)
			ts.LocalHost = "127.0.0.1"
			ts.RemoteHost = defaultRemoteHost
			fmt.Sscan(parts[1], &ts.RemotePort)
		case 3: // localPort:remoteHost:remotePort
			fmt.Sscan(parts[0], &ts.LocalPort)
			ts.LocalHost = "127.0.0.1"
			ts.RemoteHost = parts[1]
			fmt.Sscan(parts[2], &ts.RemotePort)
		case 4: // localHost:localPort:remoteHost:remotePort
			ts.LocalHost = parts[0]
			fmt.Sscan(parts[1], &ts.LocalPort)
			ts.RemoteHost = parts[2]
			fmt.Sscan(parts[3], &ts.RemotePort)
		default:
			fmt.Fprintf(os.Stderr, "Warning: invalid tunnel spec %q\n", spec)
			continue
		}
		result = append(result, ts)
	}
	return result
}
