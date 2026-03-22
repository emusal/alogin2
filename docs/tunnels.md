# SSH Tunnels

alogin v2 supports two types of SSH port-forwarding:

1. **Ad-hoc** — `-L`/`-R` flags on `alogin connect` (lives only while the session is open)
2. **Persistent (named tunnels)** — `alogin tunnel` subcommand; kept alive by a detached tmux session

---

## Ad-hoc Port Forwarding (`alogin connect -L/-R`)

Flags are accepted on `alogin connect` and work through multi-hop gateway chains.

### `-L` Local forward (your machine → remote)

```
-L PORT                          # local:PORT → dest:PORT
-L LPORT:RPORT                   # local:LPORT → dest:RPORT
-L LPORT:remoteHost:RPORT        # local:LPORT → remoteHost:RPORT via dest
-L localHost:LPORT:remoteHost:RPORT
```

Examples:

```bash
alogin connect db.prod -L 5432:5432           # local:5432 → db.prod:5432
alogin connect web-01 -L 8080:localhost:80    # local:8080 → web-01's localhost:80
alogin connect web-01 --auto-gw -L 2222:22   # through gateway + local:2222 → web-01:22
```

### `-R` Reverse / remote forward (remote → your machine)

```
-R RPORT:localHost:LPORT         # remote:RPORT → localHost:LPORT
-R remoteHost:RPORT:localHost:LPORT
```

Examples:

```bash
alogin connect web-01 -R 2222:127.0.0.1:22   # web-01's :2222 → local:22
```

### Shell-chain fallback warning

When `AllowTcpForwarding` is disabled on an intermediate hop, alogin falls back to
the shell-chain method (nested `ssh -tt`). Port forwarding is **not supported** in
shell-chain mode and a warning is printed to stderr.

---

## Persistent Tunnels (`alogin tunnel`)

Tunnel configurations are stored in the `tunnels` DB table (schema v5).
Each tunnel runs as `alogin tunnel run <name>` inside a detached tmux session named
`alogin-tunnel-<name>`.

### CLI Reference

```
alogin tunnel                         # open TUI tunnel management screen
alogin tunnel list                    # show all tunnels with status
alogin tunnel add <name> [flags]      # register new tunnel
alogin tunnel edit <name> [flags]     # update existing tunnel
alogin tunnel rm <name>               # delete (stops if running)
alogin tunnel start <name>            # spawn tmux session
alogin tunnel stop <name>             # kill tmux session
alogin tunnel status <name>           # running / stopped
```

#### `tunnel add` / `tunnel edit` flags

| Flag | Default | Description |
|------|---------|-------------|
| `--server` | _(required)_ | Server hostname (must exist in registry) |
| `--dir` | `L` | Direction: `L` (local forward) or `R` (remote/reverse) |
| `--local-host` | `127.0.0.1` | Local listen address |
| `--local-port` | _(required)_ | Local port |
| `--remote-host` | _(required)_ | Remote host |
| `--remote-port` | _(required)_ | Remote port |
| `--auto-gw` | `false` | Follow gateway chain from server registry |

### Common examples

```bash
# Forward local:5432 → db.prod:5432 (database tunnel)
alogin tunnel add db-prod \
  --server db.prod --local-port 5432 --remote-host db.prod --remote-port 5432

# Forward local:8080 → web-01's localhost:80 (web app)
alogin tunnel add web-local \
  --server web-01 --local-port 8080 --remote-host localhost --remote-port 80

# Same but route through web-01's registered gateway
alogin tunnel add web-gw \
  --server web-01 --local-port 8080 --remote-host localhost --remote-port 80 --auto-gw

# Start all tunnels
alogin tunnel list | awk 'NR>1 {print $1}' | xargs -I{} alogin tunnel start {}
```

### How it works

```
alogin tunnel start <name>
  └─ tmux new-session -d -s alogin-tunnel-<name>  <binPath> tunnel run <name>
       └─ alogin tunnel run <name>          ← hidden internal command
            ├─ looks up tunnel config in DB
            ├─ dials SSH hop chain (same as connect)
            ├─ calls client.ForwardLocal() or client.ForwardRemote()
            └─ blocks until SIGTERM/SIGINT
```

`tunnel run` is a hidden command (not shown in help). It is called exclusively
by tmux. Sending SIGTERM to the tmux session (via `tunnel stop`) causes a clean
shutdown of the SSH connection.

### TUI integration

From the TUI, type `/tunnel` to jump to the tunnel management screen.
The tunnel list polls running status asynchronously so the UI stays responsive.

### Web UI integration

The React frontend provides a tunnel list panel with start/stop buttons.
See [web-ui.md](web-ui.md) for the REST API endpoints.

---

## Implementation files

| File | Purpose |
|------|---------|
| `internal/tunnel/manager.go` | `Start`, `Stop`, `IsRunning`, `SessionName` |
| `internal/cli/tunnel.go` | Cobra subcommands (`list`, `add`, `edit`, `rm`, `start`, `stop`, `status`, `run`) |
| `internal/db/tunnel_repo.go` | `TunnelRepo` CRUD (ListAll, GetByID, GetByName, Create, Update, Delete) |
| `internal/model/model.go` | `Tunnel`, `TunnelDirection` types |
| `internal/tui/` | `stateTunnelList`, `stateTunnelForm`, `/tunnel` slash command |
| `internal/web/api/handler.go` | REST handlers for tunnel CRUD + start/stop/status |
| `web/frontend/src/components/TunnelList.tsx` | React tunnel list panel |
| `web/frontend/src/components/TunnelFormModal.tsx` | React tunnel add/edit modal |
| `schema.sql` | `tunnels` table definition (also applied via `applyMigrations` v5) |

---

## Known constraints

- Tunnels require `tmux` to be installed and available on `$PATH`.
- Shell-chain fallback (when `AllowTcpForwarding` is disabled) is **not supported** for persistent tunnels — the server must allow direct TCP forwarding.
- The `alogin tunnel run` process inherits the vault passphrase prompt from the terminal that called `tunnel start`. If the vault requires interactive unlock, start the tunnel from an interactive terminal.
