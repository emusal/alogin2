# SKILL.md — alogin v2 Agent Skills

This document describes what alogin can do for an AI agent that connects via the built-in MCP server (`alogin agent mcp`).

---

## Setup

Add the following to your Claude Desktop (or any MCP-compatible client) configuration:

```json
{
  "mcpServers": {
    "alogin": {
      "command": "alogin",
      "args": ["agent", "mcp"]
    }
  }
}
```

A full system prompt guide is available at [docs/SYSTEM_PROMPT.md](docs/SYSTEM_PROMPT.md).

---

## Core Workflow

```
DISCOVER → INSPECT → ACT → VERIFY
```

1. **DISCOVER** — `list_servers` or `list_clusters` to find targets
2. **INSPECT** — `inspect_node` to understand current state before changes
3. **ACT** — `exec_command` or `exec_on_cluster` with a clear `intent`
4. **VERIFY** — re-run a read-only check to confirm the outcome

---

## MCP Tools

### Query (read-only)

| Tool | Description |
|------|-------------|
| `list_servers` | List/search all servers in the registry |
| `get_server` | Get full details for a single server |
| `list_clusters` | List all cluster groups with member counts |
| `get_cluster` | Get a cluster with full member details |
| `list_tunnels` | List tunnel configurations with live running status |
| `get_tunnel` | Get details and status for a single tunnel |
| `inspect_node` | Get a structured health snapshot (CPU, memory, disk, top processes) |

### Execution (write)

| Tool | Description |
|------|-------------|
| `exec_command` | Run SSH commands on a single server (non-interactive or PTY mode) |
| `exec_on_cluster` | Run SSH commands on all cluster servers in parallel |

### Tunnel Lifecycle

| Tool | Description |
|------|-------------|
| `start_tunnel` | Start a saved tunnel in a detached tmux session |
| `stop_tunnel` | Stop a running tunnel |

> All `exec_command`, `exec_on_cluster`, and `inspect_node` calls are appended to `~/.config/alogin/audit.jsonl`.

---

## Safety Rules

- Always provide an `intent` parameter when calling execution tools
- Do not run destructive commands (`rm -rf`, `shutdown`, `reboot`, `DROP TABLE`) without explicit user confirmation
- Prefer `inspect_node` before any write operation
- If a server has `device_type` of `router`, `switch`, or `firewall`, do not assume standard Linux commands work
- Check `list_tunnels` before `start_tunnel` to avoid duplicates

---

## Example Workflows

### Check disk usage across a cluster

```
1. list_clusters                          → find cluster ID
2. get_cluster(id)                        → review members and device types
3. exec_on_cluster(id, ["df -h"], intent="disk pre-flight check")
```

### Deploy a config change to a single server

```
1. list_servers(query="web-01")           → get server ID
2. inspect_node(server_id)               → confirm current state
3. exec_command(server_id, ["your command"], intent="reason for change")
4. exec_command(server_id, ["verification command"])
```

### Manage a persistent SSH tunnel

```
1. list_tunnels                           → check if already running
2. start_tunnel(id)                       → start if not running
3. [use the tunnel]
4. stop_tunnel(id)                        → clean up when done
```

---

## Audit Trail

Every execution is appended to `~/.config/alogin/audit.jsonl`:

```json
{
  "timestamp": "2026-03-27T10:00:00Z",
  "event": "exec_command",
  "agent_id": "claude-desktop/session-abc",
  "server_id": 3,
  "server_host": "web-01.prod",
  "commands": ["df -h"],
  "intent": "checking disk before deploy",
  "timeout_sec": 30
}
```
