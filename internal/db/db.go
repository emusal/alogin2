package db

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// CurrentSchemaVersion is the schema version this binary expects.
// Increment when adding a new migration.
const CurrentSchemaVersion = 8

// migrationDescriptions maps each migration version to a human-readable summary.
var migrationDescriptions = map[int]string{
	2: "servers.gateway_server_id column (direct gateway reference)",
	3: "cluster_members rebuild (allow duplicate server in cluster)",
	4: "local_hosts table (custom hostname → IP mapping)",
	5: "tunnels table (persistent SSH port-forward tunnels)",
	6: "servers.device_type, servers.note columns (device classification, notes for AI context)",
	7: "audit_log table for structured MCP exec records",
	8: "servers.policy_yaml, servers.system_prompt columns (per-server policy and LLM prompt override)",
}

// MigrationDescription returns a human-readable description for a schema version.
// Returns an empty string if no description is registered.
func MigrationDescription(version int) string {
	return migrationDescriptions[version]
}

// DB wraps the SQLite connection and all repositories.
type DB struct {
	sql      *sql.DB
	Servers  ServerRepo
	Gateways GatewayRepo
	Aliases  AliasRepo
	Clusters ClusterRepo
	Themes   ThemeRepo
	Hosts    HostRepo
	Tunnels  TunnelRepo
	AuditLog AuditRepo

	// AppliedMigrations holds the schema versions that were actually applied
	// during this Open() call (i.e. were pending before the call).
	// Empty when the DB was already at CurrentSchemaVersion.
	AppliedMigrations []int
}

// Open opens (or creates) the SQLite database at the given path and
// applies the embedded schema.
func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Single writer to avoid SQLITE_BUSY
	sqlDB.SetMaxOpenConns(1)

	applied, err := applySchema(sqlDB)
	if err != nil {
		sqlDB.Close()
		return nil, err
	}

	db := &DB{
		sql:               sqlDB,
		AppliedMigrations: applied,
	}
	db.Servers = &serverRepo{db: sqlDB}
	db.Gateways = &gatewayRepo{db: sqlDB}
	db.Aliases = &aliasRepo{db: sqlDB}
	db.Clusters = &clusterRepo{db: sqlDB}
	db.Themes = &themeRepo{db: sqlDB}
	db.Hosts = &hostRepo{db: sqlDB}
	db.Tunnels = &tunnelRepo{db: sqlDB}
	db.AuditLog = &auditRepo{db: sqlDB}
	return db, nil
}

// SchemaVersion queries and returns the current schema version stored in the DB.
func (d *DB) SchemaVersion(ctx context.Context) int {
	var v int
	_ = d.sql.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&v)
	return v
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.sql.Close()
}

// Raw returns the underlying *sql.DB for ad-hoc queries.
func (d *DB) Raw() *sql.DB {
	return d.sql
}

func applySchema(db *sql.DB) ([]int, error) {
	ctx := context.Background()

	hasMigrations := tableExists(db, ctx, "schema_migrations")
	hasServers := tableExists(db, ctx, "servers")

	if hasMigrations {
		// Normal case: migration tracking already in place.
		// Run only the incremental migrations.
		return applyMigrations(db)
	}

	if !hasServers {
		// Fresh database: apply the full schema (includes all columns and
		// inserts all version markers into schema_migrations).
		if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
			return nil, fmt.Errorf("apply schema: %w", err)
		}
		// No migrations to report — everything was created at the latest version.
		return nil, nil
	}

	// Old database: servers table exists but schema_migrations does not.
	// This happens when upgrading from a version that predates migration tracking.
	// Bootstrap the migrations table at v1 (the baseline that schema.sql always
	// represented), then let applyMigrations apply every change since then.
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
		)
	`); err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}
	_, _ = db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version) VALUES (1)`)
	return applyMigrations(db)
}

// tableExists reports whether a table with the given name exists in the database.
func tableExists(db *sql.DB, ctx context.Context, name string) bool {
	var n string
	return db.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n) == nil
}

// columnExists reports whether the given column exists in the named table.
func columnExists(db *sql.DB, ctx context.Context, table, column string) bool {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			continue
		}
		if name == column {
			return true
		}
	}
	return false
}

// applyMigrations runs incremental ALTER TABLE migrations for existing databases.
// New databases get the full schema from schema.sql; existing ones need column additions.
// Returns the list of schema versions that were actually applied in this call.
func applyMigrations(db *sql.DB) ([]int, error) {
	ctx := context.Background()

	var version int
	_ = db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&version)

	var applied []int

	if version < 2 {
		_, err := db.ExecContext(ctx,
			`ALTER TABLE servers ADD COLUMN gateway_server_id INTEGER REFERENCES servers(id) ON DELETE SET NULL`)
		if err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return applied, fmt.Errorf("migration v2: %w", err)
		}
		_, _ = db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version) VALUES (2)`)
		applied = append(applied, 2)
	}

	if version < 3 {
		// Recreate cluster_members without UNIQUE(cluster_id, server_id) to allow
		// the same server to appear multiple times in a cluster.
		_, err := db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS cluster_members_new (
				id           INTEGER PRIMARY KEY AUTOINCREMENT,
				cluster_id   INTEGER NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
				server_id    INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
				user         TEXT    NOT NULL DEFAULT '',
				member_order INTEGER NOT NULL DEFAULT 0
			);
			INSERT INTO cluster_members_new SELECT id, cluster_id, server_id, user, member_order FROM cluster_members;
			DROP TABLE cluster_members;
			ALTER TABLE cluster_members_new RENAME TO cluster_members;
		`)
		if err != nil {
			return applied, fmt.Errorf("migration v3: %w", err)
		}
		_, _ = db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version) VALUES (3)`)
		applied = append(applied, 3)
	}

	if version < 4 {
		_, err := db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS local_hosts (
				id          INTEGER PRIMARY KEY AUTOINCREMENT,
				hostname    TEXT    NOT NULL UNIQUE,
				ip          TEXT    NOT NULL,
				description TEXT    NOT NULL DEFAULT '',
				created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
				updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
			)
		`)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return applied, fmt.Errorf("migration v4: %w", err)
		}
		_, err = db.ExecContext(ctx,
			`CREATE INDEX IF NOT EXISTS idx_local_hosts_hostname ON local_hosts(hostname)`)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return applied, fmt.Errorf("migration v4 index: %w", err)
		}
		_, _ = db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version) VALUES (4)`)
		applied = append(applied, 4)
	}

	if version < 5 {
		_, err := db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS tunnels (
				id          INTEGER PRIMARY KEY AUTOINCREMENT,
				name        TEXT    NOT NULL UNIQUE,
				server_id   INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
				direction   TEXT    NOT NULL DEFAULT 'L' CHECK(direction IN ('L','R')),
				local_host  TEXT    NOT NULL DEFAULT '127.0.0.1',
				local_port  INTEGER NOT NULL,
				remote_host TEXT    NOT NULL,
				remote_port INTEGER NOT NULL,
				auto_gw     INTEGER NOT NULL DEFAULT 0,
				created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
				updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
			)
		`)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return applied, fmt.Errorf("migration v5: %w", err)
		}
		_, err = db.ExecContext(ctx,
			`CREATE INDEX IF NOT EXISTS idx_tunnels_name ON tunnels(name)`)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return applied, fmt.Errorf("migration v5 index: %w", err)
		}
		_, _ = db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version) VALUES (5)`)
		applied = append(applied, 5)
	}

	// v6: use columnExists as the authoritative check — schema_migrations may have
	// been populated ahead of the actual ALTER TABLE (e.g. schema.sql INSERT ran on
	// an existing DB), leaving the version marker without the columns.
	if version < 6 || !columnExists(db, ctx, "servers", "device_type") {
		for _, col := range []struct{ name, def string }{
			{"device_type", "TEXT NOT NULL DEFAULT 'linux'"},
			{"note", "TEXT NOT NULL DEFAULT ''"},
		} {
			_, err := db.ExecContext(ctx,
				`ALTER TABLE servers ADD COLUMN `+col.name+` `+col.def)
			if err != nil && !strings.Contains(err.Error(), "duplicate column") {
				return applied, fmt.Errorf("migration v6 (%s): %w", col.name, err)
			}
		}
		_, _ = db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version) VALUES (6)`)
		applied = append(applied, 6)
	}

	if version < 7 {
		_, err := db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS audit_log (
				id            INTEGER PRIMARY KEY AUTOINCREMENT,
				timestamp     TEXT    NOT NULL,
				event         TEXT    NOT NULL,
				agent_id      TEXT    NOT NULL DEFAULT '',
				server_id     INTEGER REFERENCES servers(id) ON DELETE SET NULL,
				server_host   TEXT    NOT NULL DEFAULT '',
				cluster_id    INTEGER REFERENCES clusters(id) ON DELETE SET NULL,
				cluster_name  TEXT    NOT NULL DEFAULT '',
				commands      TEXT    NOT NULL DEFAULT '[]',
				intent        TEXT    NOT NULL DEFAULT '',
				timeout_sec   INTEGER NOT NULL DEFAULT 0,
				policy_action TEXT,
				approved_by   TEXT,
				created_at    TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
			)
		`)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return applied, fmt.Errorf("migration v7: %w", err)
		}
		for _, idx := range []string{
			`CREATE INDEX IF NOT EXISTS idx_audit_log_agent_id   ON audit_log(agent_id)`,
			`CREATE INDEX IF NOT EXISTS idx_audit_log_server_id  ON audit_log(server_id)`,
			`CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log(created_at)`,
		} {
			if _, err := db.ExecContext(ctx, idx); err != nil && !strings.Contains(err.Error(), "already exists") {
				return applied, fmt.Errorf("migration v7 index: %w", err)
			}
		}
		_, _ = db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version) VALUES (7)`)
		applied = append(applied, 7)
	}

	if version < 8 || !columnExists(db, ctx, "servers", "policy_yaml") {
		for _, col := range []struct{ name, def string }{
			{"policy_yaml", "TEXT"},
			{"system_prompt", "TEXT"},
		} {
			_, err := db.ExecContext(ctx,
				`ALTER TABLE servers ADD COLUMN `+col.name+` `+col.def)
			if err != nil && !strings.Contains(err.Error(), "duplicate column") {
				return applied, fmt.Errorf("migration v8 (%s): %w", col.name, err)
			}
		}
		_, _ = db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version) VALUES (8)`)
		applied = append(applied, 8)
	}

	return applied, nil
}
