# alogin System Prompt Guide

This document is a reference for LLMs (Claude, GPT, etc.) using alogin as an MCP server. Copy the recommended prompt snippet into your AI client's system prompt configuration.

---

## Recommended System Prompt Snippet

```
You have access to alogin, a secure SSH gateway for agentic infrastructure access.

Core workflow:
1. DISCOVER — call list_servers or list_clusters before acting on any host
2. INSPECT — call inspect_node to understand a server's current state before making changes
3. ACT — use exec_command or exec_on_cluster with a clear intent parameter
4. VERIFY — re-inspect or run a read-only check to confirm the change took effect

Safety rules:
- Always provide an "intent" parameter when calling exec_command or exec_on_cluster
- Do not run destructive commands (rm -rf, shutdown, reboot, DROP TABLE) without explicit user confirmation
- Prefer read-only inspection before any write operation
- If a server has device_type "router", "switch", or "firewall", do not assume standard Linux commands work
- When managing tunnels, check list_tunnels first to avoid starting duplicates
```

---

## Overview

alogin exposes 11 MCP tools over stdio (JSON-RPC 2.0). It manages:
- A server registry with encrypted credential vault
- Multi-hop SSH gateway routing
- Cluster session groups
- Persistent named SSH tunnels (tmux-backed)

All `exec_command` and `exec_on_cluster` calls are logged to `~/.config/alogin/audit.jsonl`.

---

## Tool Reference

### Query tools (read-only)

#### `list_servers`
List all servers in the registry.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | no | Filter by host, user, or note |

Returns: array of `{id, host, user, protocol, device_type, note, gateway_id}`

Example:
```json
{"tool": "list_servers", "arguments": {"query": "prod"}}
```

---

#### `get_server`
Get full details for a single server.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | string/number | yes | Server ID from list_servers |

---

#### `list_tunnels`
List all saved tunnel configurations with live running status.

Returns: array of `{id, name, server, direction, local_port, remote_host, remote_port, running}`

---

#### `get_tunnel`
Get details and running status for a single tunnel.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | string/number | yes | Tunnel ID from list_tunnels |

---

#### `list_clusters`
List all cluster groups with member counts.

---

#### `get_cluster`
Get a cluster with full member server details (host, user, device_type, note).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | string/number | yes | Cluster ID from list_clusters |

---

#### `inspect_node`
Get a structured health snapshot of a server: CPU load averages, memory usage, root disk usage, and top processes by CPU. Falls back to raw command output if the server's output format cannot be parsed.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `server_id` | string/number | yes | Server ID |
| `timeout_sec` | number | no | Timeout (default 30) |
| `agent_id` | string | no | Agent identifier (logged) |
| `intent` | string | no | Human-readable intent (logged) |

Returns:
```json
{
  "server_id": 3,
  "host": "web-01.prod",
  "cpu": {"load1": 0.52, "load5": 0.41, "load15": 0.38},
  "memory": {"total_bytes": 8388608000, "used_bytes": 4194304000, "free_bytes": 4194304000, "used_pct": 50.0},
  "disk": {"total_bytes": 107374182400, "used_bytes": 21474836480, "free_bytes": 85899345920, "used_pct": 20.0},
  "top_processes": [
    {"user": "www-data", "pid": "1234", "cpu_pct": 12.5, "mem_pct": 2.1, "command": "nginx: worker process"}
  ]
}
```

---

### Execution tools (write)

#### `exec_command`
Run SSH commands on a single server.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `server_id` | string/number | yes | Server ID |
| `commands` | string[] | yes | Commands to run |
| `expect` | object[] | no | PTY mode: `[{"pattern": "...", "send": "..."}]` |
| `timeout_sec` | number | no | Per-command timeout (default 30) |
| `agent_id` | string | no | Agent identifier (logged to audit) |
| `intent` | string | no | Human-readable intent (logged to audit) |

Non-interactive mode (no `expect`): each command runs in its own SSH session.
Interactive/PTY mode (with `expect`): all commands run as one PTY session with auto-responses to prompts.

Example (read-only inspection):
```json
{
  "tool": "exec_command",
  "arguments": {
    "server_id": "3",
    "commands": ["df -h", "free -m", "uptime"],
    "intent": "checking disk and memory before deploying"
  }
}
```

---

#### `exec_on_cluster`
Run SSH commands on all servers in a cluster in parallel. Individual failures are captured without stopping other servers.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cluster_id` | string/number | yes | Cluster ID |
| `commands` | string[] | yes | Commands to run on each server |
| `expect` | object[] | no | PTY mode rules |
| `timeout_sec` | number | no | Per-server timeout (default 30) |
| `agent_id` | string | no | Agent identifier (logged) |
| `intent` | string | no | Human-readable intent (logged) |

---

### Tunnel lifecycle tools

#### `start_tunnel`
Start a saved tunnel in a detached tmux session.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | string/number | yes | Tunnel ID |

Returns: `{"status": "started", "session": "alogin-tunnel-db-local"}`

---

#### `stop_tunnel`
Stop a running tunnel by killing its tmux session.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | string/number | yes | Tunnel ID |

Returns: `{"status": "stopped"}`

---

## Recommended Workflows

### Before modifying a server

```
1. list_servers → get server ID
2. inspect_node(server_id) → confirm current state
3. exec_command(server_id, ["your command"], intent="reason")
4. exec_command(server_id, ["verification command"])
```

### Managing tunnels

```
1. list_tunnels → check if tunnel already running
2. start_tunnel(id) if not running
3. [use the tunnel]
4. stop_tunnel(id) when done
```

### Cluster-wide operations

```
1. list_clusters → find cluster ID
2. get_cluster(id) → review member list and device types
3. exec_on_cluster(cluster_id, ["read-only check"], intent="pre-flight")
4. exec_on_cluster(cluster_id, ["actual command"], intent="reason for change")
```

---

## Audit Trail

All `exec_command`, `exec_on_cluster`, and `inspect_node` calls are appended to:

```
~/.config/alogin/audit.jsonl
```

Each line is a JSON object:
```json
{
  "timestamp": "2026-03-26T10:00:00Z",
  "event": "exec_command",
  "agent_id": "claude-desktop/session-abc",
  "server_id": 3,
  "server_host": "web-01.prod",
  "commands": ["df -h"],
  "intent": "checking disk before deploy",
  "timeout_sec": 30
}
```
