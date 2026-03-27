---
name: alogin
description: Securely access SSH servers, run remote commands, and manage clusters via alogin. Use this skill to query server infrastructure, inspect node health, and execute remote commands safely without handling SSH keys or ProxyJumps manually.
license: Apache-2.0
metadata: { "openclaw": { "requires": { "bins": ["alogin"] }, "homepage": "https://github.com/emusal/alogin2" } }
---

# alogin

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
alogin compute list
alogin compute add --host prod-db --user dbadmin --note "Primary DB"
alogin compute show prod-db
alogin compute passwd prod-db    # Update vault password
```

### [Access (Remote Connectivity)](https://github.com/emusal/alogin2#access--remote-connectivity)

Access handles SSH, SFTP, and Cluster sessions. It automatically injects credentials and handles multi-hop ProxyJumps.
Canonical flows:

```bash
# Simple SSH
alogin access ssh user@host

# Parallel Cluster execution
alogin access cluster add web-cluster 10.0.1.1 10.0.1.2
alogin access cluster web-cluster --cmd "uptime"

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
alogin net tunnel start db-proxy
alogin net tunnel status db-proxy
alogin net tunnel stop db-proxy
```

### [Agent (MCP & AI)](https://github.com/emusal/alogin2#ai-agent-integration-mcp)

Commands for configuring alogin as an MCP (Model Context Protocol) server for LLMs like Claude or ChatGPT.
Canonical flow:

```bash
# Setup MCP config for Claude Desktop
alogin agent setup

# Start the MCP server (called by the AI client)
alogin agent mcp

# Audit tool calls
alogin agent audit list --since 1h
```

### Piped Output

When output is piped or `--format json` is used, `alogin` emits machine-readable data:

```bash
alogin compute list --format json | jq '.[].host'
```
