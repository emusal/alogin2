# CLI Command Map

Entry: `cmd/alogin/main.go` → `internal/cli/root.go`

Commands that skip DB initialization are annotated with `alogin:skip-db` in their cobra command definition.

---

## Command hierarchy overview

```
alogin compute          Server registry management (alias: server)
alogin access           Remote connectivity
alogin auth             Credentials and routing
alogin agent            AI/MCP tools
alogin net              Network resource management
```

---

## compute — Server registry

File: `internal/cli/compute.go` (group), `internal/cli/server.go` (subcommands)

Alias: `alogin server` → `alogin compute`

```
alogin compute add    [--proto ssh] [--host HOST] [--user USER] [--password PW]
                      [--port N] [--gateway GW] [--locale LOCALE]
                      [--device-type TYPE] [--note TEXT]
alogin compute list   [--format table|json]
alogin compute show   [user@]host
alogin compute delete [user@]host
alogin compute passwd [user@]host    # update stored password in vault
alogin compute getpwd [user@]host    # retrieve password from vault
```

Device type values: `linux` | `windows` | `router` | `switch` | `firewall` | `other`

---

## access — Remote connectivity

File: `internal/cli/access.go` (group)

### `access ssh`
File: `internal/cli/connect.go`

```
alogin access ssh [user@]host... [flags]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--auto-gw` | | Force gateway lookup |
| `--dry-run` | | Print hop chain, don't connect |
| `--cmd` | `-c` | Remote command to run |
| `--local-forward` | `-L` | Local port forward spec |
| `--remote-forward` | `-R` | Remote port forward spec |

Port-forward spec: `PORT` | `LPORT:RPORT` | `LPORT:HOST:RPORT` | `LHOST:LPORT:RHOST:RPORT`

Legacy aliases: `alogin connect`, `alogin t` (direct), `alogin r` (auto-gateway)

### `access sftp`
File: `internal/cli/sftp.go`

```
alogin access sftp [user@]host                    # interactive SFTP
alogin access sftp [user@]host -p local remote    # upload
alogin access sftp [user@]host -g remote local    # download
```

| Flag | Short | Description |
|------|-------|-------------|
| `--put` | `-p` | Upload file |
| `--get` | `-g` | Download file |

### `access ftp`
File: `internal/cli/ftp.go` — delegates to system `ftp` binary.

### `access mount`
File: `internal/cli/mount.go`

```
alogin access mount [user@]host[:path] [local-path]
alogin access mount --unmount host
```

| Flag | Description |
|------|-------------|
| `--unmount` | Unmount (calls fusermount -u / umount) |

Default mount path: `~/mnt/{host}`

### `access cluster`
File: `internal/cli/cluster.go`

```
alogin access cluster [name]     # interactive picker if no name
alogin access cluster add [name] [host1] [host2...]
alogin access cluster list
```

| Flag | Short | Description |
|------|-------|-------------|
| `--mode` | | tmux \| iterm \| terminal (default: tmux) |
| `--tile-x` | `-x` | Tile columns for iTerm2/Terminal |
| `--gateway` | | Override gateway for all members |

---

## auth — Credentials and routing

File: `internal/cli/auth_group.go` (group)

### `auth gateway`
File: `internal/cli/gateway.go`

```
alogin auth gateway add    NAME hop1 [hop2 ...]
alogin auth gateway list   [--format table|json]
alogin auth gateway show   NAME
alogin auth gateway delete NAME
```

### `auth alias`
File: `internal/cli/alias.go`

```
alogin auth alias add    SHORT_NAME HOST
alogin auth alias list   [--format table|json]
alogin auth alias show   SHORT_NAME
alogin auth alias delete SHORT_NAME
```

### `auth vault`
Phase 2 stub. Uses `ALOGIN_VAULT_PASS` env var for age vault.

---

## agent — AI/MCP tools

File: `internal/cli/agent.go`

### `agent mcp`
Runs alogin as an MCP (Model Context Protocol) server over stdio.

```
alogin agent mcp
```

Skips DB init at root level (opens DB internally). Audit log: `~/.config/alogin/audit.jsonl`.

Available MCP tools (11):
- `list_servers`, `get_server` — server registry queries
- `list_tunnels`, `get_tunnel` — tunnel configuration queries
- `start_tunnel`, `stop_tunnel` — tunnel lifecycle
- `list_clusters`, `get_cluster` — cluster queries with member details
- `exec_command` — run SSH commands on a single server
- `exec_on_cluster` — run SSH commands on all cluster servers in parallel
- `inspect_node` — structured health snapshot (CPU, mem, disk, top processes)

### `agent setup`
Print MCP config snippet and system prompt for AI clients (Claude Desktop, etc.).

```
alogin agent setup
```

Skips DB init.

### `agent policy`
HITL/RBAC policy management (Phase 2 stub).

```
alogin agent policy
```

Skips DB init.

---

## net — Network resources

File: `internal/cli/net.go` (group)

### `net hosts`
File: `internal/cli/hosts.go` — local hostname→IP mappings (custom DNS table).

```
alogin net hosts add    HOSTNAME IP [-d DESCRIPTION]
alogin net hosts list   [--format table|json]
alogin net hosts show   HOSTNAME
alogin net hosts update HOSTNAME NEW_IP [-d DESCRIPTION]
alogin net hosts delete HOSTNAME
```

Aliases for delete: `del`, `rm`

### `net tunnel`
File: `internal/cli/tunnel.go` — persistent SSH port-forward tunnels (tmux-backed).

```
alogin net tunnel add    NAME --server HOST --dir L|R --local-port N
                              --remote-host H --remote-port N
                              [--local-host 127.0.0.1] [--auto-gw]
alogin net tunnel edit   NAME [same flags as add]
alogin net tunnel list   [--format table|json]
alogin net tunnel rm     NAME               # aliases: delete, del
alogin net tunnel start  NAME               # spawn detached tmux session
alogin net tunnel stop   NAME               # kill tmux session
alogin net tunnel status NAME               # print running state
alogin net tunnel run    NAME               # [hidden] foreground forward (called by tmux)
```

Tunnel directions: `L` (local forward, `-L LOCAL:REMOTE`) | `R` (reverse, `-R REMOTE:LOCAL`)

---

## Root-level commands

### Interactive UIs

#### `tui`
File: `internal/cli/tui.go`

```
alogin tui [--start server|gateway|cluster|hosts|tunnel]
```

Launches Bubbletea TUI. Default start: server list.

#### `web`
File: `internal/cli/web.go`

```
alogin web [--port N] [--no-browser]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | 8484 | HTTP listen port |
| `--no-browser` | false | Don't open browser |

### System & data management

#### `migrate`
File: `internal/cli/migration.go`

```
alogin migrate --from /path/to/v1/data [-v]
```

Imports v1 TSV files: server_list, gateway_list, alias_hosts, clusters, term_themes.

#### `db-migrate`
File: `internal/cli/db_migrate.go`

```
alogin db-migrate
```

Applies any pending DB schema migrations. Reports current → target version.

#### `upgrade`
File: `internal/cli/upgrade.go`

```
alogin upgrade [-y]
```

Checks GitHub releases API, downloads latest binary, replaces in-place, applies DB migrations.
Detects Homebrew-managed install and advises `brew upgrade alogin` instead.

#### `uninstall`
File: `internal/cli/uninstall.go`

```
alogin uninstall [--purge] [-y]
```

| Flag | Description |
|------|-------------|
| `--purge` | Also remove DB and config files |
| `-y` | Skip confirmation prompt |

#### `version`
File: `internal/cli/version.go`

```
alogin version
```

Skips DB init.

#### `shell-init`
File: `internal/cli/shell_init.go`

```
alogin shell-init [--shell zsh|bash]
```

Outputs shell function shims: `t`, `r`, `s`, `f`, `m`, `ct`, `cr`, `addsvr`, `delsvr`, `dissvr`, `chgsvr`, `chgpwd`, `addalias`, `disalias`, `tver`. Skips DB init.

#### `completion`
File: `internal/completion/completion.go`

```
alogin completion zsh
alogin completion bash
alogin completion install [--dir DIR] [--shell zsh|bash]
```

Skips DB init.

---

## Adding a new command checklist

When a CLI command is added, changed, or removed — update **all five**:

1. `README.md` — `## 명령어` section (Korean, code block with flags)
2. `README.en.md` — `## Commands` section (English equivalent)
3. `internal/completion/completion.go` — both `ZshScript` and `BashScript` (commands list + case block)
4. `internal/cli/root.go` — add annotation `alogin:skip-db` if the command doesn't need DB
5. `docs/cli-command-map.md` — this file
