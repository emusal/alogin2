# alogin 2

**SSH connection manager for macOS and Linux** — interactive TUI host picker, encrypted credential vault, multi-hop gateway routing, cluster sessions, and a browser-based web terminal.

> A full Go rewrite of the original [alogin v1](https://github.com/emusal/alogin) (~2000s era Bash + Expect scripts).

**Language** : [한국어](README.md) | English

---

<!-- 📸 Screenshot #1: TUI host picker
     Scene: Full TUI screen right after running `alogin connect` or `alogin tui`.
     A fuzzy search query is partially typed in the search bar, with several matching
     hosts highlighted in the list below. Arrow cursor is on one of the entries.
     Dark terminal background preferred.
-->
![TUI host picker](docs/screenshots/tui-picker.gif)

---

## Features

- **Interactive TUI** — fuzzy-search host picker with arrow navigation (no more typing full hostnames)
- **Pure Go SSH client** — no `expect`, no prompt-pattern hacking
- **Multi-hop gateway routing** — transparent ProxyJump chaining through bastion hosts
- **Encrypted credential vault** — macOS Keychain, Linux Secret Service, or `age`-encrypted file fallback
- **Cluster sessions** — connect to multiple hosts simultaneously via tmux (cross-platform) or iTerm2 / Terminal.app (macOS)
- **Web UI** — browser-based SSH terminal + server management dashboard (`alogin web`)
- **Persistent SSH tunnels** — named port-forward tunnels kept alive in tmux background sessions (`alogin tunnel`)
- **v1 compatibility** — drop-in `t`, `r`, `s`, `f`, `m`, `ct`, `cr` shell functions via a thin shim
- **Migration tool** — one command to import existing `server_list`, `gateway_list`, `clusters`, etc.

---

## Installation

### Script install (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/emusal/alogin2/main/install.sh | sh
```

Installs the Web UI binary to `~/.local/bin/alogin`. Customize with environment variables:

```bash
# CLI-only (no Web UI, smaller binary)
curl -fsSL https://raw.githubusercontent.com/emusal/alogin2/main/install.sh | ALOGIN_NO_WEB=1 sh

# Specific version
curl -fsSL https://raw.githubusercontent.com/emusal/alogin2/main/install.sh | ALOGIN_VERSION=2.0.3 sh

# Custom install path
curl -fsSL https://raw.githubusercontent.com/emusal/alogin2/main/install.sh | ALOGIN_INSTALL_DIR=/usr/local/bin sh
```

### Homebrew (macOS, recommended)

```bash
brew tap emusal/alogin --custom-remote git@github.com:emusal/alogin2.git
brew install alogin
```

### Windows

Native Windows binaries are not supported. Install via WSL (Windows Subsystem for Linux) using the script above.

### Download binary directly

Grab the latest release from the [Releases](https://github.com/emusal/alogin2/releases) page:

```bash
# macOS (Apple Silicon)
curl -fsSL https://github.com/emusal/alogin2/releases/latest/download/alogin-web-darwin-arm64 -o ~/.local/bin/alogin
chmod +x ~/.local/bin/alogin

# macOS (Intel)
curl -fsSL https://github.com/emusal/alogin2/releases/latest/download/alogin-web-darwin-amd64 -o ~/.local/bin/alogin
chmod +x ~/.local/bin/alogin

# Linux (amd64)
curl -fsSL https://github.com/emusal/alogin2/releases/latest/download/alogin-web-linux-amd64 -o ~/.local/bin/alogin
chmod +x ~/.local/bin/alogin

# Linux (arm64)
curl -fsSL https://github.com/emusal/alogin2/releases/latest/download/alogin-web-linux-arm64 -o ~/.local/bin/alogin
chmod +x ~/.local/bin/alogin
```

### Build from source

Requires Go 1.23+.

```bash
git clone https://github.com/emusal/alogin2.git
cd alogin2
go build -o alogin ./cmd/alogin
sudo mv alogin /usr/local/bin/
```

### Uninstall

```bash
# Remove binary, completions, and config (database and vault are preserved)
alogin uninstall

# Remove everything including database and vault (irreversible)
alogin uninstall --purge

# Script-based removal (when binary is unavailable or for remote execution)
curl -fsSL https://raw.githubusercontent.com/emusal/alogin2/main/uninstall.sh | sh

# Full removal via script
curl -fsSL https://raw.githubusercontent.com/emusal/alogin2/main/uninstall.sh | ALOGIN_PURGE=1 sh
```

---

## Quick Start

### 1. Verify installation

```bash
alogin version
```

The database is created automatically at `~/.local/share/alogin/alogin.db` on first run.

### 2. Migrate from alogin v1

If you have an existing v1 installation:

```bash
alogin migrate --from /path/to/old/alogin
```

This imports `server_list`, `gateway_list`, `alias_hosts`, `clusters`, and `term_themes` into SQLite and moves passwords into the system keychain.

### 3. Add a server

```bash
alogin server add
```

Prompts for protocol, host, user, port, gateway, and locale. Password is stored in the system keychain (macOS Keychain / Linux Secret Service) — never in the database.

### 4. Connect

```bash
alogin connect              # opens interactive TUI selector
alogin connect web-01       # connect directly by hostname
alogin connect admin@web-01 # specify user
```

### 5. Shell compatibility (v1 users)

Add to your `~/.zshrc` or `~/.bashrc`:

```bash
source <(alogin shell-init)
```

Then use the same v1 commands as before:

```bash
t web-01          # SSH connect (direct)
r admin@bastion   # SSH connect (auto gateway detection)
s web-01          # SFTP
f ftp-server      # FTP
m web-01          # SSHFS mount
ct prod-cluster   # cluster connect (tiled windows)
cr prod-cluster   # cluster connect via gateways
```

---

## Commands

### TUI

```bash
alogin tui
```

Launches the interactive terminal UI. Navigate with arrow keys, filter hosts with fuzzy search, and press `Enter` to connect. Same as `alogin connect` (no args) but starts at the welcome screen.

### Connection

```
alogin connect [user@]host... [flags]

  --auto-gw              Auto-detect gateway route (legacy 'r' behavior)
  --dry-run              Print connection route without connecting
  -c, --cmd string       Run command after login
  -L, --local-forward    Local port forward: PORT | LPORT:RPORT | LPORT:host:RPORT | lhost:LPORT:host:RPORT
  -R, --remote-forward   Reverse port forward (SSH -R): RPORT:lhost:LPORT | rhost:RPORT:lhost:LPORT
```

```bash
alogin connect                          # TUI selector
alogin connect web-01                   # direct connect
alogin connect gw-01 web-01             # explicit 2-hop
alogin connect gw-01 gw-02 web-01       # explicit 3-hop
alogin connect web-01 --auto-gw         # via registered gateway
alogin connect web-01 -L 2222:22        # forward local:2222 → web-01:22
alogin connect web-01 --auto-gw -L 2222:22  # gateway + port forward
```

### File Transfer

```bash
alogin sftp [user@]host [-p local_file] [-g remote_file]
alogin ftp  [user@]host
alogin mount [user@]host [remote_path]   # SSHFS mount
```

### Cluster

```bash
alogin cluster [name] [flags]

  --gateway          Route through gateways (legacy 'cr')
  --mode string      Session mode: tmux (default), iterm, terminal
  -x, --tile-x int   Number of tile columns (0=auto)
```

### Server Management

```bash
alogin server list
alogin server add [--proto ssh] [--host host] [--user user] [--gateway name] [--locale loc]
alogin server show [user@]host
alogin server delete [user@]host
alogin server passwd [user@]host
alogin server getpwd [user@]host    # show stored password
```

### Gateway Management

```bash
alogin gateway list
alogin gateway add
alogin gateway show name
alogin gateway delete name
```

### Alias Management

```bash
alogin alias list
alogin alias add
alogin alias show name
alogin alias delete name
```

### Migration

```bash
alogin migrate --from /path/to/alogin_root [--dry-run]
```

### Tunnel Management

```bash
alogin tunnel [name] [flags]
```

Keeps SSH port-forwards alive in tmux background sessions — survives terminal disconnect.

```bash
# Register a tunnel
alogin tunnel add db-local --server db.prod --local-port 5432 --remote-host db.prod --remote-port 5432
alogin tunnel add web-local --server web-01 --dir L --local-port 8080 --remote-host localhost --remote-port 80

# Start / stop / status
alogin tunnel start db-local    # spawn detached tmux session
alogin tunnel stop  db-local
alogin tunnel status db-local

# List / edit / remove
alogin tunnel list
alogin tunnel edit db-local --remote-port 5433
alogin tunnel rm   db-local

# Manage via TUI
alogin tunnel                   # opens tunnel management screen (/tunnel slash command)
```

`--dir L` (default): local forward (`-L localHost:localPort:remoteHost:remotePort`)
`--dir R`: reverse tunnel (`-R remotePort:localHost:localPort`)
`--auto-gw`: route through the server's registered gateway chain

### Web UI

```bash
alogin web [--port 8484] [--no-browser]
```

Opens `http://localhost:8484` automatically.

### Shell Completion

```bash
alogin completion install              # zsh (default)
alogin completion install --shell bash # bash
```

---

## Configuration

Default paths (XDG-compliant):

| Path | Description |
|------|-------------|
| `~/.local/share/alogin/alogin.db` | SQLite database |
| `~/.local/share/alogin/vault.age` | age-encrypted vault (fallback) |
| `~/.config/alogin/config.toml` | Configuration file |
| `~/.local/share/alogin/alogin.log` | Log file |

Override with environment variables:

```bash
ALOGIN_DB            # Path to SQLite database file
ALOGIN_CONFIG        # Path to config.toml
ALOGIN_LOG_LEVEL     # 0=errors, 1=info, 2=debug (default: 0)
ALOGIN_LANG          # Default locale (default: system)
ALOGIN_SSHOPT        # Extra SSH options
ALOGIN_SSHCMD        # Custom SSH binary path
ALOGIN_KEYCHAIN_USE  # If set, force Keychain backend
ALOGIN_ROOT          # Legacy: sets DB/config parent directory
```

`config.toml` example:

```toml
[ssh]
default_options = "-o StrictHostKeyChecking=no -o ServerAliveInterval=30"
connect_timeout = 10

[vault]
backend = "keychain"   # keychain | libsecret | age | plaintext

[web]
port = 8484
```

---

## Security

### Credential storage

Passwords are **never stored in the database**. The `password` column holds `_HIDDEN_` as a sentinel. Actual credentials are stored in:

1. **macOS Keychain** (default on macOS) — uses `Security.framework`
2. **Linux Secret Service** (default on Linux) — GNOME Keyring / KWallet via D-Bus
3. **age-encrypted file** — cross-platform fallback; unlocked with a master passphrase
4. **Plaintext** — only for reading legacy `server_list` during migration

### SSH key authentication

alogin respects `~/.ssh/config` and the SSH agent. For hosts where you've deployed an SSH key, alogin uses it automatically — no password entry needed.

---

## Multi-hop Gateway Routing

Define a gateway route:

```bash
alogin gateway add
# name: prod-route
# hops: bastion-01 → internal-gw → (destination)
```

Assign it to a server:

```bash
alogin server add
# ...
# gateway: prod-route
```

alogin dials each hop in order using Go's native SSH library — no ProxyCommand shell escaping, no expect patterns:

```
local → bastion-01:22 → internal-gw:22 → web-01:22
```

If an intermediate hop has `AllowTcpForwarding` disabled, alogin automatically falls back to the **shell-chain method** (runs `ssh -tt` inside the shell of each hop — identical to v1's `conn.exp` behavior).

---

## Cluster Sessions

Connect to all members of a cluster simultaneously:

```bash
alogin cluster prod-web --mode tmux      # tmux panes (macOS + Linux)
alogin cluster prod-web --mode iterm     # iTerm2 split panes (macOS)
alogin cluster prod-web --mode terminal  # Terminal.app tiles (macOS)
```

Manage clusters:

```bash
alogin cluster add prod-web web-01 web-02 web-03
alogin cluster list
alogin cluster show prod-web
alogin cluster delete prod-web
```

<!-- 📸 Screenshot #2: Cluster tmux session
     Scene: Full terminal screen after running `alogin cluster prod-web --mode tmux`.
     tmux splits the screen into 3–4 panes, each showing a live SSH session to a
     different server (web-01, web-02, web-03, etc.). Server names visible in each pane
     title bar or prompt.
-->
![Cluster tmux session](docs/screenshots/cluster-tmux.gif)

---

## Web UI

```bash
alogin web
```

Opens `http://localhost:8484` with:

- **Server list** — browse, search, connect
- **Web terminal** — full xterm.js SSH session in the browser
- **Cluster management** — create and edit clusters
- **Tunnel management** — add/edit/delete tunnels, start/stop/check status

<!-- 📸 Screenshot #3: Web browser SSH terminal
     Scene: An SSH session open in the browser via Web UI.
     The xterm.js terminal fills the right panel with a server prompt visible.
     Ideally a command like `ls -la` or `htop` is running to make it look lively.
-->
![Web browser SSH terminal](docs/screenshots/web-terminal.gif)

The web server is local-only by default. Do not expose it to the network without adding authentication.

---

## Migration from v1

alogin v2 is fully backward-compatible with v1 data files. Run once:

```bash
alogin migrate --from $ALOGIN_ROOT
```

This:
- Parses `server_list`, `gateway_list`, `alias_hosts`, `clusters`, `term_themes`
- Converts `<space>` / `<tab>` literals to real characters
- Imports all data into SQLite
- Moves passwords to the system keychain (removes from database)
- Leaves the original files untouched

After migration, source the compatibility shim to keep using `t`, `r`, `s`, etc.:

```bash
source <(alogin shell-init)
```

---

## Development

### Prerequisites

- Go 1.23+
- Node.js 20+ (for Web UI frontend only)

### Build

```bash
# CLI only
go build ./cmd/alogin

# Web UI frontend (required before embedding)
cd web/frontend && npm install && npm run build

# Full build with embedded Web UI
go build -tags web ./cmd/alogin
```

### Test

```bash
go test ./...
go vet ./...
```

### Cross-compile

```bash
make dist       # darwin/arm64, darwin/amd64, linux/amd64, linux/arm64
make dist-web   # macOS builds with embedded Web UI
```

### Project structure

```
cmd/alogin/          Entry point
internal/
  cli/               Cobra command definitions
  config/            Config loading (XDG + env vars)
  model/             Data types (Server, Gateway, Cluster ...)
  db/                SQLite repositories (schema: internal/db/schema.sql)
  vault/             Secret backends (Keychain / libsecret / age / plaintext)
  ssh/               Native SSH client (multi-hop, PTY, tunnel, SFTP, SSHFS)
  migrate/           v1 TSV → SQLite migration parsers
  cluster/           Cluster session orchestration (tmux / iTerm2 / Terminal.app)
  tui/               Bubbletea interactive host picker
  tunnel/            tmux-backed persistent SSH tunnel manager (start/stop/status)
  web/               HTTP server + WebSocket terminal + REST API
web/frontend/        React + xterm.js (Vite)
completions/         Shell shim + zsh/bash completion scripts
docs/                Detailed documentation
```

---

## License

MIT
