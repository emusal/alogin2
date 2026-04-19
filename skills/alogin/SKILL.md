---
name: alogin
description: >
  Agentic SSH access and remote server operations via alogin.
  Prefers MCP tools (remote_shell, exec_command, inspect_node, run_script,
  remote_replace) when available; falls back to alogin CLI automatically.
  Use for: connecting to servers, health inspection, persistent shell sessions,
  cluster execution, file transfer, background jobs, tunnel management,
  and server memory. Does NOT include admin operations (approve/deny, vault
  read, policy set) — see alogin-admin for those.
license: Apache-2.0
metadata:
  openclaw:
    requires:
      bins: [alogin]
    homepage: https://github.com/emusal/alogin2
---

# alogin — Agent Skill (MCP-first)

Use this skill when the task involves remote server access, health inspection,
persistent shell sessions, cluster execution, file transfer, or tunnel management.

Common triggers: SSH, remote shell, server login, bastion, health check,
inspect node, run command, persistent session, cluster, SFTP, background job,
server memory, `alogin`.

> **MCP vs CLI priority**
>
> If MCP tools are available in your environment, always prefer them over the
> alogin CLI — they carry richer audit metadata, handle reconnection
> automatically, and keep the session state server-side.
>
> | Task | MCP (preferred) | CLI (fallback) |
> |------|-----------------|----------------|
> | Persistent shell | `remote_shell` | `alogin ssh session exec` |
> | One-shot command | `exec_command` | `alogin ssh connect --cmd` |
> | Health snapshot | `inspect_node` | `alogin ssh connect --cmd "..."` |
> | Upload + run script | `run_script` | `alogin ssh run-script` |
> | Edit remote file | `remote_replace` | `alogin ssh session exec sed` |
> | File transfer | `push_file` / `pull_file` | `alogin scp push/pull` |
> | Cluster exec | `exec_on_cluster` | `alogin ssh cluster <name> --cmd` |
> | List servers | `list_servers` | `alogin server list --format json` |
> | Tunnel manage | `start_tunnel` / `stop_tunnel` | `alogin net tunnel start/stop` |
> | Server memory | `get_memory` / `save_memory` | `alogin agent server-memory list/add` |

---

## Standard Workflow

```
1. DISCOVER  — list_servers / alogin server list --format json
2. RECALL    — get_memory(server_id) before any work on a known server
3. SHELL     — remote_shell(target) → session_id, then reuse for every command
4. INSPECT   — inspect_node before making changes
5. ACT       — remote_shell for sequences; exec_command only for one-shot
6. VERIFY    — read-only check after change
7. SAVE      — save_memory() with findings before closing session
8. CLEANUP   — remote_shell(target, session_id, command="exit")
```

Always supply an `intent` parameter on execution tools — it is written to the
audit log and is the primary signal for HITL reviewers.

---

## MCP Tool Reference

### Query tools (read-only)

#### `list_servers`
List servers in the registry.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | no | Filter by host, user, or note |

Returns: `[{id, host, user, protocol, device_type, note, gateway_id}]`

---

#### `get_server`
Full detail for one server.

| Parameter | Type | Required |
|-----------|------|----------|
| `id` | string/number | yes |

---

#### `list_clusters` / `get_cluster`
List cluster groups or get one with full member detail.

| Parameter | Type | Required |
|-----------|------|----------|
| `id` (get_cluster) | string/number | yes |

---

#### `inspect_node`
Structured health snapshot: CPU load, memory, disk, top processes.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `server_id` | string/number | yes | |
| `timeout_sec` | number | no | default 30 |
| `agent_id` | string | no | logged |
| `intent` | string | no | logged |

---

#### `list_tunnels` / `get_tunnel`
List saved tunnels with live running status, or get one by ID.

---

#### `get_memory`
Retrieve operational notes written by previous agents for this server.
**Call immediately after identifying the target server.**

| Parameter | Type | Required |
|-----------|------|----------|
| `server_id` | string/number | yes |

Returns: `[{id, server_id, content, created_at}]`

---

#### `save_memory`
Record lasting operational knowledge (PATH quirks, version constraints,
service layout). Write entries that will be useful three sessions from now.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `server_id` | string/number | yes | |
| `content` | string | yes | Free-form operational note |

---

### Execution tools (write)

#### `remote_shell` ⭐ Primary tool
Persistent SSH session identified by `session_id`. Shell state (cwd, env vars)
carries across calls. Reconnects automatically on TCP drop.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `target` | string/number | yes | Server ID |
| `command` | string | no | Omit to open session; `"exit"` to close |
| `session_id` | string | no | Omit to create new session |
| `pty` | boolean | no | Allocate PTY for TUI programs (top, vi, htop) |
| `login_shell` | boolean | no | Source `~/.bash_profile` (default true) |
| `timeout_sec` | number | no | Per-command wall-clock limit (default 120) |
| `agent_id` | string | no | logged |
| `intent` | string | no | logged |

```json
// Open session
{"tool": "remote_shell", "arguments": {"target": "3", "intent": "deploy app"}}
// → {"session_id": "abc-123", "status": "created"}

// Reuse session
{"tool": "remote_shell", "arguments": {"target": "3", "session_id": "abc-123", "command": "df -h"}}

// Close session
{"tool": "remote_shell", "arguments": {"target": "3", "session_id": "abc-123", "command": "exit"}}
```

---

#### `exec_command`
One-shot SSH command. Use only when no follow-up commands are expected.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `server_id` | string/number | yes | |
| `commands` | string[] | yes | |
| `expect` | object[] | no | PTY mode: `[{"pattern": "...", "send": "..."}]` |
| `timeout_sec` | number | no | default 30 |
| `agent_id` | string | no | logged |
| `intent` | string | no | logged |

---

#### `exec_on_cluster`
Run commands on all cluster members in parallel.

| Parameter | Type | Required |
|-----------|------|----------|
| `cluster_id` | string/number | yes |
| `commands` | string[] | yes |
| `timeout_sec` | number | no |
| `agent_id` | string | no |
| `intent` | string | no |

---

#### `run_script`
Upload a script string to the remote host, execute it, then delete the temp file.
Preferred over `push_file` + `remote_shell` for multi-line scripts.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `server_id` | string/number | yes | |
| `content` | string | yes | Script source |
| `interpreter` | string | no | default `bash` |
| `login_shell` | boolean | no | default false |
| `timeout_sec` | number | no | default 120 |
| `agent_id` | string | no | logged |
| `intent` | string | no | logged |

---

#### `remote_replace`
Safe in-place string substitution in a remote file.
Prefer over `cat` + `run_script` when editing config files.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `server_id` | string/number | yes | |
| `path` | string | yes | Absolute path |
| `old_string` | string | yes | Exact literal (case-sensitive) |
| `new_string` | string | yes | Replacement (empty = delete) |
| `all` | boolean | no | Replace all occurrences (default true) |
| `agent_id` | string | no | logged |
| `intent` | string | no | logged |

Returns: `{"replaced": N}` — `0` means not found, file unchanged.

---

### Tunnel lifecycle tools

#### `start_tunnel` / `stop_tunnel`
Start or stop a saved tunnel (tmux-backed).

| Parameter | Type | Required |
|-----------|------|----------|
| `id` | string/number | yes |

Always call `list_tunnels` first to avoid starting duplicates.

---

## CLI Fallback Reference

Use the following when MCP tools are not available.

### Server registry

```bash
alogin server list --format json
alogin server show <host> --format json
alogin server add --host 10.0.0.10 --user admin
alogin server add --host bastion --user ops --auth-method key --identity-file ~/.ssh/id_ed25519
alogin server alias add prod admin@prod-db
alogin server alias list --format json
```

### SSH sessions (stateful — preferred over one-shot)

```bash
# Start session (prints session ID)
id=$(alogin ssh session start web-01)

# Commands share state — cd, exports persist
alogin ssh session exec "$id" "cd /var/log && ls"
alogin ssh session exec --timeout 300 "$id" "make build"
alogin ssh session exec --force-utf8 "$id" "cat /etc/os-release"

# List and reuse
alogin ssh session list
alogin ssh session exec "$existing_id" "uptime"

# Stop when done
alogin ssh session stop "$id"
```

`--timeout N` = wall-clock limit (default 30 s).
`--idle-timeout N` = cut if silent for N seconds.
`--force-utf8` = fix garbled multi-byte output (Korean, Japanese, etc.).

### One-shot command (stateless)

```bash
alogin ssh connect web-01 --cmd "df -h"
alogin ssh connect user@host --cmd "hostname" --force-utf8
```

Use only for truly independent, stateless single commands.

### Background jobs

```bash
id=$(alogin ssh session start web-01)
job=$(alogin ssh session bg-exec "$id" "apt-get upgrade -y")
alogin ssh session job status "$job"
alogin ssh session job logs --follow "$job"
alogin ssh session job list --session "$id" --json
alogin ssh session job cancel "$job"
alogin ssh session job purge
```

### Cluster execution

```bash
alogin ssh cluster add web-cluster 10.0.1.1 10.0.1.2
alogin ssh cluster list --format json
alogin ssh cluster web-cluster --cmd "uptime"
alogin ssh cluster web-cluster --cmd "df -h" --format json
```

### File transfer (SCP)

```bash
alogin scp push ./deploy.tar.gz web-01:/opt/releases/
alogin scp pull web-01:/var/log/app.log ./
alogin scp push --recursive ./dist/ web-01:/var/www/app/
alogin scp pull -r web-01:/var/log/nginx/ ./logs/
```

### Run local script / inline script

```bash
# Local file
alogin ssh run-local ./deploy.sh --remote web-01
alogin ssh run-local ./check.py --remote web-01 --interpreter python3
alogin ssh run-local ./setup.sh --remote web-01 -- --env prod

# Inline / piped content
echo "df -h && uptime" | alogin ssh run-script --remote web-01
alogin ssh run-script --remote web-01 --content "#!/bin/bash\ndf -h\nuptime"
```

### Gateways, profiles, tunnels

```bash
alogin net gateway add secure-zone bastion.ext.com internal-jump
alogin net gateway list --format json
alogin server add --host prod-sql --gateway secure-zone
alogin net profile add office --gateway secure-zone
alogin net profile use office
alogin net profile use none

alogin net tunnel add db-proxy --server prod-db --local-port 5432 --remote-port 5432
alogin net tunnel start db-proxy
alogin net tunnel status db-proxy
alogin net tunnel stop db-proxy
```

### Server memory (CLI)

```bash
alogin agent server-memory list <server-id>
alogin agent server-memory add  <server-id> --text "nginx config: /etc/nginx/nginx.conf"
alogin agent server-memory del  <server-id> <note-id>
```

### Read server prompt before acting

```bash
alogin server show <id> --format json   # includes system_prompt field
```

The `system_prompt` field contains server-specific restrictions.
**Read it before connecting and adhere to it throughout the session.**

---

## Safety Rules

- Always provide `intent` on execution tool calls.
- Read-only inspection before any write operation.
- Do not run destructive commands (`rm -rf`, `shutdown`, `DROP TABLE`) without
  explicit user confirmation — HITL policy will block or queue them.
- If a server `device_type` is `router`, `switch`, or `firewall`, do not assume
  standard Linux commands work.
- Check `list_tunnels` before starting a tunnel to avoid duplicates.
- For remote file edits, prefer `remote_replace` over full-file rewrites.

---

## Agent Memory Patterns

Three patterns to apply every session:

1. **Dynamic profiling** — After exploring a new server, call `save_memory` with
   log paths, installed tools, service layout. Next session starts with full
   context, zero exploration turns.

2. **State drift detection** — Save baselines (disk %, process counts, config
   checksums). On subsequent sessions, compare and flag anomalies proactively.

3. **Multi-agent handoff** — Notes written by one agent are immediately available
   to the next. No duplicate work; no knowledge lost between agents.

---

## JSON Output (CLI)

```bash
alogin server list --format json | jq '.[].host'
alogin ssh cluster prod --cmd "uptime" --format json | jq '.[] | {host, output}'
alogin net tunnel list --format json
alogin ssh session job list --json
```
