# Data Files & Schema

Schema: `internal/db/schema.sql` (embedded by `internal/db/db.go`).

Current schema version: **v10**

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
| `audit_log` | _(new in v7)_ | MCP tool execution audit trail |
| `app_servers` | _(new in v10)_ | Named server+plugin bindings |
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

## `audit_log` columns (v7–v9)

| Column | Type | Notes |
|--------|------|-------|
| id | INTEGER PK | |
| agent_id | TEXT | MCP client identifier |
| server_id | INTEGER | FK → servers (nullable) |
| event | TEXT | `exec` \| `cluster` \| `exec_app` |
| tool | TEXT | MCP tool name |
| params | TEXT | JSON-encoded tool parameters |
| output | TEXT | Truncated tool output |
| exit_code | INTEGER | |
| policy_action | TEXT | `allow` \| `deny` \| `require_approval` |
| approved_by | TEXT | Approver identifier (HITL) |
| plugin_name | TEXT | Plugin name when event=exec_app (v9) |
| plugin_vars | TEXT | JSON array of injected var names, no values (v9) |
| plugin_strategy | TEXT | `docker` \| `native` (v9) |
| created_at | DATETIME | |

## `app_servers` columns (v10)

| Column | Type | Notes |
|--------|------|-------|
| id | INTEGER PK | |
| name | TEXT UNIQUE | Human-readable binding identifier |
| server_id | INTEGER | FK → servers ON DELETE CASCADE |
| plugin_name | TEXT | Matches plugin YAML filename without extension |
| auto_gw | INTEGER | 0/1 boolean |
| description | TEXT | Free-form notes |
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
| v7 | Added `audit_log` table (MCP execution audit trail) |
| v8 | Added `policy_yaml` and `system_prompt` columns to `servers` |
| v9 | Added `plugin_name`, `plugin_vars`, `plugin_strategy` columns to `audit_log` |
| v10 | Added `app_servers` table (named server+plugin bindings) |

Database location: `~/.local/share/alogin/alogin.db` (override: `ALOGIN_DB`).
