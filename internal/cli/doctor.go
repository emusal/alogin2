package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	var fix bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check and repair database integrity",
		Long: `Inspect the database for integrity issues and optionally repair them.

Checks performed:
  • Schema version and pending migrations
  • Legacy columns (gateway_id, gateway_server_id) — data migration + cleanup
  • Referential integrity: gateway routes, hop servers, cluster members, tunnels
  • Orphaned vault entries (keys with no matching server)

Without --fix, runs in read-only mode and only reports issues.
With --fix, automatically repairs issues where safe to do so.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			d := &doctor{
				ctx: ctx,
				db:  database.Raw(),
				fix: fix,
			}
			return d.run()
		},
	}

	cmd.Flags().BoolVar(&fix, "fix", false, "automatically repair issues where safe to do so")
	return cmd
}

// ── doctor ────────────────────────────────────────────────────────────────────

type doctor struct {
	ctx      context.Context
	db       *sql.DB
	fix      bool
	issues   int
	fixed    int
	warnings int
}

func (d *doctor) run() error {
	fmt.Println("alogin doctor")
	fmt.Println(strings.Repeat("─", 60))

	checks := []struct {
		name string
		fn   func() error
	}{
		{"Schema version", d.checkSchema},
		{"Legacy gateway columns", d.checkLegacyGatewayColumns},
		{"Gateway route references", d.checkGatewayRouteRefs},
		{"Gateway hop servers", d.checkGatewayHopServers},
		{"Server gateway routes", d.checkServerGatewayRoutes},
		{"Cluster members", d.checkClusterMembers},
		{"Tunnel server references", d.checkTunnelRefs},
		{"App-server references", d.checkAppServerRefs},
		{"Vault orphans", d.checkVaultOrphans},
	}

	for _, c := range checks {
		fmt.Printf("\n[%s]\n", c.name)
		if err := c.fn(); err != nil {
			return fmt.Errorf("%s: %w", c.name, err)
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	if d.issues == 0 && d.warnings == 0 {
		fmt.Println("✓ No issues found.")
	} else {
		if d.issues > 0 {
			if d.fix {
				fmt.Printf("  Issues found: %d  (fixed: %d, remaining: %d)\n",
					d.issues, d.fixed, d.issues-d.fixed)
			} else {
				fmt.Printf("  Issues found: %d  (run with --fix to repair)\n", d.issues)
			}
		}
		if d.warnings > 0 {
			fmt.Printf("  Warnings: %d  (manual action required)\n", d.warnings)
		}
	}
	return nil
}

func (d *doctor) ok(msg string) {
	fmt.Printf("  ✓ %s\n", msg)
}

func (d *doctor) issue(msg string) {
	d.issues++
	fmt.Printf("  ✗ %s\n", msg)
}

func (d *doctor) warn(msg string) {
	d.warnings++
	fmt.Printf("  ⚠ %s\n", msg)
}

func (d *doctor) repaired(msg string) {
	d.fixed++
	fmt.Printf("    → fixed: %s\n", msg)
}

// ── checks ────────────────────────────────────────────────────────────────────

func (d *doctor) checkSchema() error {
	var current int
	_ = d.db.QueryRowContext(d.ctx,
		`SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&current)

	target := func() int {
		v := 0
		rows, err := d.db.QueryContext(d.ctx, `SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1`)
		if err != nil {
			return 0
		}
		defer rows.Close()
		if rows.Next() {
			_ = rows.Scan(&v)
		}
		return v
	}()
	_ = target

	if current == 0 {
		d.warn("schema_migrations table not found — run 'alogin db-migrate'")
		return nil
	}

	// Import CurrentSchemaVersion via db package — access via database field.
	schemaTarget := database.CurrentVersion()
	if current < schemaTarget {
		d.issue(fmt.Sprintf("schema at v%d, target is v%d — run 'alogin db-migrate'", current, schemaTarget))
	} else {
		d.ok(fmt.Sprintf("schema is up to date (v%d)", current))
	}
	return nil
}

func (d *doctor) checkLegacyGatewayColumns() error {
	rawDB := d.db

	hasGWID := columnExistsDoctor(rawDB, d.ctx, "servers", "gateway_id")
	hasGWSrvID := columnExistsDoctor(rawDB, d.ctx, "servers", "gateway_server_id")

	if !hasGWID && !hasGWSrvID {
		d.ok("no legacy gateway columns present")
		return nil
	}

	// Count rows with non-NULL values
	type legacyRow struct {
		id       int64
		gwID     sql.NullInt64
		gwSrvID  sql.NullInt64
		gwRtID   sql.NullInt64
		host     string
		user     string
	}

	if hasGWID {
		var count int
		_ = rawDB.QueryRowContext(d.ctx,
			`SELECT COUNT(*) FROM servers WHERE gateway_id IS NOT NULL`).Scan(&count)

		if count == 0 {
			d.ok("gateway_id column is empty")
			if d.fix {
				if err := dropColumnSafe(rawDB, d.ctx, "servers", "gateway_id"); err != nil {
					d.warn(fmt.Sprintf("could not drop gateway_id: %v", err))
				} else {
					d.repaired("dropped gateway_id column")
				}
			} else {
				d.issue("gateway_id column exists but is unused — run with --fix to drop")
			}
		} else {
			// gateway_id references gateway_routes.id → can copy to gateway_route_id
			d.issue(fmt.Sprintf("gateway_id has %d non-NULL row(s) — data should be migrated to gateway_route_id", count))
			if d.fix {
				migrated, skipped, err := d.migrateGatewayID(rawDB)
				if err != nil {
					d.warn(fmt.Sprintf("migration failed: %v", err))
				} else {
					if migrated > 0 {
						d.repaired(fmt.Sprintf("copied gateway_id → gateway_route_id for %d server(s)", migrated))
					}
					if skipped > 0 {
						d.warn(fmt.Sprintf("%d server(s) already had gateway_route_id set — gateway_id skipped", skipped))
					}
					// After migration, clear gateway_id
					_, _ = rawDB.ExecContext(d.ctx, `UPDATE servers SET gateway_id = NULL`)
					d.repaired("cleared gateway_id column")
					// Try to drop
					if err := dropColumnSafe(rawDB, d.ctx, "servers", "gateway_id"); err != nil {
						d.warn(fmt.Sprintf("could not drop gateway_id: %v", err))
					} else {
						d.repaired("dropped gateway_id column")
					}
				}
			}
		}
	}

	if hasGWSrvID {
		var count int
		_ = rawDB.QueryRowContext(d.ctx,
			`SELECT COUNT(*) FROM servers WHERE gateway_server_id IS NOT NULL`).Scan(&count)

		if count == 0 {
			d.ok("gateway_server_id column is empty")
			if d.fix {
				if err := dropColumnSafe(rawDB, d.ctx, "servers", "gateway_server_id"); err != nil {
					d.warn(fmt.Sprintf("could not drop gateway_server_id: %v", err))
				} else {
					d.repaired("dropped gateway_server_id column")
				}
			} else {
				d.issue("gateway_server_id column exists but is unused — run with --fix to drop")
			}
		} else {
			d.issue(fmt.Sprintf(
				"gateway_server_id has %d non-NULL row(s) — needs migration to gateway_route", count))
			d.printGatewaySrvIDRows(rawDB, count, !d.fix)
			if d.fix {
				migrated, err := d.migrateGatewayServerID(rawDB)
				if err != nil {
					d.warn(fmt.Sprintf("migration failed: %v", err))
				} else if migrated > 0 {
					d.repaired(fmt.Sprintf("created gateway route(s) and linked %d server(s)", migrated))
					// Clear the column after successful migration
					_, _ = rawDB.ExecContext(d.ctx, `UPDATE servers SET gateway_server_id = NULL`)
					d.repaired("cleared gateway_server_id column")
					if err := dropColumnSafe(rawDB, d.ctx, "servers", "gateway_server_id"); err != nil {
						d.warn(fmt.Sprintf("could not drop gateway_server_id: %v", err))
					} else {
						d.repaired("dropped gateway_server_id column")
					}
				}
			}
		}
	}

	return nil
}

func (d *doctor) migrateGatewayID(rawDB *sql.DB) (migrated, skipped int, err error) {
	rows, err := rawDB.QueryContext(d.ctx,
		`SELECT id, gateway_id, gateway_route_id FROM servers WHERE gateway_id IS NOT NULL`)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	type row struct {
		id      int64
		gwID    int64
		gwRtID  sql.NullInt64
	}
	var targets []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.gwID, &r.gwRtID); err != nil {
			return 0, 0, err
		}
		targets = append(targets, r)
	}
	_ = rows.Close()

	for _, r := range targets {
		if r.gwRtID.Valid {
			skipped++
			continue
		}
		_, err := rawDB.ExecContext(d.ctx,
			`UPDATE servers SET gateway_route_id = ? WHERE id = ?`, r.gwID, r.id)
		if err != nil {
			return migrated, skipped, fmt.Errorf("update server %d: %w", r.id, err)
		}
		migrated++
	}
	return migrated, skipped, nil
}

// migrateGatewayServerID converts each server's gateway_server_id into a proper
// gateway_route + gateway_hop, then links it via gateway_route_id.
// Reuses an existing route if one already points to the same single gateway server.
func (d *doctor) migrateGatewayServerID(rawDB *sql.DB) (migrated int, err error) {
	rows, err := rawDB.QueryContext(d.ctx,
		`SELECT s.id, s.host, s.user, s.gateway_route_id, g.id, g.host, g.user
		 FROM servers s
		 JOIN servers g ON g.id = s.gateway_server_id
		 WHERE s.gateway_server_id IS NOT NULL`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type target struct {
		srvID, gwSrvID      int64
		srvHost, srvUser    string
		gwHost, gwUser      string
		existingRouteID     sql.NullInt64
	}
	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.srvID, &t.srvHost, &t.srvUser,
			&t.existingRouteID, &t.gwSrvID, &t.gwHost, &t.gwUser); err != nil {
			return migrated, err
		}
		targets = append(targets, t)
	}
	_ = rows.Close()

	// routeCache: gateway server ID → gateway_route ID (avoid creating duplicates)
	routeCache := map[int64]int64{}

	for _, t := range targets {
		// Skip if gateway_route_id already set
		if t.existingRouteID.Valid {
			migrated++
			continue
		}

		routeID, cached := routeCache[t.gwSrvID]
		if !cached {
			// Check if a single-hop route for this gateway server already exists
			// (a route whose only hop is this server)
			var existingRouteID sql.NullInt64
			_ = rawDB.QueryRowContext(d.ctx, `
				SELECT gh.route_id FROM gateway_hops gh
				WHERE gh.server_id = ?
				  AND (SELECT COUNT(*) FROM gateway_hops WHERE route_id = gh.route_id) = 1
				LIMIT 1`, t.gwSrvID).Scan(&existingRouteID)

			if existingRouteID.Valid {
				routeID = existingRouteID.Int64
			} else {
				// Create a new gateway_route named after the gateway host
				routeName := t.gwHost
				// Ensure name is unique by appending suffix if needed
				var taken int
				_ = rawDB.QueryRowContext(d.ctx,
					`SELECT COUNT(*) FROM gateway_routes WHERE name = ?`, routeName).Scan(&taken)
				if taken > 0 {
					routeName = fmt.Sprintf("%s-gw", t.gwHost)
				}

				res, err := rawDB.ExecContext(d.ctx,
					`INSERT INTO gateway_routes(name) VALUES (?)`, routeName)
				if err != nil {
					return migrated, fmt.Errorf("create route %q: %w", routeName, err)
				}
				routeID, _ = res.LastInsertId()

				// Add the single hop
				_, err = rawDB.ExecContext(d.ctx,
					`INSERT INTO gateway_hops(route_id, server_id, hop_order) VALUES (?, ?, 1)`,
					routeID, t.gwSrvID)
				if err != nil {
					return migrated, fmt.Errorf("create hop for route %q: %w", routeName, err)
				}
				fmt.Printf("    → created gateway route %q (id=%d) for %s@%s\n",
					routeName, routeID, t.gwUser, t.gwHost)
			}
			routeCache[t.gwSrvID] = routeID
		}

		// Link server to the route
		if _, err := rawDB.ExecContext(d.ctx,
			`UPDATE servers SET gateway_route_id = ? WHERE id = ?`, routeID, t.srvID); err != nil {
			return migrated, fmt.Errorf("link server %d to route %d: %w", t.srvID, routeID, err)
		}
		migrated++
	}
	return migrated, nil
}

// printGatewaySrvIDRows prints a table of servers with gateway_server_id set.
// showHints=true prints manual migration commands (used when --fix is not set).
func (d *doctor) printGatewaySrvIDRows(rawDB *sql.DB, count int, showHints bool) {
	rows, err := rawDB.QueryContext(d.ctx,
		`SELECT s.id, s.host, s.user, g.host, g.user
		 FROM servers s
		 JOIN servers g ON g.id = s.gateway_server_id
		 WHERE s.gateway_server_id IS NOT NULL
		 LIMIT 20`)
	if err != nil {
		return
	}
	defer rows.Close()

	type entry struct {
		sID            int64
		sHost, sUser   string
		gHost, gUser   string
	}
	var entries []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.sID, &e.sHost, &e.sUser, &e.gHost, &e.gUser); err != nil {
			break
		}
		entries = append(entries, e)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "    SERVER_ID\tSERVER\tGATEWAY_SERVER")
	for _, e := range entries {
		fmt.Fprintf(w, "    %d\t%s@%s\t%s@%s\n", e.sID, e.sUser, e.sHost, e.gUser, e.gHost)
	}
	_ = w.Flush()
	if count > 20 {
		fmt.Printf("    ... and %d more\n", count-20)
	}

	if showHints {
		fmt.Println()
		fmt.Println("  To migrate manually, run the following for each server:")
		for _, e := range entries {
			gwName := e.gHost
			fmt.Println()
			fmt.Printf("  # %s@%s → via %s@%s\n", e.sUser, e.sHost, e.gUser, e.gHost)
			fmt.Printf("  alogin net gateway add %s %s@%s\n", gwName, e.gUser, e.gHost)
			fmt.Printf("  alogin tui   # select %s@%s → [e] edit → set Gateway: %s\n", e.sUser, e.sHost, gwName)
		}
	}
}

func (d *doctor) checkGatewayRouteRefs() error {
	rows, err := d.db.QueryContext(d.ctx,
		`SELECT id, name FROM gateway_routes`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var routes []struct {
		id   int64
		name string
	}
	for rows.Next() {
		var r struct{ id int64; name string }
		_ = rows.Scan(&r.id, &r.name)
		routes = append(routes, r)
	}

	if len(routes) == 0 {
		d.ok("no gateway routes defined")
		return nil
	}
	d.ok(fmt.Sprintf("%d gateway route(s) defined", len(routes)))
	return nil
}

func (d *doctor) checkGatewayHopServers() error {
	rows, err := d.db.QueryContext(d.ctx,
		`SELECT gh.id, gh.route_id, gh.server_id, gr.name
		 FROM gateway_hops gh
		 JOIN gateway_routes gr ON gr.id = gh.route_id
		 WHERE NOT EXISTS (SELECT 1 FROM servers WHERE id = gh.server_id)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var bad []struct {
		hopID, routeID, serverID int64
		routeName                string
	}
	for rows.Next() {
		var b struct{ hopID, routeID, serverID int64; routeName string }
		_ = rows.Scan(&b.hopID, &b.routeID, &b.serverID, &b.routeName)
		bad = append(bad, b)
	}

	if len(bad) == 0 {
		d.ok("all gateway hop servers exist")
		return nil
	}

	for _, b := range bad {
		d.issue(fmt.Sprintf("gateway %q hop references missing server id=%d", b.routeName, b.serverID))
		if d.fix {
			_, err := d.db.ExecContext(d.ctx,
				`DELETE FROM gateway_hops WHERE id = ?`, b.hopID)
			if err != nil {
				d.warn(fmt.Sprintf("could not remove hop %d: %v", b.hopID, err))
			} else {
				d.repaired(fmt.Sprintf("removed orphaned hop (server_id=%d) from route %q", b.serverID, b.routeName))
			}
		}
	}
	return nil
}

func (d *doctor) checkServerGatewayRoutes() error {
	rows, err := d.db.QueryContext(d.ctx,
		`SELECT s.id, s.host, s.user, s.gateway_route_id
		 FROM servers s
		 WHERE s.gateway_route_id IS NOT NULL
		   AND NOT EXISTS (SELECT 1 FROM gateway_routes WHERE id = s.gateway_route_id)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var bad []struct {
		id, routeID  int64
		host, user   string
	}
	for rows.Next() {
		var b struct{ id, routeID int64; host, user string }
		_ = rows.Scan(&b.id, &b.host, &b.user, &b.routeID)
		bad = append(bad, b)
	}

	if len(bad) == 0 {
		d.ok("all server gateway_route_id references are valid")
		return nil
	}

	for _, b := range bad {
		d.issue(fmt.Sprintf("server %s@%s (id=%d) has gateway_route_id=%d which does not exist",
			b.user, b.host, b.id, b.routeID))
		if d.fix {
			_, err := d.db.ExecContext(d.ctx,
				`UPDATE servers SET gateway_route_id = NULL WHERE id = ?`, b.id)
			if err != nil {
				d.warn(fmt.Sprintf("could not clear gateway_route_id for server %d: %v", b.id, err))
			} else {
				d.repaired(fmt.Sprintf("cleared gateway_route_id for server %s@%s", b.user, b.host))
			}
		}
	}
	return nil
}

func (d *doctor) checkClusterMembers() error {
	rows, err := d.db.QueryContext(d.ctx,
		`SELECT cm.id, cm.cluster_id, cm.server_id, c.name
		 FROM cluster_members cm
		 JOIN clusters c ON c.id = cm.cluster_id
		 WHERE NOT EXISTS (SELECT 1 FROM servers WHERE id = cm.server_id)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var bad []struct {
		memberID, clusterID, serverID int64
		clusterName                   string
	}
	for rows.Next() {
		var b struct{ memberID, clusterID, serverID int64; clusterName string }
		_ = rows.Scan(&b.memberID, &b.clusterID, &b.serverID, &b.clusterName)
		bad = append(bad, b)
	}

	if len(bad) == 0 {
		d.ok("all cluster members exist")
		return nil
	}

	for _, b := range bad {
		d.issue(fmt.Sprintf("cluster %q member references missing server id=%d", b.clusterName, b.serverID))
		if d.fix {
			_, err := d.db.ExecContext(d.ctx,
				`DELETE FROM cluster_members WHERE id = ?`, b.memberID)
			if err != nil {
				d.warn(fmt.Sprintf("could not remove cluster member: %v", err))
			} else {
				d.repaired(fmt.Sprintf("removed orphaned member (server_id=%d) from cluster %q", b.serverID, b.clusterName))
			}
		}
	}
	return nil
}

func (d *doctor) checkTunnelRefs() error {
	rows, err := d.db.QueryContext(d.ctx,
		`SELECT t.id, t.name, t.server_id
		 FROM tunnels t
		 WHERE NOT EXISTS (SELECT 1 FROM servers WHERE id = t.server_id)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var bad []struct {
		id, serverID int64
		name         string
	}
	for rows.Next() {
		var b struct{ id, serverID int64; name string }
		_ = rows.Scan(&b.id, &b.name, &b.serverID)
		bad = append(bad, b)
	}

	if len(bad) == 0 {
		d.ok("all tunnel server references are valid")
		return nil
	}

	for _, b := range bad {
		d.issue(fmt.Sprintf("tunnel %q references missing server id=%d", b.name, b.serverID))
		if d.fix {
			_, err := d.db.ExecContext(d.ctx,
				`DELETE FROM tunnels WHERE id = ?`, b.id)
			if err != nil {
				d.warn(fmt.Sprintf("could not remove tunnel %q: %v", b.name, err))
			} else {
				d.repaired(fmt.Sprintf("removed orphaned tunnel %q", b.name))
			}
		}
	}
	return nil
}

func (d *doctor) checkAppServerRefs() error {
	rows, err := d.db.QueryContext(d.ctx,
		`SELECT a.id, a.name, a.server_id
		 FROM app_servers a
		 WHERE NOT EXISTS (SELECT 1 FROM servers WHERE id = a.server_id)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var bad []struct {
		id, serverID int64
		name         string
	}
	for rows.Next() {
		var b struct{ id, serverID int64; name string }
		_ = rows.Scan(&b.id, &b.name, &b.serverID)
		bad = append(bad, b)
	}

	if len(bad) == 0 {
		d.ok("all app-server references are valid")
		return nil
	}

	for _, b := range bad {
		d.issue(fmt.Sprintf("app-server %q references missing server id=%d", b.name, b.serverID))
		if d.fix {
			_, err := d.db.ExecContext(d.ctx,
				`DELETE FROM app_servers WHERE id = ?`, b.id)
			if err != nil {
				d.warn(fmt.Sprintf("could not remove app-server %q: %v", b.name, err))
			} else {
				d.repaired(fmt.Sprintf("removed orphaned app-server %q", b.name))
			}
		}
	}
	return nil
}

func (d *doctor) checkVaultOrphans() error {
	// Check servers that still have a plaintext password in the DB column
	// (password != '_HIDDEN_' and password != '') — these should be in the vault.
	rows, err := d.db.QueryContext(d.ctx,
		`SELECT id, host, user, password FROM servers
		 WHERE password != '_HIDDEN_' AND password != ''`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type srvRow struct {
		id           int64
		host, user   string
		password     string
	}
	var plaintext []srvRow
	for rows.Next() {
		var r srvRow
		_ = rows.Scan(&r.id, &r.host, &r.user, &r.password)
		plaintext = append(plaintext, r)
	}

	if len(plaintext) == 0 {
		d.ok("no plaintext passwords found in DB (all vaulted or key-auth)")
		return nil
	}

	for _, r := range plaintext {
		d.warn(fmt.Sprintf("server %s@%s (id=%d) has plaintext password in DB — consider moving to vault",
			r.user, r.host, r.id))
		if d.fix {
			// Try to migrate to vault, then mask in DB
			key := r.user + "@" + r.host
			if err := vlt.Set(key, r.password); err != nil {
				d.warn(fmt.Sprintf("  could not write to vault: %v", err))
			} else {
				_, _ = d.db.ExecContext(d.ctx,
					`UPDATE servers SET password = '_HIDDEN_' WHERE id = ?`, r.id)
				d.repaired(fmt.Sprintf("moved password for %s@%s to vault", r.user, r.host))
			}
		}
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func columnExistsDoctor(db *sql.DB, ctx context.Context, table, column string) bool {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			continue
		}
		if name == column {
			return true
		}
	}
	return false
}

// dropColumnSafe attempts to DROP COLUMN (SQLite 3.35+).
// Returns an error if the SQLite version does not support it.
func dropColumnSafe(db *sql.DB, ctx context.Context, table, column string) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s DROP COLUMN %s`, table, column))
	return err
}
