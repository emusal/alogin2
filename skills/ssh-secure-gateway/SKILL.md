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
alogin compute add --host 10.0.0.10 --user admin

# 3. Connect instantly
alogin access ssh 10.0.0.10

# 4. Run a command and exit
alogin access ssh 10.0.0.10 --cmd "df -h"

# 5. List all servers in JSON for parsing
alogin compute list --format json
```

## Core Concepts

### [Compute (Server Registry)](https://github.com/emusal/alogin2#compute--server-registry)

The registry stores server metadata and credentials in an encrypted vault (macOS Keychain, Linux Secret Service, or `age`).
Canonical flow:

```bash
alogin compute list                                          # table (default)
alogin compute list --format json                            # machine-readable
alogin compute add --host prod-db --user dbadmin --note "Primary DB"
alogin compute show prod-db                                  # human-readable detail
alogin compute show prod-db --format json                    # full detail as JSON
alogin compute passwd prod-db                                # update vault password
```

### [Access (Remote Connectivity)](https://github.com/emusal/alogin2#access--remote-connectivity)

Access handles SSH, SFTP, and Cluster sessions. It automatically injects credentials and handles multi-hop ProxyJumps.
Canonical flows:

```bash
# Simple SSH
alogin access ssh user@host
alogin access ssh user@host --cmd "df -h"     # run command, no interactive shell

# Parallel Cluster execution — results from all members in parallel
alogin access cluster add web-cluster 10.0.1.1 10.0.1.2
alogin access cluster web-cluster --cmd "uptime"              # table output
alogin access cluster web-cluster --cmd "df -h" --format json # JSON array

# List clusters
alogin access cluster list --format json

# Mounting remote FS
alogin access mount user@host:/var/log ~/mnt/logs
```

### [Auth (Gateway & Routing)](https://github.com/emusal/alogin2#multi-hop-gateway-routing)

Define multi-hop jump paths once, then use them for any server.
Mental model:

- A gateway is a sequence of hops.
- A server is assigned a gateway for automatic routing.

Canonical flow:

```bash
# 1. Register hops
alogin compute add --host bastion.ext.com
alogin compute add --host internal-jump --gateway bastion.ext.com

# 2. Define a named gateway route
alogin auth gateway add secure-zone bastion.ext.com internal-jump
alogin auth gateway list --format json
alogin auth gateway show secure-zone --format json

# 3. Route a target server via the gateway
alogin compute add --host prod-sql --gateway secure-zone
alogin access ssh prod-sql --auto-gw
```

### [Net (Tunnels & DNS)](https://github.com/emusal/alogin2#connection--tunnels)

Manage persistent SSH port-forwards in detached `tmux` sessions and local DNS overrides.
Canonical flow:

```bash
# Register a persistent tunnel
alogin net tunnel add db-proxy --server prod-db --local-port 5432 --remote-port 5432

# Lifecycle management
alogin net tunnel list --format json
alogin net tunnel start db-proxy
alogin net tunnel status db-proxy
alogin net tunnel stop db-proxy

# Local DNS overrides
alogin net hosts list --format json
alogin net hosts show prod-db --format json
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

### [App-Server (Named Application Bindings)](https://github.com/emusal/alogin2#app-server--named-application-bindings)

App-server bindings pair a compute server with an application plugin (DB client, container shell, etc.)
so a single name launches the correct app with automatic credential injection.

Canonical flow:

```bash
# 1. Add a binding
alogin app-server add --name prod-mysql --server prod-db --app mariadb --auto-gw

# 2. List all bindings
alogin app-server list
alogin app-server list --format json

# 3. Connect (launches plugin with automatic credential injection)
alogin app-server connect prod-mysql

# 4. Non-interactive command via the plugin
alogin app-server connect prod-mysql --cmd "SHOW DATABASES"

# 5. Show or delete
alogin app-server show prod-mysql
alogin app-server delete prod-mysql

# 6. List installed plugin definitions
alogin app-server plugin list
alogin app-server plugin list --format json
```

Plugin YAML files live in `~/.config/alogin/plugins/<name>.yaml`.
Credentials are resolved from vault and injected via PTY automation (expect/send) — never exposed in command arguments or logs.

### JSON Output

All list and show commands support `--format json` for machine-readable output:

| Command | `--format json` output |
|---------|----------------------|
| `compute list` | array of server objects |
| `compute show <host>` | single server object (incl. policy_yaml, system_prompt) |
| `auth gateway list` | array of gateway objects |
| `auth gateway show <name>` | gateway object with hops array |
| `auth alias list` | array of alias objects |
| `auth alias show <name>` | single alias object |
| `net tunnel list` | array of tunnel objects with running status |
| `net hosts list` | array of host mapping objects |
| `net hosts show <hostname>` | single host mapping object |
| `access cluster list` | array of cluster objects |
| `access cluster <name> --cmd <cmd>` | array of `{host, output, exit_code, error}` |
| `agent audit list` | array of audit entry objects |
| `agent audit tail` | newline-delimited JSON stream |
| `app-server list` | array of app-server binding objects |
| `app-server show <name>` | single binding object |

```bash
# Examples
alogin compute list --format json | jq '.[].host'
alogin access cluster prod --cmd "uptime" --format json | jq '.[] | {host, output}'
alogin agent audit list --since 24h --format json | jq '.[] | select(.policy_action == "require_approval")'
```
