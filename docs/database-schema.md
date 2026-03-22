# Data Files & Schema

Schema: `internal/db/schema.sql` (embedded by `internal/db/db.go`).

Key tables:

| Table | Replaces | Notes |
|-------|----------|-------|
| `servers` | `server_list` TSV | `port=0` → protocol default; `password='_HIDDEN_'` when vault active |
| `gateway_routes` + `gateway_hops` | `gateway_list` | Ordered hop chain; destination server has `gateway_id` FK |
| `aliases` | `alias_hosts` | Short name → server_id mapping |
| `clusters` + `cluster_members` | `clusters` | Ordered member list |
| `term_themes` | `term_themes` + `special_hosts` | `host_pattern` regex takes priority over `locale_pattern` |
| `tunnels` | _(new in v5)_ | Named SSH port-forward configs; `direction IN ('L','R')`; `server_id` FK → `servers(id)` ON DELETE CASCADE |

Schema migration versions: v1 (initial) → v4 (local hosts) → **v5 (tunnels)**.

Database location: `~/.local/share/alogin/alogin.db` (override: `ALOGIN_DB`).
