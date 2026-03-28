# Web API & Frontend Structure

Entry: `internal/cli/web.go` → `internal/web/server.go`

Default URL: `http://localhost:8484` (override: `--port` flag)

Build tag: `//go:build web` — without `-tags web`, the server starts but serves a placeholder page.

---

## HTTP router layout

```
internal/web/server.go       — chi router setup, static file serving
internal/web/api/handler.go  — REST endpoint handlers
internal/web/terminal/ws_pty.go — WebSocket PTY bridge
```

---

## REST API endpoints

Base path: `/api`

### Compute

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/compute` | List all servers |
| `POST` | `/api/compute` | Create server |
| `GET` | `/api/compute/{id}` | Get server by ID |
| `PUT` | `/api/compute/{id}` | Update server |
| `DELETE` | `/api/compute/{id}` | Delete server |

### Gateways

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/gateways` | List all gateways |
| `POST` | `/api/gateways` | Create gateway |
| `GET` | `/api/gateways/{id}` | Get gateway by ID |
| `PUT` | `/api/gateways/{id}` | Update gateway |
| `DELETE` | `/api/gateways/{id}` | Delete gateway |

### Clusters

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/clusters` | List all clusters |
| `POST` | `/api/clusters` | Create cluster |
| `GET` | `/api/clusters/{id}` | Get cluster by ID |
| `PUT` | `/api/clusters/{id}` | Update cluster |
| `DELETE` | `/api/clusters/{id}` | Delete cluster |

### Aliases

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/aliases` | List all aliases (read-only) |

### Local Hosts

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/hosts` | List all local hosts |
| `POST` | `/api/hosts` | Create local host |
| `GET` | `/api/hosts/{id}` | Get local host by ID |
| `PUT` | `/api/hosts/{id}` | Update local host |
| `DELETE` | `/api/hosts/{id}` | Delete local host |

### Tunnels

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

### App-Servers

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/app-servers` | List all app-server bindings |
| `POST` | `/api/app-servers` | Create app-server binding |
| `GET` | `/api/app-servers/{id}` | Get binding by ID |
| `PUT` | `/api/app-servers/{id}` | Update binding |
| `DELETE` | `/api/app-servers/{id}` | Delete binding |
| `POST` | `/api/app-servers/{id}/connect` | Return `{server_id, auto_gw, app}` for WebSocket terminal |

---

## WebSocket

| Path | Description |
|------|-------------|
| `GET /ws/terminal/{serverID}` | PTY terminal — bridges `ws_pty.go` ↔ SSH PTY session |

Query parameters: `?auto_gw=true` (route via gateway), `?app=<plugin>` (launch plugin after SSH, with PTY automation)

Protocol: xterm.js ↔ WebSocket ↔ `ws_pty.go` ↔ `internal/ssh/session.go`

---

## Static frontend

```
GET /*   → web/frontend/dist/  (embedded via web/embed.go)
```

- Framework: React + TypeScript + xterm.js (Vite build)
- Source: `web/frontend/src/`
- Build output: `web/frontend/dist/`

---

## Handler construction

The `Handler` struct requires `binPath` (path to the running `alogin` binary) to spawn `alogin tunnel run {name}` inside tmux:

```go
h := api.NewHandlerWithBin(db, vault, binPath)
```

Do **not** use the older `NewHandler` — it lacks `binPath` and tunnel start/stop will fail.

---

## Modifying the Web UI

- **Add a REST endpoint**: add route in `server.go`, implement handler method in `api/handler.go`.
- **Add a WebSocket handler**: register in `server.go`, implement in `terminal/` or a new sub-package.
- **Add a frontend feature**: edit `web/frontend/src/`, run `npm run build` inside `web/frontend/`, then rebuild Go binary with `-tags web`.
- **Adding a new resource (e.g. new table)**: follow the pattern of handlers for an existing resource (e.g. `/api/compute`), add CRUD routes + handler methods.
