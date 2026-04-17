---
name: ssh-secure-gateway
description: Securely access SSH servers, run remote commands, and manage clusters via alogin. ALWAYS prefer SSH sessions (alogin ssh session start/exec) over one-shot --cmd invocations — sessions preserve working directory, environment variables, and shell state across commands. Use this skill to query server infrastructure, inspect node health, and execute remote commands safely without handling SSH keys or ProxyJumps manually.
license: Apache-2.0
metadata:
  {
    'openclaw':
      {
        'requires': { 'bins': ['alogin'] },
        'homepage': 'https://github.com/emusal/alogin2',
      },
  }
---

# alogin-based SSH Secure Gateway

The secure gateway for Agentic AI and System Administrators.

Use `alogin --help` and `<command> --help` for flags, arguments, and full examples.
This skill focuses on concepts and canonical workflows.

## Quick Start

```bash
# 1. Install
curl -fsSL https://raw.githubusercontent.com/emusal/alogin2/main/install.sh | sh

# 2. Add a server to the encrypted registry
alogin server add --host 10.0.0.10 --user admin

# 3. Connect instantly
alogin ssh connect 10.0.0.10

# 4. Run a command and exit
alogin ssh connect 10.0.0.10 --cmd "df -h"

# 5. List all servers in JSON for parsing
alogin server list --format json
```

## Core Concepts

### [Server (Server Registry)](https://github.com/emusal/alogin2#server--server-registry)

The registry stores server metadata and credentials in an encrypted vault (macOS Keychain, Linux Secret Service, or `age`).
Canonical flow:

```bash
alogin server list                                           # table (default)
alogin server list --format json                             # machine-readable
alogin server add --host prod-db --user dbadmin --note "Primary DB"
alogin server show prod-db                                   # human-readable detail
alogin server show prod-db --format json                     # full detail as JSON
alogin server passwd prod-db                                 # update vault password

# Server aliases
alogin server alias add prod admin@prod-db
alogin server alias list --format json
```

### [SSH (Remote Connectivity)](https://github.com/emusal/alogin2#ssh--remote-connectivity)

SSH handles connections, SFTP, and cluster sessions. It automatically injects credentials and handles multi-hop ProxyJumps.
Canonical flows:

```bash
# Simple SSH
alogin ssh connect user@host
alogin ssh connect user@host --cmd "df -h"     # run command, no interactive shell

# Parallel Cluster execution — results from all members in parallel
alogin ssh cluster add web-cluster 10.0.1.1 10.0.1.2
alogin ssh cluster web-cluster --cmd "uptime"              # table output
alogin ssh cluster web-cluster --cmd "df -h" --format json # JSON array

# List clusters
alogin ssh cluster list --format json

# Mounting remote FS
alogin ssh mount user@host:/var/log ~/mnt/logs
```

### [SSH Session (Stateful Shell)](https://github.com/emusal/alogin2#ssh-session) ⭐ Preferred

> **Always prefer sessions over one-shot `--cmd`.** A session keeps working directory, environment variables, and shell state intact across commands. One-shot `--cmd` starts a fresh process every time and loses all state — use it only for truly independent, stateless single commands.

A session holds a single persistent bash process on the remote server. Sessions are backed by a tmux session and persist across separate `alogin` invocations. For MCP-based persistence use the `remote_shell` tool.

```bash
# Start a session (prints session ID) — do this first before running any commands
id=$(alogin ssh session start web-01)

# Commands share state — cd persists to the next call
alogin ssh session exec "$id" "cd /var/log"
alogin ssh session exec "$id" "pwd"             # outputs /var/log
alogin ssh session exec "$id" "export FOO=bar"
alogin ssh session exec "$id" "echo \$FOO"      # outputs bar

# Re-use an existing session instead of starting a new one
alogin ssh session list                         # find an existing session ID
alogin ssh session exec "$existing_id" "uptime"

# Terminate when done
alogin ssh session stop "$id"
```

Use `--timeout N` (seconds, default 30) on `exec` for long-running commands.

**When to use one-shot `--cmd` instead:** only when you need a single, truly stateless command and no follow-up commands are expected (e.g., a quick health check in a CI script).

### [SCP (File Transfer)](https://github.com/emusal/alogin2#scp)

Copy files between local and remote hosts via SFTP. Credentials and multi-hop routing follow the same profile/gateway chain as `alogin ssh connect`.

```bash
# Upload local file to remote
alogin scp push ./deploy.tar.gz web-01:/opt/releases/
alogin scp push ./script.py admin@web-01:/tmp/

# Download remote file to local
alogin scp pull web-01:/var/log/app.log ./
alogin scp pull admin@web-01:/etc/nginx/nginx.conf ./nginx.conf.bak
```

If the destination path ends with `/`, the source filename is appended automatically.

### [Net (Gateway, Profile, Tunnels & DNS)](https://github.com/emusal/alogin2#multi-hop-gateway-routing)

Define multi-hop jump paths once, then use them for any server. Manage persistent tunnels and local DNS overrides.
Mental model:

- A **gateway** is a sequence of hops.
- A **profile** activates a gateway globally for all connections.

Canonical flow:

```bash
# 1. Register hops
alogin server add --host bastion.ext.com
alogin server add --host internal-jump

# 2. Define a named gateway route
alogin net gateway add secure-zone bastion.ext.com internal-jump
alogin net gateway list --format json
alogin net gateway show secure-zone --format json

# 3. Route a target server via the gateway
alogin server add --host prod-sql --gateway secure-zone
alogin ssh connect prod-sql

# 4. Or activate a profile to route all connections automatically
alogin net profile add office --gateway secure-zone --desc "Office network"
alogin net profile use office
alogin net profile use none    # disable gateway routing

# Persistent tunnels
alogin net tunnel add db-proxy --server prod-db --local-port 5432 --remote-port 5432
alogin net tunnel list --format json
alogin net tunnel start db-proxy
alogin net tunnel status db-proxy
alogin net tunnel stop db-proxy

# Local DNS overrides
alogin net hosts list --format json
alogin net hosts show prod-db --format json
```

### [Vault (Stored Credentials)](https://github.com/emusal/alogin2#vault)

```bash
alogin vault set testuser@prod-db
alogin vault get testuser@prod-db
alogin vault delete testuser@prod-db
```

### [Agent (MCP & AI Safety)](https://github.com/emusal/alogin2#ai-agent-integration-mcp)

Commands for configuring alogin as an MCP (Model Context Protocol) server for LLMs like Claude or ChatGPT.
Includes human-in-the-loop approval, policy-based RBAC, and a full audit trail.
Canonical flow:

```bash
# Setup MCP config for Claude Desktop
alogin agent setup

# Start the MCP server (called by the AI client)
alogin agent mcp

# Audit tool calls
alogin agent audit list --since 1h
alogin agent audit list --since 1h --format json
alogin agent audit tail --format json    # stream new events

# Human approval workflow
alogin agent pending                     # list pending approvals
alogin agent approve <token>
alogin agent deny <token>

# Policy dry-run — check if a command would be allowed before running it
alogin agent policy dry-run --cmd "rm -rf /"
alogin agent policy dry-run --cmd "df -h" --cmd "uptime"
alogin agent policy dry-run --cmd "shutdown now" --agent claude-dev --server 3
alogin agent policy dry-run --cmd "rm -rf /" --json   # machine-readable output

# Per-server policy and system prompt overrides
alogin agent server-policy set <id> --file policy.yaml
alogin agent server-policy show <id>
alogin agent server-prompt set <id> --text "Only run read-only commands."
```

### [App (Named Application Bindings)](https://github.com/emusal/alogin2#app--named-application-bindings)

App bindings pair a server with an application plugin (DB client, container shell, etc.)
so a single name launches the correct app with automatic credential injection.

Canonical flow:

```bash
# 1. Add a binding
alogin app add --name prod-mysql --server prod-db --app mariadb

# 2. List all bindings
alogin app list
alogin app list --format json

# 3. Connect (launches plugin with automatic credential injection)
alogin app connect prod-mysql

# 4. Non-interactive command via the plugin
alogin app connect prod-mysql --cmd "SHOW DATABASES"

# 5. Show or delete
alogin app show prod-mysql
alogin app delete prod-mysql

# 6. List installed plugin definitions
alogin app plugin list
alogin app plugin list --format json
```

Plugin YAML files live in `~/.config/alogin/plugins/<name>.yaml`.
Credentials are resolved from vault and injected via PTY automation (expect/send) — never exposed in command arguments or logs.

### JSON Output

All list and show commands support `--format json` for machine-readable output:

| Command | `--format json` output |
|---------|----------------------|
| `server list` | array of server objects |
| `server show <host>` | single server object (incl. policy_yaml, system_prompt) |
| `server alias list` | array of alias objects |
| `server alias show <name>` | single alias object |
| `net gateway list` | array of gateway objects |
| `net gateway show <name>` | gateway object with hops array |
| `net tunnel list` | array of tunnel objects with running status |
| `net hosts list` | array of host mapping objects |
| `net hosts show <hostname>` | single host mapping object |
| `net profile list` | array of profile objects |
| `ssh cluster list` | array of cluster objects |
| `ssh cluster <name> --cmd <cmd>` | array of `{host, output, exit_code, error}` |
| `agent audit list` | array of audit entry objects |
| `agent audit tail` | newline-delimited JSON stream |
| `agent policy dry-run --json` | `{action, commands, agent_id, server_id, policy, rule?}` |
| `app list` | array of app binding objects |
| `app show <name>` | single binding object |

```bash
# Examples
alogin server list --format json | jq '.[].host'
alogin ssh cluster prod --cmd "uptime" --format json | jq '.[] | {host, output}'
alogin agent audit list --since 24h --format json | jq '.[] | select(.policy_action == "require_approval")'
```
