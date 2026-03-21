# Package Map

| Package | Responsibility |
|---------|---------------|
| `cmd/alogin` | Entry point; wires cobra root command |
| `internal/cli` | All cobra command definitions (`connect.go`, `server.go`, etc.) |
| `internal/config` | XDG paths, `ALOGIN_*` env overlay, viper config loading |
| `internal/model` | Pure data types: `Server`, `GatewayRoute`, `Cluster`, `Alias`, `ConnectOptions` |
| `internal/db` | SQLite repositories — `ServerRepo`, `GatewayRepo`, `AliasRepo`, `ClusterRepo`, `ThemeRepo` |
| `internal/vault` | Secret backends: `Keychain` (darwin), `LibSecret` (linux), `Age`, `Plaintext` |
| `internal/ssh` | SSH client, multi-hop dialer (`proxy.go`), shell-chain fallback (`shell_chain.go`), PTY session, tunnel, SFTP, SSHFS, Docker |
| `internal/migrate` | TSV parsers (`server_list`, `gateway_list`, etc.) → SQLite import |
| `internal/cluster` | Cluster session orchestration: tmux (all), iTerm2 (darwin), Terminal.app (darwin) |
| `internal/tui` | Bubbletea interactive host picker (fuzzy search, arrow navigation) |
| `internal/completion` | Shell completion script generators (zsh/bash) |
| `internal/web` | HTTP server, WebSocket terminal, REST API, React frontend embed |
| `web/frontend` | React + TypeScript + xterm.js (Vite build → `web/frontend/dist`) |
| `completions` | Shell shim (`alogin.zsh`, `alogin.bash`) providing `t`/`r`/`s`/`f`/`m`/`ct`/`cr` |
