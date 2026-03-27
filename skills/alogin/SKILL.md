---
name: alogin
description: Securely access SSH servers, run remote commands, inspect node health, and manage persistent SSH tunnels via the alogin MCP server. Use when you need to connect to servers, run commands on remote hosts, check disk/CPU/memory, manage clusters, or control SSH tunnels.
license: Apache-2.0
compatibility: Requires the `alogin` MCP server connected via stdio. Available via Smithery.
metadata:
  author: emusal
  version: '2.2.0'
  homepage: https://github.com/emusal/alogin2
  mcp-transport: stdio
---

# alogin — Agentic SSH Gateway (MCP Tools)

This skill provides instructions on how to orchestrate the **`alogin` MCP tools** exposed over a `stdio` connection. 
Because the `alogin` MCP server automatically handles SSH routing, ProxyJumps, and credential injection, **you do not need to use CLI commands (like `ssh`, `bash`, or `curl`) directly on your local machine.** Instead, you MUST exclusively use the provided MCP tools to manage remote infrastructure securely.

## Core Workflow (Plan-Validate-Execute)

Always follow this validation loop when interacting with infrastructure:
1. **DISCOVER** — Use the `list_servers` or `list_clusters` MCP tools to find the target IDs of your remote hosts.
2. **INSPECT (Validate)** — Call the `inspect_node` tool to understand the server's current state (CPU, memory, disk, background processes) BEFORE attempting any state changes.
3. **ACT (Execute)** — Call `exec_command` or `exec_on_cluster`. You MUST specify a descriptive `intent` parameter explaining why you are taking this action for the audit log.
4. **VERIFY** — Re-run a read-only command or `inspect_node` to verify your changes succeeded without unintended side-effects.

## Available MCP Tools Overview

### Query tools (read-only)
- `list_servers`: Find servers by host, user, or note.
- `get_server`: Get full details (gateway routes, device types) for one server.
- `list_clusters`: See all cluster groups.
- `get_cluster`: Get all member servers of a cluster.
- `list_tunnels`: Check all SSH tunnel statuses.
- `get_tunnel`: Get details for a specific tunnel.
- `inspect_node`: Get CPU load, memory, disk, top processes — **always run before writes**.

### Execution tools (write)
- `exec_command`: Run commands on a single server. Use standard Linux commands unless the device type is restricted.
- `exec_on_cluster`: Run commands on all cluster members in parallel.

### Tunnel lifecycle tools
- `start_tunnel`: Start a saved tunnel in a detached background session.
- `stop_tunnel`: Stop a running tunnel.

## Safety Rules & Gotchas

### Gotchas
- **Non-Linux devices**: If the server's `device_type` is `router`, `switch`, or `firewall`, do not assume standard POSIX commands (like `bash` or `df -h`) will work. Use network-specific syntax via `exec_command`.
- **Duplicate Tunnels**: Always check `list_tunnels` before calling `start_tunnel` to avoid conflicts on local ports.
- **Audit Logging**: Every write and inspect call is strictly recorded by the MCP server along with your `intent`. Do not omit reasoning.

### Safety Checklist
- [ ] Did I call `inspect_node` before changing state?
- [ ] Is my `intent` string detailed enough to explain **why** this action was taken?
- [ ] Did I check for potentially destructive commands (`rm -rf`, `shutdown`, `reboot`, `DROP TABLE`)? Do **not** run these without explicit user confirmation.

## Example Procedures

### 1. Check disk usage across a cluster
1. Call the `list_clusters` tool to find the target cluster ID.
2. Call `get_cluster(id)` to review its members and verify they are standard Linux hosts.
3. Call `exec_on_cluster(id, ["df -h"], intent="disk pre-flight check")`.
4. Parse the tabular output to ensure no node is at 100% capacity.

### 2. Manage a persistent SSH tunnel
1. Call `list_tunnels` to verify if the tunnel is already running.
2. If inactive, call `start_tunnel(id)`.
3. Use the local port-forwarding for your task.
4. Once completed, always call `stop_tunnel(id)` to clean up resources.

## Smithery & MCP Configuration Note

If deployed via **Smithery**, the MCP server is initialized automatically using the `stdio` transport. The tools listed above will be seamlessly injected into the LLM context. No local command-line installation (e.g., `brew` or `curl`) is required by the agent. If manual configuration in a client (like Claude Desktop) is needed, ensure the transport is set to `stdio` executing the `alogin agent mcp` command.
