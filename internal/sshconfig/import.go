package sshconfig

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/emusal/alogin2/internal/db"
	"github.com/emusal/alogin2/internal/model"
)

// ImportOptions configures a single import run.
type ImportOptions struct {
	DryRun  bool
	Verbose bool
}

// ImportResult summarises the outcome of an import run.
type ImportResult struct {
	ServersCreated  int
	ServersSkipped  int
	GatewaysCreated int
	AliasesCreated  int
	Warnings        []string
}

type jumpKey struct{ host, user string }

// Import runs the full import pipeline: parse → resolve hops → write to DB.
//
// Pass order (mirrors internal/migrate/migrate.go dependency order):
//  1. Filter wildcard blocks
//  2. Collect all jump hosts
//  3. Ensure jump-host servers exist in DB
//  4. Create or look up GatewayRoutes for each unique hop chain
//  5. Create destination servers
//  6. Create aliases
func Import(ctx context.Context, database *db.DB, cfg *ParsedConfig, opts ImportOptions) (*ImportResult, error) {
	res := &ImportResult{}

	// Resolve the current OS user once; used as fallback when a Host block has no User directive.
	var currentUser string
	if u, err := user.Current(); err == nil && u.Username != "" {
		currentUser = u.Username
		// Windows may return "DOMAIN\username"; strip domain prefix.
		if idx := strings.LastIndex(currentUser, `\`); idx >= 0 {
			currentUser = currentUser[idx+1:]
		}
	}
	if currentUser == "" {
		if v := os.Getenv("USER"); v != "" {
			currentUser = v
		} else {
			currentUser = os.Getenv("USERNAME")
		}
	}

	logv := func(format string, args ...any) {
		if opts.Verbose || opts.DryRun {
			fmt.Printf(format+"\n", args...)
		}
	}
	warn := func(msg string) {
		res.Warnings = append(res.Warnings, msg)
	}

	// Pass 1 — filter wildcard blocks
	var blocks []*HostBlock
	for _, b := range cfg.Blocks {
		if b.IsWildcard {
			logv("[SKIP] wildcard pattern %q", b.Pattern)
			continue
		}
		blocks = append(blocks, b)
	}

	// Pass 2 — collect unique jump hosts
	jumpHosts := map[jumpKey]JumpHop{}

	for _, b := range blocks {
		var hops []JumpHop
		if b.ProxyJump != "" {
			hops = ParseProxyJump(b.ProxyJump)
		} else if b.ProxyCommand != "" {
			if hop, ok := ParseProxyCommand(b.ProxyCommand); ok {
				hops = []JumpHop{hop}
			} else {
				warn(fmt.Sprintf("ProxyCommand not parseable for %q: %s", b.Pattern, b.ProxyCommand))
			}
		}
		for _, h := range hops {
			jumpHosts[jumpKey{h.Host, h.User}] = h
		}
	}

	// Pass 3 — ensure each jump-host server exists in DB
	// jumpServerIDs maps jumpKey → server ID (real or synthetic for dry-run)
	jumpServerIDs := map[jumpKey]int64{}
	syntheticID := int64(-1)

	for k, hop := range jumpHosts {
		existing, err := database.Servers.GetByHost(ctx, hop.Host, hop.User)
		if err != nil {
			return nil, fmt.Errorf("lookup jump server %s: %w", hop.Host, err)
		}
		if existing != nil {
			jumpServerIDs[k] = existing.ID
			continue
		}
		if opts.DryRun {
			logv("[dry-run] server   %s (jump host)", formatHopAddr(hop))
			jumpServerIDs[k] = syntheticID
			syntheticID--
			res.ServersCreated++
			continue
		}
		srvUser := hop.User
		if srvUser == "" {
			srvUser = currentUser
		}
		srv := &model.Server{
			Protocol:   model.ProtoSSH,
			Host:       hop.Host,
			User:       srvUser,
			Port:       hop.Port,
			AuthMethod: "password",
			DeviceType: model.DeviceLinux,
		}
		if err := database.Servers.Create(ctx, srv, ""); err != nil {
			warn(fmt.Sprintf("create jump server %s: %v", hop.Host, err))
			jumpServerIDs[k] = -999
			continue
		}
		logv("[+] server   %s (jump host)", formatHopAddr(hop))
		jumpServerIDs[k] = srv.ID
		res.ServersCreated++
	}

	// Pass 4 — create or look up GatewayRoutes for each unique hop chain
	// gatewayIDForBlock maps HostBlock.Pattern → gatewayRouteID
	gatewayIDForBlock := map[string]*int64{}

	for _, b := range blocks {
		var hops []JumpHop
		if b.ProxyJump != "" {
			hops = ParseProxyJump(b.ProxyJump)
		} else if b.ProxyCommand != "" {
			if hop, ok := ParseProxyCommand(b.ProxyCommand); ok {
				hops = []JumpHop{hop}
			}
		}
		if len(hops) == 0 {
			continue
		}

		routeName := buildRouteName(hops)
		hopIDs := resolveHopIDs(hops, jumpServerIDs)

		// Check for unresolved hops (creation failures)
		allResolved := true
		for _, id := range hopIDs {
			if id <= 0 && !opts.DryRun {
				allResolved = false
				break
			}
		}
		if !allResolved {
			warn(fmt.Sprintf("gateway %q: one or more hop servers could not be created", routeName))
			continue
		}

		existing, err := database.Gateways.GetByName(ctx, routeName)
		if err != nil {
			return nil, fmt.Errorf("lookup gateway %s: %w", routeName, err)
		}
		if existing != nil {
			gatewayIDForBlock[b.Pattern] = &existing.ID
			continue
		}

		if opts.DryRun {
			logv("[dry-run] gateway  %s (%d hop(s))", routeName, len(hopIDs))
			syntheticID--
			sid := syntheticID
			gatewayIDForBlock[b.Pattern] = &sid
			res.GatewaysCreated++
			continue
		}
		gw, err := database.Gateways.Create(ctx, routeName, hopIDs)
		if err != nil {
			warn(fmt.Sprintf("create gateway %s: %v", routeName, err))
			continue
		}
		logv("[+] gateway  %s (%d hop(s))", routeName, len(gw.Hops))
		gatewayIDForBlock[b.Pattern] = &gw.ID
		res.GatewaysCreated++
	}

	// Pass 5 — create destination servers
	type pendingAlias struct {
		name     string
		serverID int64
		user     string
	}
	var aliasQueue []pendingAlias

	for _, b := range blocks {
		// Skip blocks that are only jump hosts (they were handled in Pass 3).
		// A block is a "pure jump host" if it was discovered in Pass 2 AND has
		// no unique content beyond being referenced as a hop. We still import it
		// as a regular server if it has its own Host block — the duplicate check
		// below handles collision gracefully.

		lookupUser := b.User
		if lookupUser == "" {
			lookupUser = currentUser
		}
		existing, err := database.Servers.GetByHost(ctx, b.HostName, lookupUser)
		if err != nil {
			return nil, fmt.Errorf("lookup server %s: %w", b.HostName, err)
		}
		if existing != nil {
			logv("[SKIP] %s already exists", formatBlockAddr(b))
			res.ServersSkipped++
			continue
		}

		authMethod := "password"
		if b.IdentityFile != "" {
			authMethod = "key"
		}

		gwID := gatewayIDForBlock[b.Pattern]
		var note string
		if b.Pattern != b.HostName {
			note = fmt.Sprintf("ssh-config alias: %s", b.Pattern)
		}

		if opts.DryRun {
			if authMethod == "key" {
				logv("[dry-run] server   %s  (key: %s)", formatBlockAddr(b), b.IdentityFile)
			} else {
				logv("[dry-run] server   %s  (password: will prompt on first connect)", formatBlockAddr(b))
			}
			res.ServersCreated++
			if isSimpleName(b.Pattern) && b.Pattern != b.HostName {
				logv("[dry-run] alias    %q → %s", b.Pattern, formatBlockAddr(b))
			}
			continue
		}

		srvUser := b.User
		if srvUser == "" {
			srvUser = currentUser
		}
		sshOpts := model.SSHOptions{
			Ciphers:           b.Ciphers,
			KexAlgorithms:     b.KexAlgorithms,
			HostKeyAlgorithms: b.HostKeyAlgorithms,
		}
		srv := &model.Server{
			Protocol:       model.ProtoSSH,
			Host:           b.HostName,
			User:           srvUser,
			Port:           b.Port,
			AuthMethod:     authMethod,
			IdentityFile:   b.IdentityFile,
			DeviceType:     model.DeviceLinux,
			Note:           note,
			GatewayRouteID: gwID,
			SSHOptions:     sshOpts,
		}
		if err := database.Servers.Create(ctx, srv, ""); err != nil {
			warn(fmt.Sprintf("create server %s: %v", b.HostName, err))
			continue
		}
		logv("[+] server   %s  (auth: %s)", formatBlockAddr(b), authMethod)
		res.ServersCreated++

		if isSimpleName(b.Pattern) && b.Pattern != b.HostName {
			aliasQueue = append(aliasQueue, pendingAlias{
				name:     b.Pattern,
				serverID: srv.ID,
				user:     b.User,
			})
		}
	}

	// Pass 6 — create aliases
	for _, a := range aliasQueue {
		alias := &model.Alias{Name: a.name, ServerID: a.serverID, User: a.user}
		if err := database.Aliases.Create(ctx, alias); err != nil {
			warn(fmt.Sprintf("create alias %s: %v", a.name, err))
			continue
		}
		logv("[+] alias    %q → server %d", a.name, a.serverID)
		res.AliasesCreated++
	}

	return res, nil
}

// buildRouteName returns a stable name for a gateway route from its hop chain.
func buildRouteName(hops []JumpHop) string {
	names := make([]string, len(hops))
	for i, h := range hops {
		names[i] = h.Host
	}
	return strings.Join(names, "→")
}

// resolveHopIDs returns server IDs for each hop in order.
func resolveHopIDs(hops []JumpHop, ids map[jumpKey]int64) []int64 {
	result := make([]int64, len(hops))
	for i, h := range hops {
		result[i] = ids[jumpKey{h.Host, h.User}]
	}
	return result
}

// isSimpleName returns true when s contains no glob characters or spaces,
// making it usable as an alogin alias.
func isSimpleName(s string) bool {
	return !strings.ContainsAny(s, "*?[ \t")
}

func formatHopAddr(h JumpHop) string {
	user := h.User
	if user == "" {
		user = "<user>"
	}
	port := h.Port
	if port == 0 {
		port = 22
	}
	return fmt.Sprintf("%s@%s:%d", user, h.Host, port)
}

func formatBlockAddr(b *HostBlock) string {
	user := b.User
	if user == "" {
		user = "<user>"
	}
	port := b.Port
	if port == 0 {
		port = 22
	}
	return fmt.Sprintf("%s@%s:%d", user, b.HostName, port)
}
