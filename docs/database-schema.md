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

Database location: `~/.local/share/alogin/alogin.db` (override: `ALOGIN_DB`).
