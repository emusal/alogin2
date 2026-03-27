-- ALOGIN v2 SQLite schema
-- Replaces: server_list, gateway_list, alias_hosts, clusters, term_themes, special_hosts
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

-- ----------------------------------------------------------------
-- servers: replaces server_list TSV
-- password column stores "_HIDDEN_" when vault is active, or the
-- literal password in plaintext/legacy mode.
-- port = 0 means "use protocol default".
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS servers (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    protocol          TEXT    NOT NULL DEFAULT 'ssh'
                              CHECK(protocol IN ('ssh','sftp','ftp','sshfs',
                                                 'telnet','rlogin','vagrant','docker')),
    host              TEXT    NOT NULL,
    user              TEXT    NOT NULL,
    password          TEXT    NOT NULL DEFAULT '_HIDDEN_',
    port              INTEGER NOT NULL DEFAULT 0,
    gateway_id        INTEGER REFERENCES gateway_routes(id) ON DELETE SET NULL,
    gateway_server_id INTEGER REFERENCES servers(id) ON DELETE SET NULL,
    locale            TEXT    NOT NULL DEFAULT '-',
    device_type       TEXT    NOT NULL DEFAULT 'linux',
    note              TEXT    NOT NULL DEFAULT '',
    policy_yaml       TEXT,
    system_prompt     TEXT,
    created_at        TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at        TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    UNIQUE(host, user)
);

CREATE INDEX IF NOT EXISTS idx_servers_host ON servers(host);
CREATE INDEX IF NOT EXISTS idx_servers_user ON servers(user);

-- ----------------------------------------------------------------
-- gateway_routes: named routes (replaces gateway_list)
-- Hops are in gateway_hops, ordered by hop_order.
-- The destination server is the servers row with gateway_id pointing here.
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS gateway_routes (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT    NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS gateway_hops (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    route_id  INTEGER NOT NULL REFERENCES gateway_routes(id) ON DELETE CASCADE,
    server_id INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    hop_order INTEGER NOT NULL,
    UNIQUE(route_id, hop_order)
);

CREATE INDEX IF NOT EXISTS idx_gateway_hops_route ON gateway_hops(route_id);

-- ----------------------------------------------------------------
-- aliases: replaces alias_hosts
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS aliases (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    alias_name TEXT    NOT NULL UNIQUE,
    server_id  INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    user       TEXT    NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_aliases_name ON aliases(alias_name);

-- ----------------------------------------------------------------
-- clusters: replaces clusters file
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS clusters (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT    NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS cluster_members (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    cluster_id   INTEGER NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    server_id    INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    user         TEXT    NOT NULL DEFAULT '',
    member_order INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_cluster_members_cluster ON cluster_members(cluster_id);

-- ----------------------------------------------------------------
-- term_themes: replaces term_themes + special_hosts
-- host_pattern (regexp) takes priority over locale_pattern.
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS term_themes (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    locale_pattern TEXT    NOT NULL DEFAULT '',
    host_pattern   TEXT    NOT NULL DEFAULT '',
    theme_name     TEXT    NOT NULL,
    priority       INTEGER NOT NULL DEFAULT 0
);

-- ----------------------------------------------------------------
-- local_hosts: custom /etc/hosts-like hostname → IP mapping.
-- Resolved before DNS during connection. hostname is unique.
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS local_hosts (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    hostname    TEXT    NOT NULL UNIQUE,
    ip          TEXT    NOT NULL,
    description TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE INDEX IF NOT EXISTS idx_local_hosts_hostname ON local_hosts(hostname);

-- ----------------------------------------------------------------
-- tunnels: persistent SSH port-forward tunnels (v5)
-- ----------------------------------------------------------------
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
);

CREATE INDEX IF NOT EXISTS idx_tunnels_name ON tunnels(name);

-- ----------------------------------------------------------------
-- audit_log: structured record of all MCP exec events (v7)
-- ----------------------------------------------------------------
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
);

CREATE INDEX IF NOT EXISTS idx_audit_log_agent_id   ON audit_log(agent_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_server_id  ON audit_log(server_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log(created_at);

-- ----------------------------------------------------------------
-- schema_migrations: version tracking
-- ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

INSERT OR IGNORE INTO schema_migrations(version) VALUES (1);
INSERT OR IGNORE INTO schema_migrations(version) VALUES (2);
INSERT OR IGNORE INTO schema_migrations(version) VALUES (3);
INSERT OR IGNORE INTO schema_migrations(version) VALUES (4);
INSERT OR IGNORE INTO schema_migrations(version) VALUES (5);
INSERT OR IGNORE INTO schema_migrations(version) VALUES (6);
INSERT OR IGNORE INTO schema_migrations(version) VALUES (7);
INSERT OR IGNORE INTO schema_migrations(version) VALUES (8);
