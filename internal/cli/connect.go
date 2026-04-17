package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/emusal/alogin2/internal/model"
	"github.com/emusal/alogin2/internal/policy"
	internalssh "github.com/emusal/alogin2/internal/ssh"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func newConnectCmd() *cobra.Command {
	var (
		profileOverride string
		dryRun          bool
		command         string
		tunnelL         []string
		tunnelR         []string
		appName         string
		forceUTF8       bool
	)

	cmd := &cobra.Command{
		Use:     "connect [user@]host...",
		Aliases: []string{"t", "r"},
		Short:   "Connect to a host via SSH",
		Long: `Connect to a host via SSH.

If no host is provided, opens the interactive TUI host selector.

Single host via active profile gateway (default):
  alogin ssh connect web-01

Single host bypassing gateway (direct connection):
  alogin ssh connect web-01 --profile none

Single host via a named profile:
  alogin ssh connect web-01 --profile home

Explicit multi-hop (each host is an SSH jump, resolved from the registry):
  alogin ssh connect gw-01 web-01
  alogin ssh connect gw-01 gw-02 web-01

Port forwarding (-L local, -R remote):
  alogin ssh connect web-01 -L 8080:localhost:80       # full spec
  alogin ssh connect web-01 -L 2222:22                 # shorthand: local:2222 → dest:22
  alogin ssh connect web-01 -L 2222:22 --profile home  # works through profile gateway too
  alogin ssh connect web-01 -R 2222:127.0.0.1:22       # remote→local reverse tunnel

Other options:
  alogin ssh connect web-01 --cmd "tail -f /var/log/app.log"`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if forceUTF8 && command != "" {
				command = withUTF8(command)
			}
			opts := &model.ConnectOptions{
				Command:         command,
				ProfileOverride: profileOverride,
				DryRun:          dryRun,
				TunnelL:         tunnelL,
				TunnelR:         tunnelR,
				AppName:         appName,
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

	cmd.Flags().StringVar(&profileOverride, "profile", "",
		`network profile override: named profile, "none" for direct connection, or empty to use active profile`)
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print connection route without connecting")
	cmd.Flags().StringVarP(&command, "cmd", "c", "", "command to run after login")
	cmd.Flags().StringArrayVarP(&tunnelL, "local-forward", "L", nil, "local port forward: PORT | LPORT:RPORT | LPORT:host:RPORT | lhost:LPORT:host:RPORT")
	cmd.Flags().StringArrayVarP(&tunnelR, "remote-forward", "R", nil, "remote port forward (SSH -R): RPORT:lhost:LPORT | rhost:RPORT:lhost:LPORT")
	cmd.Flags().StringVar(&appName, "app", "", "application plugin to launch after connecting (e.g. mariadb)")
	cmd.Flags().BoolVar(&forceUTF8, "force-utf8", false, "Force LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 for --cmd (prevents garbled multi-byte output)")

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

	// Policy check for --cmd
	if opts.Command != "" {
		if err := checkCLIPolicy(ctx, srv.ID, srv.PolicyYAML, []string{opts.Command}); err != nil {
			return err
		}
	}

	// Build hop chain via active profile
	hops, err := buildHopChain(ctx, srv, user, opts.ProfileOverride)
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

// buildHopChain constructs the SSH hop chain for a single destination using
// the active network profile (or a profile override).
//
// Profile resolution order:
//  1. profileOverride == "none"   → direct connection, no gateway
//  2. profileOverride == "<name>" → use that named profile's gateway route
//  3. profileOverride == ""       → use the currently active profile's gateway route
//
// Loop prevention: if the destination server IS one of the gateway hop servers,
// connect directly without prepending the gateway chain.
func buildHopChain(ctx context.Context, srv *model.Server, user string, profileOverride string) ([]internalssh.HopConfig, error) {
	var hops []internalssh.HopConfig
	var activeProfile *model.Profile

	if profileOverride != "none" {
		var err error

		if profileOverride != "" {
			activeProfile, err = database.Profiles.GetByName(ctx, profileOverride)
			if err != nil {
				return nil, fmt.Errorf("profile %q: %w", profileOverride, err)
			}
			if activeProfile == nil {
				return nil, fmt.Errorf("profile %q not found", profileOverride)
			}
		} else {
			activeProfile, err = database.Profiles.GetActive(ctx)
			if err != nil {
				return nil, fmt.Errorf("get active profile: %w", err)
			}
		}

		if activeProfile != nil && activeProfile.GatewayRouteID != nil {
			gw, err := database.Gateways.GetByID(ctx, *activeProfile.GatewayRouteID)
			if err != nil || gw == nil {
				return nil, fmt.Errorf("profile gateway route not found: %w", err)
			}

			// Loop prevention: if destination is one of the gateway hops, connect directly.
			isGWHop := false
			for _, h := range gw.Hops {
				if h.ServerID == srv.ID {
					isGWHop = true
					break
				}
			}

			if !isGWHop {
				for _, h := range gw.Hops {
					hopSrv, err := database.Servers.GetByID(ctx, h.ServerID)
					if err != nil || hopSrv == nil {
						return nil, fmt.Errorf("gateway hop server %d not found", h.ServerID)
					}
					pwd, _ := vlt.Get(vaultKey(hopSrv))
					hops = append(hops, internalssh.HopConfig{
						Host:       resolveHost(ctx, hopSrv.Host),
						Port:       hopSrv.EffectivePort(),
						User:       hopSrv.User,
						Password:   pwd,
						AuthMethod: hopSrv.AuthMethod,
						KeyPath:    hopSrv.IdentityFile,
					})
				}
			}
		}
	}

	// Server-level internal gateway hops (applied after profile gateway, before destination).
	// Full path: profile.gateway_hops → server.gateway_route_hops → server
	// Dedup: if server gateway is the same route as the profile gateway, skip it.
	if srv.GatewayRouteID != nil && (activeProfile == nil ||
		activeProfile.GatewayRouteID == nil ||
		*srv.GatewayRouteID != *activeProfile.GatewayRouteID) {
		srvGW, err := database.Gateways.GetByID(ctx, *srv.GatewayRouteID)
		if err != nil || srvGW == nil {
			return nil, fmt.Errorf("server gateway route %d not found", *srv.GatewayRouteID)
		}
		for _, h := range srvGW.Hops {
			// Loop prevention: skip if this hop IS the destination server.
			if h.ServerID == srv.ID {
				continue
			}
			hopSrv, err := database.Servers.GetByID(ctx, h.ServerID)
			if err != nil || hopSrv == nil {
				return nil, fmt.Errorf("server gateway hop %d not found", h.ServerID)
			}
			pwd, _ := vlt.Get(vaultKey(hopSrv))
			hops = append(hops, internalssh.HopConfig{
				Host:       resolveHost(ctx, hopSrv.Host),
				Port:       hopSrv.EffectivePort(),
				User:       hopSrv.User,
				Password:   pwd,
				AuthMethod: hopSrv.AuthMethod,
				KeyPath:    hopSrv.IdentityFile,
			})
		}
	}

	// Destination hop
	pwd, _ := vlt.Get(vaultKey(srv))
	hops = append(hops, internalssh.HopConfig{
		Host:       resolveHost(ctx, srv.Host),
		Port:       srv.EffectivePort(),
		User:       user,
		Password:   pwd,
		AuthMethod: srv.AuthMethod,
		KeyPath:    srv.IdentityFile,
	})

	return hops, nil
}

// doConnectChain handles explicit multi-hop: `ssh gw1 gw2 dest`.
// Each argument is looked up in the registry in order.
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
			Host:       resolveHost(ctx, srv.Host),
			Port:       srv.EffectivePort(),
			User:       user,
			Password:   pwd,
			AuthMethod: srv.AuthMethod,
			KeyPath:    srv.IdentityFile,
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

// checkCLIPolicy enforces agent-policy.yaml for commands run via "alogin ssh connect --cmd".
func checkCLIPolicy(ctx context.Context, serverID int64, serverPolicyYAML string, commands []string) error {
	policyFile := filepath.Join(cfg.ConfigDir, "agent-policy.yaml")
	eng, err := policy.LoadFile(policyFile)
	if err != nil {
		return fmt.Errorf("load policy: %w", err)
	}

	if !policy.EnforceCLICmd(eng) {
		return nil
	}

	eng, err = policy.ResolveFor(eng, serverPolicyYAML)
	if err != nil {
		return fmt.Errorf("per-server policy: %w", err)
	}

	result := policy.EvalPolicy(eng, policy.CheckRequest{
		Commands: commands,
		ServerID: serverID,
	})

	switch result.Action {
	case "deny":
		msg := "policy denied"
		if result.RuleName != "" {
			msg += fmt.Sprintf(": rule %q", result.RuleName)
		}
		return fmt.Errorf("%s", msg)
	case "require_approval":
		token := uuid.New().String()
		pending := policy.PendingRequest{
			Token:    token,
			ServerID: serverID,
			Commands: commands,
		}
		timeout := policy.HITLTimeoutFor(eng)
		outcome, err := policy.RequestApproval(ctx, cfg.ConfigDir, pending, timeout)
		if err != nil {
			return fmt.Errorf("HITL error: %w", err)
		}
		if outcome != policy.OutcomeApproved {
			return fmt.Errorf("HITL: %s (token: %s)", outcome, token)
		}
		return nil
	default:
		return nil
	}
}

func parseTunnelSpecs(specs []string, defaultRemoteHost string) []internalssh.TunnelSpec {
	var result []internalssh.TunnelSpec
	for _, spec := range specs {
		parts := strings.Split(spec, ":")
		var ts internalssh.TunnelSpec
		switch len(parts) {
		case 1:
			fmt.Sscan(parts[0], &ts.LocalPort)
			ts.LocalHost = "127.0.0.1"
			ts.RemoteHost = defaultRemoteHost
			ts.RemotePort = ts.LocalPort
		case 2:
			fmt.Sscan(parts[0], &ts.LocalPort)
			ts.LocalHost = "127.0.0.1"
			ts.RemoteHost = defaultRemoteHost
			fmt.Sscan(parts[1], &ts.RemotePort)
		case 3:
			fmt.Sscan(parts[0], &ts.LocalPort)
			ts.LocalHost = "127.0.0.1"
			ts.RemoteHost = parts[1]
			fmt.Sscan(parts[2], &ts.RemotePort)
		case 4:
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
