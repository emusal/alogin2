# Architecture

## Connection flow

```
alogin connect [host]
  │
  ├─ internal/cli/connect.go     — parse args, resolve user/host
  ├─ internal/db/server_repo.go  — look up server record + gateway chain
  ├─ internal/vault/             — fetch password (Keychain/libsecret/age)
  ├─ internal/ssh/proxy.go       — DialChain: hop1 → hop2 → ... → dest
  └─ internal/ssh/session.go     — interactive PTY session (SIGWINCH forwarded)
```

## Key design decisions

1. **No `expect`** — `golang.org/x/crypto/ssh` provides a programmatic SSH client. All prompt detection, password injection, and multi-hop routing from the old `conn.exp` is replaced by native Go code.

2. **Multi-hop SSH pattern** (`internal/ssh/proxy.go`):
   ```go
   hop1, _ := Dial(hops[0])
   raw, _  := hop1.client.Dial("tcp", hops[1].addr())
   hop2, _ := newClientFromConn(raw, hops[1].config())
   // ... continue chaining
   ```

3. **Vault chain** (`internal/vault/vault.go`): `ChainVault` tries backends in order — Keychain (darwin) → libsecret (linux) → age → plaintext. Build tags (`//go:build darwin`, `//go:build linux`) isolate platform-specific code.

4. **SQLite instead of TSV** (`internal/db/`): Schema in `internal/db/schema.sql` (embedded via `//go:embed`). `port=0` means "use protocol default" — same semantics as `-` in the old TSV. `password` column holds `_HIDDEN_` when vault is active.

5. **No CGO** — `modernc.org/sqlite` is a pure-Go SQLite port; enables cross-compilation without a C toolchain.

6. **Persistent tunnels via tmux** (`internal/tunnel/manager.go`): `alogin tunnel run <name>` is spawned inside a named tmux session. `Start`/`Stop`/`IsRunning` manage the lifecycle. Tunnel configs live in the `tunnels` DB table.

7. **MCP server** (`internal/mcp/`): Model Context Protocol server over stdio, exposes 10 tools for LLM integration (list_servers, exec_command, tunnel lifecycle, cluster ops).

## Layer overview

```
cmd/alogin/main.go
  └─ internal/cli/          ← cobra commands (one file per command)
       ├─ connect.go
       ├─ server.go / gateway.go / alias.go / hosts.go
       ├─ cluster.go / tunnel.go
       ├─ tui.go / web.go / mcp.go
       ├─ upgrade.go / db_migrate.go / uninstall.go
       └─ root.go            ← PersistentPreRunE (DB init, skip-list)

  ├─ internal/db/            ← SQLite repos (schema.sql embedded)
  ├─ internal/model/         ← plain data structs (no deps)
  ├─ internal/vault/         ← pluggable secret backends
  ├─ internal/ssh/           ← SSH client, PTY, SFTP, SSHFS, proxy chain
  ├─ internal/tunnel/        ← tmux tunnel manager
  ├─ internal/cluster/       ← multi-host session orchestration
  ├─ internal/tui/           ← Bubbletea TUI (state machine)
  ├─ internal/web/           ← HTTP server + REST API + WebSocket PTY
  ├─ internal/mcp/           ← MCP stdio server
  ├─ internal/migrate/       ← v1 TSV → SQLite import
  ├─ internal/completion/    ← shell completion generators
  └─ internal/config/        ← XDG paths, ALOGIN_* env vars
```
