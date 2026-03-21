# Web UI Notes

- Frontend: `web/frontend/src/` (React + TypeScript + xterm.js, built with Vite)
- Build output: `web/frontend/dist/` — embedded into Go binary via `web/embed.go`
- WebSocket terminal endpoint: `GET /ws/terminal/{serverID}` — bridges `internal/web/terminal/ws_pty.go` ↔ SSH PTY
- REST API: `internal/web/api/handler.go` — CRUD for servers/gateways/clusters
- HTTP server: `internal/web/server.go` — chi router + gorilla/websocket

The `web` build tag controls whether the frontend is embedded:
```go
//go:build web
// +build web
```

Without `-tags web`, the `alogin web` command still starts the server but serves a placeholder page.
