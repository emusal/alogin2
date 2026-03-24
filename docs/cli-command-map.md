# CLI Command Map

Entry: `cmd/alogin/main.go` → `internal/cli/root.go`

Commands that skip DB initialization are listed in the `skip` map in `root.go` (version, shell-init, completion, mcp-server).

---

## Connection commands

### `connect` (aliases: `t`, `r`)
File: `internal/cli/connect.go`

```
alogin connect [user@]host [gw1 gw2 ...]
alogin t [user@]host           # direct (no auto-gw)
alogin r [user@]host           # auto-gateway resolution
```

| Flag | Short | Description |
|------|-------|-------------|
| `--auto-gw` | | Force gateway lookup |
| `--dry-run` | | Print hop chain, don't connect |
| `--cmd` | `-c` | Remote command to run |
| `--local-forward` | `-L` | Local port forward spec |
| `--remote-forward` | `-R` | Remote port forward spec |

Port-forward spec: `PORT` | `LPORT:RPORT` | `LPORT:HOST:RPORT` | `LHOST:LPORT:RHOST:RPORT`

### `sftp`
File: `internal/cli/sftp.go`

```
alogin sftp [user@]host                    # interactive SFTP
alogin sftp [user@]host -p local remote    # upload
alogin sftp [user@]host -g remote local    # download
```

| Flag | Short | Description |
|------|-------|-------------|
| `--put` | `-p` | Upload file |
| `--get` | `-g` | Download file |

### `mount`
File: `internal/cli/mount.go`

```
alogin mount [user@]host[:path] [local-path]
alogin mount --unmount host
```

| Flag | Short | Description |
|------|-------|-------------|
| `--unmount` | | Unmount (calls fusermount -u / umount) |

Default mount path: `~/mnt/{host}`

### `ftp`
File: `internal/cli/ftp.go` — delegates to system `ftp` binary.

### `cluster`
File: `internal/cli/cluster.go`

```
alogin cluster [name]                   # interactive picker if no name
alogin cluster list
```

| Flag | Short | Description |
|------|-------|-------------|
| `--mode` | | tmux \| iterm \| terminal (default: tmux) |
| `--tile-x` | `-x` | Tile columns for iTerm2/Terminal |
| `--gateway` | | Override gateway for all members |

---

## Registry management

### `server`
File: `internal/cli/server.go`

```
alogin server add    --proto ssh --host HOST --user USER [--password PW] [--port N] [--gateway GW] [--locale LOCALE] [--device-type TYPE] [--note TEXT]
alogin server list
alogin server show   HOST
alogin server delete HOST
alogin server passwd HOST    # store password in vault
alogin server getpwd HOST    # retrieve password from vault
```

Device type values: `linux` | `windows` | `router` | `switch` | `firewall` | `other`

### `gateway`
File: `internal/cli/gateway.go`

```
alogin gateway add  NAME hop1 [hop2 ...]
alogin gateway list
alogin gateway show NAME
alogin gateway delete NAME
```

### `alias`
File: `internal/cli/alias.go`

```
alogin alias add  SHORT_NAME HOST
alogin alias list
alogin alias show SHORT_NAME
alogin alias delete SHORT_NAME
```

### `hosts`
File: `internal/cli/hosts.go`

```
alogin hosts add    HOSTNAME IP [-d DESCRIPTION]
alogin hosts list
alogin hosts show   HOSTNAME
alogin hosts update HOSTNAME IP [-d DESCRIPTION]
alogin hosts delete HOSTNAME
```

### `tunnel`
File: `internal/cli/tunnel.go`

```
alogin tunnel add    NAME --server HOST --dir L|R --local-port N --remote-host H --remote-port N [--local-host H] [--auto-gw]
alogin tunnel edit   NAME [same flags as add]
alogin tunnel list
alogin tunnel rm     NAME
alogin tunnel start  NAME    # spawn tmux session running "alogin tunnel run NAME"
alogin tunnel stop   NAME    # kill tmux session
alogin tunnel status NAME    # print running state
alogin tunnel run    NAME    # foreground SSH port-forward (called by tmux)
```

---

## Interactive UI

### `tui`
File: `internal/cli/tui.go`

```
alogin tui [--start server|gateway|cluster|hosts|tunnel]
```

Launches Bubbletea TUI. Default start: server list.

### `web`
File: `internal/cli/web.go`

```
alogin web [--port N] [--no-browser]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | 8484 | HTTP listen port |
| `--no-browser` | false | Don't open browser |

---

## System & data management

### `migrate`
File: `internal/cli/migration.go`

```
alogin migrate --from /path/to/v1/data [-v]
```

Imports v1 TSV files: server_list, gateway_list, alias_hosts, clusters, term_themes.

### `db-migrate`
File: `internal/cli/db_migrate.go`

```
alogin db-migrate
```

Applies any pending DB schema migrations. Reports current → target version.

### `upgrade`
File: `internal/cli/upgrade.go`

```
alogin upgrade [-y]
```

Checks GitHub releases API, downloads latest binary, replaces in-place, applies DB migrations.
Detects Homebrew-managed install and advises `brew upgrade alogin` instead.

### `uninstall`
File: `internal/cli/uninstall.go`

```
alogin uninstall [--purge] [-y]
```

| Flag | Description |
|------|-------------|
| `--purge` | Also remove DB and config files |
| `-y` | Skip confirmation prompt |

### `version`
File: `internal/cli/version.go`

```
alogin version
```

Skips DB init (in `skip` map).

### `shell-init`
File: `internal/cli/shell_init.go` (or similar)

```
alogin shell-init [--shell zsh|bash]
```

Outputs shell function shims: `t`, `r`, `s`, `f`, `m`, `ct`, `cr`, `addsvr`, `delsvr`, `dissvr`, `chgsvr`, `chgpwd`, `addalias`, `disalias`, `tver`. Skips DB init.

### `completion`
File: `internal/completion/completion.go`

```
alogin completion zsh
alogin completion bash
alogin completion install [--dir DIR] [--shell zsh|bash]
```

Skips DB init.

### `mcp-server`
File: `internal/cli/mcp.go`

```
alogin mcp-server
```

Starts MCP (Model Context Protocol) stdio server for Claude Desktop integration.
Skips DB init at root level (opens DB internally).

Tools exposed: `list_servers`, `get_server`, `list_tunnels`, `get_tunnel`, `start_tunnel`, `stop_tunnel`, `list_clusters`, `get_cluster`, `exec_command`, `exec_on_cluster`

---

## Adding a new command checklist

When a CLI command is added, changed, or removed — update **all four**:

1. `README.md` — `## 명령어` section (Korean, code block with flags)
2. `README.en.md` — `## Commands` section (English equivalent)
3. `internal/completion/completion.go` — both `ZshScript` and `BashScript` (commands list + case block)
4. `internal/cli/root.go` — add to `skip` map if the command doesn't need DB
