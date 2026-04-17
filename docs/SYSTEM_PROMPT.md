# alogin System Prompt Guide

This document is a reference for LLMs (Claude, GPT, etc.) using alogin as an MCP server. Copy the recommended prompt snippet into your AI client's system prompt configuration.

---

## Recommended System Prompt Snippet

```
You have access to alogin, a secure SSH gateway for agentic infrastructure access.

Core workflow:
1. DISCOVER — call list_servers or list_clusters before acting on any host
2. SHELL — call remote_shell(target) to open a persistent session (get session_id), then
           call remote_shell(target, session_id, command) for each subsequent command.
           This is the PREFERRED tool for all remote work. Use exec_command only as fallback.
3. INSPECT — call inspect_node to understand a server's current state before making changes
4. ACT — use remote_shell for sequences; exec_command only for single one-shot commands
5. VERIFY — re-inspect or run a read-only check to confirm the change took effect
6. CLEANUP — call remote_shell(target, session_id, command="exit") when done

Safety rules:
- Always provide an "intent" parameter when calling exec_command, exec_on_cluster, or remote_shell
- Do not run destructive commands (rm -rf, shutdown, reboot, DROP TABLE) without explicit user confirmation
- Prefer read-only inspection before any write operation
- If a server has device_type "router", "switch", or "firewall", do not assume standard Linux commands work
- When managing tunnels, check list_tunnels first to avoid starting duplicates
- For any remote shell task, always try remote_shell first before exec_command
- To edit a remote file, prefer remote_replace over cat+run_script; it is safer and handles special characters correctly
```

---

## Overview

alogin exposes 16 MCP tools over stdio (JSON-RPC 2.0). It manages:
- A server registry with encrypted credential vault
- Multi-hop SSH gateway routing
- Cluster session groups
- Persistent named SSH tunnels (tmux-backed)

All `exec_command`, `exec_on_cluster`, `inspect_node`, and `remote_shell` calls are logged to `~/.config/alogin/audit.jsonl`.

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

#### `remote_shell`
**Primary and preferred tool for ALL remote shell access.** Use this first for any remote server interaction. Provides a persistent SSH connection identified by a `session_id` — call it repeatedly with the same `session_id` to run multiple commands on the same server.

> **Note:** Each command runs inside a persistent bash process, so `cd`, exported variables, and shell state carry over between calls.
>
> **Reconnect:** If an i/o timeout drops the TCP connection, `remote_shell` automatically reconnects and retries the command on the same `session_id`. The response will include `"reconnected": true`.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `target` | string/number | yes | Server ID from list_servers |
| `command` | string | no | Command to run. Omit to establish session only. Use `"exit"` to close. |
| `session_id` | string | no | Reuse an existing session. Omit to create a new one. |
| `pty` | boolean | no | Allocate a PTY. Required for TUI programs: `top`, `watch`, `vi`, `htop`, etc. Default false. |
| `login_shell` | boolean | no | Start bash as a login shell (`bash -l`) to source `~/.bash_profile`. Enables PATH, nvm, pyenv, rbenv, etc. **Default true.** Set `false` only when a pristine environment is required. |
| `timeout_sec` | number | no | Per-command timeout in seconds (default 120). PTY commands are sent SIGINT after this duration. |
| `agent_id` | string | no | Agent identifier (logged to audit) |
| `intent` | string | no | Human-readable intent (logged to audit) |

Session lifecycle example:
```json
// Step 1: establish session
{"tool": "remote_shell", "arguments": {"target": "3", "intent": "deploy app"}}
// → {"session_id": "abc-123", "status": "created"}

// Step 2: run commands (reuse session_id)
{"tool": "remote_shell", "arguments": {"target": "3", "session_id": "abc-123", "command": "ls /var/log"}}
{"tool": "remote_shell", "arguments": {"target": "3", "session_id": "abc-123", "command": "cd /app && python manage.py migrate"}}

// Step 3: close when done
{"tool": "remote_shell", "arguments": {"target": "3", "session_id": "abc-123", "command": "exit"}}
// → {"session_id": "abc-123", "status": "closed"}
```

Returns on command execution:
```json
{"session_id": "abc-123", "results": [{"command": "ls /var/log", "output": "...", "exit_code": 0}]}
```

---

#### `run_script`
Upload and execute a script on a remote server in a single atomic call. The script content is passed as a string — no local file or quoting gymnastics required. The script is uploaded to a temp path via SFTP, executed, then automatically deleted.

**Use this instead of `push_file` + `remote_shell` when you want to run a multi-line script.**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `server_id` | string/number | yes | Server ID from list_servers |
| `content` | string | yes | Script source code to upload and run |
| `interpreter` | string | no | Interpreter to use (default: `bash`). Examples: `python3`, `sh`, `ruby` |
| `login_shell` | boolean | no | Run via a login shell (`bash -l`) so `~/.bash_profile` is sourced. Default false. |
| `timeout_sec` | number | no | Execution timeout in seconds (default 120) |
| `agent_id` | string | no | Agent identifier (logged to audit) |
| `intent` | string | no | Human-readable intent (logged to audit) |

Example:
```json
{
  "tool": "run_script",
  "arguments": {
    "server_id": "3",
    "content": "#!/bin/bash\napt list --installed 2>/dev/null | grep nginx",
    "intent": "check nginx version"
  }
}
```

Returns:
```json
{"server_id": 3, "output": "nginx/stable,now 1.24.0 ...", "exit_code": 0}
```

---

#### `remote_replace`
Safely replace a string inside a remote file in-place. Reads the file, substitutes `old_string` with `new_string`, and writes it back. Use this instead of `cat` + `run_script` when editing config files or code — it avoids full-file overwrites and handles arbitrary special characters safely.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `server_id` | string/number | yes | Server ID from list_servers |
| `path` | string | yes | Absolute path of the remote file to edit |
| `old_string` | string | yes | Exact literal string to search for (case-sensitive, not a regex) |
| `new_string` | string | yes | Replacement string. May be empty to delete `old_string`. |
| `all` | boolean | no | Replace all occurrences. Default true. Set false for first-only. |
| `agent_id` | string | no | Agent identifier (logged) |
| `intent` | string | no | Human-readable intent (logged) |

Returns:
```json
{"server_id": 3, "path": "/etc/nginx/nginx.conf", "replaced": 2}
```
`replaced: 0` means `old_string` was not found; the file is unchanged.

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

All `exec_command`, `exec_on_cluster`, `inspect_node`, and `remote_shell` calls are appended to:

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
