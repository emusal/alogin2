// Package migrate converts legacy ALOGIN flat-file data to the v2 SQLite database.
package migrate

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/emusal/alogin2/internal/db"
	"github.com/emusal/alogin2/internal/model"
)

// Options configures a migration run.
type Options struct {
	// LegacyRoot is the path to the old ALOGIN_ROOT directory.
	LegacyRoot string
	// Verbose prints each imported row if true.
	Verbose bool
}

// Run performs the full migration from legacy files into the DB.
func Run(ctx context.Context, database *db.DB, opts Options) error {
	root := opts.LegacyRoot
	log := func(format string, args ...any) {
		if opts.Verbose {
			fmt.Printf(format+"\n", args...)
		}
	}

	// 1. Servers (must be first — gateways/aliases/clusters reference them)
	servers, err := ParseServerList(filepath.Join(root, "server_list"))
	if err != nil {
		return fmt.Errorf("parse server_list: %w", err)
	}
	log("Importing %d servers...", len(servers))
	for _, s := range servers {
		srv := &model.Server{
			Protocol: s.Protocol,
			Host:     s.Host,
			User:     s.User,
			Port:     s.Port,
			Locale:   s.Locale,
		}
		if err := database.Servers.Create(ctx, srv, s.Password); err != nil {
			log("  WARN: server %s@%s: %v", s.User, s.Host, err)
		} else {
			log("  + server %s@%s", s.User, s.Host)
		}
	}

	// 2. Gateway routes
	gateways, err := ParseGatewayList(filepath.Join(root, "gateway_list"))
	if err != nil {
		return fmt.Errorf("parse gateway_list: %w", err)
	}
	log("Importing %d gateway routes...", len(gateways))
	for _, gw := range gateways {
		hopIDs := make([]int64, 0, len(gw.Hops))
		for _, hopHost := range gw.Hops {
			srv, err := database.Servers.GetByHost(ctx, hopHost, "")
			if err != nil || srv == nil {
				log("  WARN: gateway %s hop %s not found in servers", gw.Name, hopHost)
				continue
			}
			hopIDs = append(hopIDs, srv.ID)
		}
		if _, err := database.Gateways.Create(ctx, gw.Name, hopIDs); err != nil {
			log("  WARN: gateway %s: %v", gw.Name, err)
		} else {
			log("  + gateway %s (%d hops)", gw.Name, len(hopIDs))
		}
	}

	// 3. Link server gateway references after both servers and gateway routes are created.
	//
	// V1 semantics: server_list.gateway can be either:
	//   (a) a name in gateway_list  → sets gateway_id (named route)
	//   (b) a hostname in server_list → sets gateway_server_id (direct server ref, recursive chain)
	for _, s := range servers {
		if s.Gateway == "" {
			continue
		}
		dbSrv, err := database.Servers.GetByHost(ctx, s.Host, s.User)
		if err != nil || dbSrv == nil {
			continue
		}

		// Case (a): gateway name matches a named route in gateway_list.
		if route, err := database.Gateways.GetByName(ctx, s.Gateway); err == nil && route != nil {
			dbSrv.GatewayID = &route.ID
			if err := database.Servers.Update(ctx, dbSrv, ""); err != nil {
				log("  WARN: link gateway route for %s: %v", s.Host, err)
			} else {
				log("  ~ gateway route %s → %s", s.Host, s.Gateway)
			}
			continue
		}

		// Case (b): gateway is a server hostname — direct server reference.
		gwSrv, err := database.Servers.GetByHost(ctx, s.Gateway, "")
		if err != nil || gwSrv == nil {
			log("  WARN: gateway %q for %s not found in servers or gateway routes", s.Gateway, s.Host)
			continue
		}
		dbSrv.GatewayServerID = &gwSrv.ID
		if err := database.Servers.Update(ctx, dbSrv, ""); err != nil {
			log("  WARN: link gateway server for %s: %v", s.Host, err)
		} else {
			log("  ~ gateway server %s → %s", s.Host, s.Gateway)
		}
	}

	// 4. Aliases
	aliases, err := ParseAliasHosts(filepath.Join(root, "alias_hosts"))
	if err != nil {
		return fmt.Errorf("parse alias_hosts: %w", err)
	}
	log("Importing %d aliases...", len(aliases))
	for _, a := range aliases {
		srv, err := database.Servers.GetByHost(ctx, a.Host, a.User)
		if err != nil || srv == nil {
			log("  WARN: alias %s → %s not found", a.Alias, a.Host)
			continue
		}
		alias := &model.Alias{Name: a.Alias, ServerID: srv.ID, User: a.User}
		if err := database.Aliases.Create(ctx, alias); err != nil {
			log("  WARN: alias %s: %v", a.Alias, err)
		} else {
			log("  + alias %s → %s", a.Alias, a.Host)
		}
	}

	// 5. Clusters
	clusters, err := ParseClusters(filepath.Join(root, "clusters"))
	if err != nil {
		return fmt.Errorf("parse clusters: %w", err)
	}
	log("Importing %d clusters...", len(clusters))
	for _, c := range clusters {
		var members []model.ClusterMember
		for i, host := range c.Hosts {
			srv, err := database.Servers.GetByHost(ctx, host, "")
			if err != nil || srv == nil {
				log("  WARN: cluster %s member %s not found", c.Name, host)
				continue
			}
			members = append(members, model.ClusterMember{
				ServerID:    srv.ID,
				MemberOrder: i,
			})
		}
		if _, err := database.Clusters.Create(ctx, c.Name, members); err != nil {
			log("  WARN: cluster %s: %v", c.Name, err)
		} else {
			log("  + cluster %s (%d members)", c.Name, len(members))
		}
	}

	// 6. Term themes
	themes, err := ParseTermThemes(filepath.Join(root, "term_themes"))
	if err != nil {
		return fmt.Errorf("parse term_themes: %w", err)
	}
	log("Importing %d terminal themes...", len(themes))
	for i, t := range themes {
		theme := &model.TermTheme{
			LocalePattern: t.Locale,
			ThemeName:     t.Theme,
			Priority:      i,
		}
		if err := database.Themes.Create(ctx, theme); err != nil {
			log("  WARN: theme %s: %v", t.Locale, err)
		} else {
			log("  + theme %s → %s", t.Locale, t.Theme)
		}
	}

	fmt.Println("Migration complete.")
	return nil
}
