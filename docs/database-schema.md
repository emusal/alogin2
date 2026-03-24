# Data Files & Schema

Schema: `internal/db/schema.sql` (embedded by `internal/db/db.go`).

Current schema version: **v6**

## Tables

| Table | Replaces | Notes |
|-------|----------|-------|
| `servers` | `server_list` TSV | `port=0` → protocol default; `password='_HIDDEN_'` when vault active; `device_type` + `note` added in v6 |
| `gateway_routes` + `gateway_hops` | `gateway_list` | Ordered hop chain; destination server has `gateway_id` FK |
| `aliases` | `alias_hosts` | Short name → server_id mapping |
| `clusters` + `cluster_members` | `clusters` | Ordered member list; duplicate entries allowed |
| `term_themes` | `term_themes` + `special_hosts` | `host_pattern` regex takes priority over `locale_pattern` |
| `local_hosts` | _(new in v4)_ | Custom hostname → IP mappings (replaces /etc/hosts entries) |
| `tunnels` | _(new in v5)_ | Named SSH port-forward configs; `direction IN ('L','R')`; `server_id` FK → `servers(id)` ON DELETE CASCADE |
| `schema_migrations` | — | Version tracking; `version` integer |

## `servers` columns (v6)

| Column | Type | Notes |
|--------|------|-------|
| id | INTEGER PK | |
| protocol | TEXT | ssh, telnet, etc. |
| host | TEXT | |
| user | TEXT | |
| password | TEXT | `_HIDDEN_` when vault active |
| port | INTEGER | 0 = protocol default |
| gateway_id | INTEGER | FK → gateway_routes |
| gateway_server_id | INTEGER | FK → servers (direct jump host) |
| locale | TEXT | SSH env locale |
| device_type | TEXT | linux / windows / router / switch / firewall / other (v6) |
| note | TEXT | Free-form description for AI context (v6) |
| created_at | DATETIME | |
| updated_at | DATETIME | |

Unique constraint: `(host, user)`

## `tunnels` columns (v5)

| Column | Type | Notes |
|--------|------|-------|
| id | INTEGER PK | |
| name | TEXT UNIQUE | Human-readable identifier |
| server_id | INTEGER | FK → servers ON DELETE CASCADE |
| direction | TEXT | `L` (local forward) or `R` (remote forward) |
| local_host | TEXT | |
| local_port | INTEGER | |
| remote_host | TEXT | |
| remote_port | INTEGER | |
| auto_gw | INTEGER | 0/1 boolean |
| created_at | DATETIME | |
| updated_at | DATETIME | |

## Migration history

| Version | Change |
|---------|--------|
| v1 | Baseline schema (servers, gateways, aliases, clusters, term_themes) |
| v2 | Added `gateway_server_id` to servers |
| v3 | Rebuilt cluster_members to allow duplicate server entries |
| v4 | Added `local_hosts` table |
| v5 | Added `tunnels` table |
| v6 | Added `device_type` and `note` columns to servers |

Database location: `~/.local/share/alogin/alogin.db` (override: `ALOGIN_DB`).
