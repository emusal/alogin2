---
name: alogin
description: Securely access SSH servers, run remote commands, inspect node health, and manage persistent SSH tunnels via alogin — a secure gateway for agentic infrastructure access. Use when the user wants to connect to servers, run commands on remote hosts, check disk/CPU/memory, manage clusters, or control SSH tunnels.
license: Apache-2.0
compatibility: Requires alogin v2.2.0+ installed and configured with at least one server in the registry. Run `alogin agent mcp` as an MCP server in Claude Desktop or any MCP-compatible client.
metadata:
  author: emusal
  version: '2.2.0'
  homepage: https://github.com/emusal/alogin2
  mcp-transport: stdio
---

# alogin — Agentic SSH Gateway

alogin is a secure SSH gateway that lets AI agents safely access infrastructure without handling credentials or ProxyJump routing directly. The human administrator provisions trust once; agents operate freely within it.

## Core Workflow

Always follow this sequence:

```
DISCOVER → INSPECT → ACT → VERIFY
```

1. **DISCOVER** — call `list_servers` or `list_clusters` to find target IDs
2. **INSPECT** — call `inspect_node` to understand the server's current state before changes
3. **ACT** — call `exec_command` or `exec_on_cluster` with a clear `intent` parameter
4. **VERIFY** — re-run a read-only command to confirm the outcome

## Available Tools

### Query tools (read-only)

| Tool            | When to use                                                          |
| --------------- | -------------------------------------------------------------------- |
| `list_servers`  | Find servers by host, user, or note                                  |
| `get_server`    | Get full details (gateway route, device type) for one server         |
| `list_clusters` | See all cluster groups                                               |
| `get_cluster`   | Get cluster members before running cluster-wide commands             |
| `list_tunnels`  | Check tunnel status before starting one                              |
| `get_tunnel`    | Get details for a specific tunnel                                    |
| `inspect_node`  | Get CPU load, memory, disk, top processes — always run before writes |

### Execution tools (write)

| Tool              | When to use                                                               |
| ----------------- | ------------------------------------------------------------------------- |
| `exec_command`    | Run commands on a single server; use `expect` for interactive/PTY prompts |
| `exec_on_cluster` | Run commands on all cluster members in parallel                           |

### Tunnel lifecycle tools

| Tool           | When to use                                     |
| -------------- | ----------------------------------------------- |
| `start_tunnel` | Start a saved tunnel in a detached tmux session |
| `stop_tunnel`  | Stop a running tunnel                           |

## Safety Rules

- Always set `intent` when calling `exec_command` or `exec_on_cluster` — it is logged to the audit trail
- Do **not** run destructive commands (`rm -rf`, `shutdown`, `reboot`, `DROP TABLE`) without explicit user confirmation
- Always call `inspect_node` before any write operation
- If `device_type` is `router`, `switch`, or `firewall`, do not assume standard Linux commands work
- Check `list_tunnels` before `start_tunnel` to avoid starting duplicates

## Example Workflows

### Check disk usage across a cluster

```
1. list_clusters                                   → find cluster ID
2. get_cluster(id)                                 → review members and device types
3. exec_on_cluster(id, ["df -h"], intent="disk pre-flight check")
```

### Deploy a change to a single server

```
1. list_servers(query="web-01")                    → get server ID
2. inspect_node(server_id)                         → confirm current state
3. exec_command(server_id, ["your command"], intent="reason")
4. exec_command(server_id, ["verify command"])     → confirm outcome
```

### Manage a persistent SSH tunnel

```
1. list_tunnels                                    → check if already running
2. start_tunnel(id)                                → start if not running
3. [use the tunnel for local port-forwarding]
4. stop_tunnel(id)                                 → clean up when done
```

## Audit Trail

Every `exec_command`, `exec_on_cluster`, and `inspect_node` call is appended to:

```
~/.config/alogin/audit.jsonl
```

Each entry records `timestamp`, `event`, `agent_id`, `server_host`, `commands`, `intent`, and `timeout_sec`.

## Setup

Add to Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

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

Full tool reference: [docs/SYSTEM_PROMPT.md](docs/SYSTEM_PROMPT.md)
