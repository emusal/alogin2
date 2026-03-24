# Package Map

| Package | Responsibility |
|---------|---------------|
| `cmd/alogin` | Entry point; wires cobra root command |
| `internal/cli` | All cobra command definitions (`connect.go`, `server.go`, `tunnel.go`, `mcp.go`, `upgrade.go`, etc.) |
| `internal/config` | XDG paths, `ALOGIN_*` env overlay, viper config loading |
| `internal/model` | Pure data types: `Server`, `GatewayRoute`, `Cluster`, `Alias`, `Tunnel`, `LocalHost`, `ConnectOptions` |
| `internal/db` | SQLite repositories — `ServerRepo`, `GatewayRepo`, `AliasRepo`, `ClusterRepo`, `ThemeRepo`, `LocalHostRepo`, `TunnelRepo` |
| `internal/vault` | Secret backends: `Keychain` (darwin), `LibSecret` (linux), `Age`, `Plaintext`; chain pattern |
| `internal/ssh` | SSH client, multi-hop dialer (`proxy.go`), shell-chain fallback (`shell_chain.go`), PTY session, tunnel, SFTP, SSHFS, Docker |
| `internal/tunnel` | tmux-backed persistent SSH tunnel manager: `Start`, `Stop`, `IsRunning`, `SessionName` |
| `internal/cluster` | Cluster session orchestration: tmux (all), iTerm2 (darwin), Terminal.app (darwin) |
| `internal/tui` | Bubbletea state-machine TUI: server/gateway/cluster/hosts/tunnel management panels |
| `internal/web` | HTTP server (`server.go`), static frontend embed, WebSocket PTY (`terminal/ws_pty.go`) |
| `internal/web/api` | REST CRUD handlers for all resources |
| `internal/web/terminal` | WebSocket PTY bridge (xterm.js ↔ SSH session) |
| `internal/mcp` | Model Context Protocol stdio server; 10 tools for LLM integration |
| `internal/migrate` | TSV parsers (`server_list`, `gateway_list`, etc.) → SQLite import |
| `internal/completion` | Shell completion script generators (zsh/bash) |
| `web/frontend` | React + TypeScript + xterm.js (Vite build → `web/frontend/dist`) |
| `completions` | Shell shim files (`alogin.zsh`, `alogin.bash`) providing `t`/`r`/`s`/`f`/`m`/`ct`/`cr` |
