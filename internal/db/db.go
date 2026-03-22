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

	if err := applySchema(sqlDB); err != nil {
		sqlDB.Close()
		return nil, err
	}

	db := &DB{sql: sqlDB}
	db.Servers = &serverRepo{db: sqlDB}
	db.Gateways = &gatewayRepo{db: sqlDB}
	db.Aliases = &aliasRepo{db: sqlDB}
	db.Clusters = &clusterRepo{db: sqlDB}
	db.Themes = &themeRepo{db: sqlDB}
	db.Hosts = &hostRepo{db: sqlDB}
	db.Tunnels = &tunnelRepo{db: sqlDB}
	return db, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.sql.Close()
}

// Raw returns the underlying *sql.DB for ad-hoc queries.
func (d *DB) Raw() *sql.DB {
	return d.sql
}

func applySchema(db *sql.DB) error {
	_, err := db.ExecContext(context.Background(), schemaSQL)
	if err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return applyMigrations(db)
}

// applyMigrations runs incremental ALTER TABLE migrations for existing databases.
// New databases get the full schema from schema.sql; existing ones need column additions.
func applyMigrations(db *sql.DB) error {
	ctx := context.Background()

	var version int
	_ = db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&version)

	if version < 2 {
		_, err := db.ExecContext(ctx,
			`ALTER TABLE servers ADD COLUMN gateway_server_id INTEGER REFERENCES servers(id) ON DELETE SET NULL`)
		if err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("migration v2: %w", err)
		}
		_, _ = db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version) VALUES (2)`)
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
			return fmt.Errorf("migration v3: %w", err)
		}
		_, _ = db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version) VALUES (3)`)
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
			return fmt.Errorf("migration v4: %w", err)
		}
		_, err = db.ExecContext(ctx,
			`CREATE INDEX IF NOT EXISTS idx_local_hosts_hostname ON local_hosts(hostname)`)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("migration v4 index: %w", err)
		}
		_, _ = db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version) VALUES (4)`)
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
			return fmt.Errorf("migration v5: %w", err)
		}
		_, err = db.ExecContext(ctx,
			`CREATE INDEX IF NOT EXISTS idx_tunnels_name ON tunnels(name)`)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("migration v5 index: %w", err)
		}
		_, _ = db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version) VALUES (5)`)
	}

	return nil
}
