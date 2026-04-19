# CLI Command Map

Entry: `cmd/alogin/main.go` → `internal/cli/root.go`

Commands that skip DB initialization are annotated with `alogin:skip-db` in their cobra command definition.

---

## Command hierarchy overview

```
alogin server           Server registry management
alogin app              Named server+plugin bindings
alogin ssh              Remote connectivity (SSH, SFTP, FTP, SSHFS, cluster, session)
alogin scp              File transfer to/from remote hosts (push/pull via SFTP)
alogin vault            Stored credentials
alogin net              Network resource management (hosts, tunnels, gateways, profiles)
alogin agent            AI/MCP tools
```

---

## server — Server registry

File: `internal/cli/compute.go` (group), `internal/cli/server.go` (subcommands), `internal/cli/alias.go` (alias subcommand)

```
alogin server add    [--proto ssh] [--host HOST] [--user USER] [--password PW]
                     [--port N] [--gateway GW] [--locale LOCALE]
                     [--device-type TYPE] [--note TEXT]
alogin server list   [--format table|json]
alogin server show   [user@]host
alogin server delete [user@]host
alogin server passwd [user@]host    # update stored password in vault
alogin server getpwd [user@]host    # retrieve password from vault
alogin server alias add    SHORT_NAME HOST
alogin server alias list   [--format table|json]
alogin server alias show   SHORT_NAME
alogin server alias delete SHORT_NAME
```

Device type values: `linux` | `windows` | `router` | `switch` | `firewall` | `other`

| Flag | Description |
|------|-------------|
| `--gateway` | Per-server internal gateway route (applied *after* the active profile gateway). Use for servers that require an extra internal jump beyond the profile gateway. Full path: `profile.gateway_hops → server.gateway_hops → server`. Omit for servers reachable directly from the profile gateway. |

---

## app — Named server+plugin bindings

File: `internal/cli/app_server.go`, `internal/cli/plugin.go`

```
alogin app list    [--format table|json]
alogin app add     --name NAME --server HOST --app PLUGIN [--desc TEXT]
alogin app show    NAME
alogin app delete  NAME                   # aliases: rm, del
alogin app connect NAME [--cmd COMMAND]
alogin app plugin list  [--format table|json]
```

`app` binds a server with an application plugin so a single name launches the correct app (DB client, container shell, etc.) with automatic credential injection.

| Flag | Description |
|------|-------------|
| `--name` | Unique binding name |
| `--server` | Server hostname (must exist in server registry) |
| `--app` | Plugin name (matches `~/.config/alogin/plugins/<name>.yaml`) |
| `--desc` | Free-form description |
| `--cmd` | (connect only) Non-interactive command to run via the plugin |
| `--format` | `table` (default) or `json` |

---

## ssh — Remote connectivity

File: `internal/cli/access.go` (group)

### `ssh connect`
File: `internal/cli/connect.go`

```
alogin ssh connect [user@]host... [flags]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--profile` | | Network profile override: name, `none` (direct), or empty (active profile) |
| `--dry-run` | | Print hop chain, don't connect |
| `--cmd` | `-c` | Remote command to run |
| `--local-forward` | `-L` | Local port forward spec |
| `--remote-forward` | `-R` | Remote port forward spec |
| `--app` | | Application plugin to launch after connecting |

Port-forward spec: `PORT` | `LPORT:RPORT` | `LPORT:HOST:RPORT` | `LHOST:LPORT:RHOST:RPORT`

### `ssh sftp`
File: `internal/cli/sftp.go`

```
alogin ssh sftp [user@]host                    # interactive SFTP
alogin ssh sftp [user@]host -p local remote    # upload
alogin ssh sftp [user@]host -g remote local    # download
```

| Flag | Short | Description |
|------|-------|-------------|
| `--put` | `-p` | Upload file |
| `--get` | `-g` | Download file |

### `ssh ftp`
File: `internal/cli/ftp.go` — delegates to system `ftp` binary.

### `ssh mount`
File: `internal/cli/mount.go`

```
alogin ssh mount [user@]host[:path] [local-path]
alogin ssh mount --unmount host
```

| Flag | Description |
|------|-------------|
| `--unmount` | Unmount (calls fusermount -u / umount) |

Default mount path: `~/mnt/{host}`

### `ssh cluster`
File: `internal/cli/cluster.go`

```
alogin ssh cluster [name]     # interactive picker if no name
alogin ssh cluster add [name] [host1] [host2...]
alogin ssh cluster list
```

| Flag | Short | Description |
|------|-------|-------------|
| `--mode` | | tmux \| iterm \| terminal (default: tmux) |
| `--tile-x` | `-x` | Tile columns for iTerm2/Terminal |
| `--cmd` | `-c` | Run command on all members in parallel (no tmux) |
| `--format` | | Output format when using `--cmd`: table\|json |

### `ssh session`
File: `internal/cli/session.go`, `internal/cli/session_job.go`

Manage persistent stateful SSH sessions. A session holds a single bash process on the remote server so that cwd, environment variables, and shell variables persist across commands.

**Note:** sessions are backed by tmux and persist across separate `alogin` invocations.

```
alogin ssh session start   [user@]host [--id NAME]         # start session, prints session ID
alogin ssh session exec    <id> <command> [--timeout N]    # run command, wait for output
alogin ssh session bg-exec <id> <command> [--timeout N]    # run in background, print job ID
alogin ssh session job status <job-id> [--json]            # poll job state
alogin ssh session job logs   <job-id>                     # fetch captured output
alogin ssh session job list   [--session <id>] [--json]    # list all jobs
alogin ssh session job cancel <job-id>                     # cancel pending/running job
alogin ssh session stop  <id>                              # terminate session
alogin ssh session list                                    # list active sessions
```

| Flag | Description |
|------|-------------|
| `--id` | Session name (default: generated UUID) |
| `--timeout` | Command timeout in seconds (`exec` default 30, `bg-exec` default 3600) |

Job statuses: `pending` → `running` → `done` \| `failed` \| `cancelled`. Output is captured incrementally and available via `job logs` while the job is still running.

Example:
```bash
id=$(alogin ssh session start web-01)
alogin ssh session exec "$id" "cd /var/log"
alogin ssh session exec "$id" "pwd"    # outputs /var/log

job=$(alogin ssh session bg-exec "$id" "apt-get upgrade -y")
alogin ssh session job status "$job"
alogin ssh session job logs   "$job"
alogin ssh session stop "$id"
```

---

## scp — File transfer

File: `internal/cli/scp.go`, `internal/ssh/sftp.go`

Copy files between local and remote hosts via SFTP. Source first, destination second (same convention as `scp(1)`).

```
alogin scp push [-r] <local-path>              <[user@]host:/remote-path>   # upload
alogin scp pull [-r] <[user@]host:/remote-path> <local-path>                # download
```

| Flag | Short | Description |
|------|-------|-------------|
| `--recursive` | `-r` | Recursively transfer a directory tree |

If the destination path ends with `/`, the source filename (or directory name) is appended automatically.

Examples:
```bash
alogin scp push ./deploy.tar.gz web-01:/opt/releases/
alogin scp pull web-01:/var/log/app.log ./
alogin scp pull admin@web-01:/etc/nginx/nginx.conf ./nginx.conf.bak

# Recursive directory transfer
alogin scp push --recursive ./dist/ web-01:/var/www/app/
alogin scp pull -r web-01:/var/log/nginx/ ./logs/
```

Credentials and multi-hop routing follow the same profile/gateway chain as `alogin ssh connect`.

---

## vault — Stored credentials

File: `internal/cli/auth_group.go`

Skips DB init. Uses `ALOGIN_VAULT_PASS` env var for age vault.

```
alogin vault set    <account>   # account format: user@host
alogin vault get    <account>
alogin vault delete <account>
```

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
                              [--local-host 127.0.0.1]
alogin net tunnel edit   NAME [same flags as add]
alogin net tunnel list   [--format table|json]
alogin net tunnel rm     NAME               # aliases: delete, del
alogin net tunnel start  NAME               # spawn detached tmux session
alogin net tunnel stop   NAME               # kill tmux session
alogin net tunnel status NAME               # print running state
alogin net tunnel run    NAME               # [hidden] foreground forward (called by tmux)
```

Tunnel directions: `L` (local forward, `-L LOCAL:REMOTE`) | `R` (reverse, `-R REMOTE:LOCAL`)

### `net gateway`
File: `internal/cli/gateway.go`

```
alogin net gateway add    NAME hop1 [hop2 ...]
alogin net gateway list   [--format table|json]
alogin net gateway show   NAME
alogin net gateway delete NAME
```

### `net profile`
File: `internal/cli/profile.go`

```
alogin net profile add    NAME [--gateway ROUTE] [--desc TEXT]
alogin net profile list   [--format table|json]
alogin net profile show   NAME
alogin net profile edit   NAME [--gateway ROUTE|none] [--desc TEXT]
alogin net profile delete NAME
alogin net profile use    NAME|none
```

A profile activates a gateway route globally. Once active, all SSH connections automatically route through its gateway — no per-command flags needed. Use `alogin net profile use none` to disable gateway routing.

| Flag | Description |
|------|-------------|
| `--gateway` | Gateway route name to attach (use `none` to detach) |
| `--desc` | Free-form description |

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

### `agent setup [client]`
Register alogin as an MCP server in supported AI clients. Without a client argument, runs interactively. Skips DB init.

```
alogin agent setup                  # interactive: detect clients, prompt to choose
alogin agent setup cursor           # Cursor settings.json
alogin agent setup claude-desktop   # claude_desktop_config.json (prompts restart)
alogin agent setup vscode           # VS Code settings.json
alogin agent setup all              # all detected clients
```

Merges `mcpServers` / `cursor.mcpServers` / `mcp.servers` key — does not overwrite unrelated settings.

### `agent skills`
Manage alogin agent skills. Subcommands: `install`, `list`. Skips DB init.

```
alogin agent skills install                      # fetch from GitHub latest release
alogin agent skills install --dir ~/my-skills    # custom destination
alogin agent skills list                         # show installed skills
alogin agent skills list --dir ~/my-skills
```

Skills are installed as `<dir>/<skill-name>/SKILL.md`. Source: `github.com/emusal/alogin2` latest release `skills.tar.gz`.

### `agent policy`
Global HITL/RBAC policy management. Subcommands: `show`, `validate`, `dry-run`. Skips DB init.

```
alogin agent policy show
alogin agent policy validate
alogin agent policy dry-run --cmd <cmd> [--cmd <cmd>...] [--agent <id>] [--server <id>] [--json]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--cmd` | | Command string to evaluate (repeatable) |
| `--agent` | | Agent ID to test against policy rules |
| `--server` | | Server ID to test against policy rules |
| `--json` | | Output result as JSON |

Evaluates the active policy (global `~/.config/alogin/agent-policy.yaml` + built-in destructive patterns) and prints `allow`, `deny`, or `require_approval` with the matched rule name. Does **not** execute the command or trigger HITL.

### `agent audit`
Query the MCP execution audit log stored in SQLite. Subcommands: `list`, `tail`.

```
alogin agent audit list [--agent <id>] [--server <id>] [--event exec|cluster] [--since 1h] [--limit 50] [--json]
alogin agent audit tail
```

### `agent approve / deny / pending`
Manage HITL (Human-in-the-Loop) approval requests. Skips DB init.

```
alogin agent approve <token>
alogin agent deny    <token>
alogin agent pending [--json]
```

### `agent trust / untrust / trust-list`
Grant a temporary trust window so that `require_approval` requests are auto-approved without waiting for a human. Skips DB init.

```
alogin agent trust       [--duration D] [--agent ID] [--server ID]
alogin agent untrust     [--agent ID] [--server ID]
alogin agent trust-list  [--json]
```

| Flag | Description |
|------|-------------|
| `--duration` | Window length: `30m`, `1h`, `2h`, etc. (default `1h`) |
| `--agent` | Restrict auto-approval to this agent ID only |
| `--server` | Restrict auto-approval to this server ID only |
| (no flags) | Global scope — applies to all agents and servers |

Scope priority (most specific wins): `agent:<id>` > `server:<id>` > `global`.
Trust window state is stored in `~/.config/alogin/hitl/trust/` and checked in `applyPolicyResult` before falling through to interactive HITL.

### `agent server-policy`
Manage per-server policy YAML overrides stored in the database. When set, replaces the global `agent-policy.yaml` for commands targeting that server.

```
alogin agent server-policy set   <server-id> [--file policy.yaml | --stdin]
alogin agent server-policy show  <server-id>
alogin agent server-policy clear <server-id>
```

### `agent server-prompt`
Manage per-server LLM system prompt snippets stored in the database.

```
alogin agent server-prompt set   <server-id> [--text "..." | --file prompt.txt | --stdin]
alogin agent server-prompt show  <server-id>
alogin agent server-prompt clear <server-id>
```

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

#### `doctor`
File: `internal/cli/doctor.go`

```
alogin doctor [--fix]
```

| Flag | Description |
|------|-------------|
| `--fix` | Automatically repair issues where safe to do so |

Inspects the database for integrity issues and optionally repairs them. Checks performed:

- Schema version and pending migrations
- Legacy columns (`gateway_id`, `gateway_server_id`) — migrates data to `gateway_route_id` and drops the columns
- Referential integrity: gateway route/hop servers, server `gateway_route_id`, cluster members, tunnels, app-servers
- Plaintext passwords in the DB column (should be in vault)

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
File: `internal/cli/version.go`

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

When a CLI command is added, changed, or removed — update **all four**:

1. `README.md` — `## Commands Overview` section
2. `README.ko.md` — `## 명령어 한눈에 보기` section
3. `internal/completion/completion.go` — both `ZshScript` and `BashScript` (commands list + case block)
4. `internal/cli/root.go` — add annotation `alogin:skip-db` if the command doesn't need DB
5. `docs/cli-command-map.md` — this file
