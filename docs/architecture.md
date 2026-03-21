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
