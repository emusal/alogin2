---
name: ssh-secure-gateway
description: Securely access SSH servers, run remote commands, and manage clusters via alogin. Use this skill to query server infrastructure, inspect node health, and execute remote commands safely without handling SSH keys or ProxyJumps manually.
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
| `app list` | array of app binding objects |
| `app show <name>` | single binding object |

```bash
# Examples
alogin server list --format json | jq '.[].host'
alogin ssh cluster prod --cmd "uptime" --format json | jq '.[] | {host, output}'
alogin agent audit list --since 24h --format json | jq '.[] | select(.policy_action == "require_approval")'
```
