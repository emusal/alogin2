# Web UI Notes

- Frontend: `web/frontend/src/` (React + TypeScript + xterm.js, built with Vite)
- Build output: `web/frontend/dist/` — embedded into Go binary via `web/embed.go`
- WebSocket terminal endpoint: `GET /ws/terminal/{serverID}` — bridges `internal/web/terminal/ws_pty.go` ↔ SSH PTY
- REST API: `internal/web/api/handler.go` — CRUD for servers/gateways/clusters/tunnels
- HTTP server: `internal/web/server.go` — chi router + gorilla/websocket

The `web` build tag controls whether the frontend is embedded:
```go
//go:build web
// +build web
```

Without `-tags web`, the `alogin web` command still starts the server but serves a placeholder page.

## Tunnel REST API

Registered under `/api/tunnels`:

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/tunnels` | List all tunnel configs |
| `POST` | `/api/tunnels` | Create tunnel |
| `GET` | `/api/tunnels/{id}` | Get tunnel by ID |
| `PUT` | `/api/tunnels/{id}` | Update tunnel |
| `DELETE` | `/api/tunnels/{id}` | Delete tunnel (stops if running) |
| `POST` | `/api/tunnels/{id}/start` | Start tunnel (spawns tmux session) |
| `POST` | `/api/tunnels/{id}/stop` | Stop tunnel (kills tmux session) |
| `GET` | `/api/tunnels/{id}/status` | Check running state |

The `Handler` struct requires `binPath` (path to the running `alogin` binary) to spawn `alogin tunnel run {name}` inside tmux. Use `NewHandlerWithBin(db, vault, binPath)` instead of the older `NewHandler`.
