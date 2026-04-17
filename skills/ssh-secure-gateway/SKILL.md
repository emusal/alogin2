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

Add `--force-utf8` to any `exec` or `connect --cmd` call when the server's default locale may produce garbled multi-byte (Korean, Japanese, Chinese, etc.) output:

```bash
alogin ssh session exec --force-utf8 "$id" "cat /etc/os-release"
alogin ssh connect web-01 --cmd "hostname" --force-utf8
```

**When to use one-shot `--cmd` instead:** only when you need a single, truly stateless command and no follow-up commands are expected (e.g., a quick health check in a CI script).

#### Timeout behaviour

`exec` uses a **wall-clock timeout**: `--timeout N` is the maximum total seconds the command is allowed to run. The command is interrupted after N seconds regardless of output activity.

- `--timeout N` (default **30 s**) = wall-clock limit — the command is killed after this duration (exit 130).
- `--idle-timeout N` (default **disabled**) = additionally cut the connection if no output is received for N consecutive seconds. Use this for commands where silence itself indicates a hang.

```bash
# Allow up to 5 min total
alogin ssh session exec --timeout 300 "$id" "make build"

# Kill if silent for 60 s (e.g. interactive prompt waiting for input)
alogin ssh session exec --timeout 300 --idle-timeout 60 "$id" "some-interactive-cmd"
```

If a command times out, the session remains usable — cwd and env are preserved.

> **When NOT to use `exec`:** commands that run indefinitely or longer than any reasonable timeout — streaming log tails (`tail -f`), long-running daemons, browser automation with unpredictable duration. Use `bg-exec` for these cases instead.

#### Login Shell

By default, the session starts `bash -l` (login shell), which sources `~/.bash_profile` and `~/.profile`.
`PATH` extensions from `nvm`, `pyenv`, `rbenv`, conda, etc. are loaded automatically — no extra flags needed.

If a pristine environment is required (e.g. CI scripts that must not be affected by user profile customisations),
start the session with `--no-login-shell` to use `bash --noprofile --norc` instead.

### [Background Execution (bg-exec)](https://github.com/emusal/alogin2#ssh-session)

For long-running commands or commands that produce little/no stdout for extended periods — package installs, data migrations, large backups, browser automation (Playwright, Puppeteer), cold-start runtimes — use `bg-exec` to fire-and-poll instead of blocking. These commands will hit `exec`'s idle timeout regardless of the `--timeout` value.

`bg-exec` spawns a detached tmux session that outlives the CLI process — the job keeps running even after the terminal closes.

For MCP-based background execution use the `bg_exec_command` tool (runs as a goroutine inside the MCP server process, no tmux needed), then poll with `job_status` / `job_logs`.

```bash
id=$(alogin ssh session start web-01)

# Fire-and-forget: prints a job ID immediately, execution continues in the background
job=$(alogin ssh session bg-exec "$id" "apt-get upgrade -y")
job=$(alogin ssh session bg-exec "$id" "backup.sh" --timeout 7200)

# Poll status (job-id can be the full UUID or a unique prefix — 8 chars is enough)
alogin ssh session job status "$job"          # pending | running | done | failed
alogin ssh session job status "$job" --json   # machine-readable

# Fetch captured output (available while running and after completion)
alogin ssh session job logs "$job"

# List all jobs (optionally filter by session)
alogin ssh session job list
alogin ssh session job list --session "$id"
alogin ssh session job list --json

# Cancel a pending or running job
alogin ssh session job cancel "$job"

# Delete a single job record (any status, by prefix or full UUID)
alogin ssh session job delete "$job"

# Bulk-delete finished jobs (done / failed / cancelled)
alogin ssh session job purge

# Delete every job record including pending and running
alogin ssh session job purge --all
```

Job statuses: `pending` → `running` → `done` | `failed` | `cancelled`

**When to use bg-exec vs exec:**

- `exec`: short commands where you need the output immediately (≤ timeout, default 30 s)
- `bg-exec`: anything that may exceed the timeout, or when you want to launch multiple jobs in parallel and poll later

#### `--follow` / `-f`: Stream output until done

Instead of polling `job status` in a loop, pass `--follow` to `job logs` to stream output incrementally until the job finishes:

```bash
job=$(alogin ssh session bg-exec "$id" "apt-get upgrade -y")

# Block here, printing output as it arrives; exits when job completes
alogin ssh session job logs --follow "$job"
# or: alogin ssh session job logs -f "$job"
```

The command prints each new chunk as it is written to the DB, then exits with a summary line (`[job <id>: done, exit 0]`) when the job finishes. Ctrl-C interrupts the follow without cancelling the job itself.

### [SCP (File Transfer)](https://github.com/emusal/alogin2#scp)

Copy files between local and remote hosts via SFTP. Credentials and multi-hop routing follow the same profile/gateway chain as `alogin ssh connect`.

```bash
# Upload local file to remote
alogin scp push ./deploy.tar.gz web-01:/opt/releases/
alogin scp push ./script.py admin@web-01:/tmp/

# Download remote file to local
alogin scp pull web-01:/var/log/app.log ./
alogin scp pull admin@web-01:/etc/nginx/nginx.conf ./nginx.conf.bak

# Recursive directory transfer (use -r / --recursive)
alogin scp push --recursive ./dist/ web-01:/var/www/app/
alogin scp pull -r web-01:/var/log/nginx/ ./logs/
```

If the destination path ends with `/`, the source filename (or directory name) is appended automatically.

For MCP-based file transfer use the `push_file` / `pull_file` tools with `recursive=true` for directory trees.

### [Run Local Script (run-local)](https://github.com/emusal/alogin2#ssh--remote-connectivity)

Upload a local script **file**, execute it on the remote host, and delete the temp file — all in one command.
Eliminates the manual `scp push → session exec → rm` workflow.

```bash
# Basic usage — interpreter auto-detected from shebang or extension
alogin ssh run-local ./deploy.sh --remote web-01
alogin ssh run-local ./check.py --remote admin@web-01

# Explicit interpreter
alogin ssh run-local ./migrate.py --remote web-01 --interpreter python3

# Pass script arguments (use -- to separate from alogin flags)
alogin ssh run-local ./setup.sh --remote web-01 -- --env prod --version 1.2

# Pristine environment (skip ~/.bash_profile)
alogin ssh run-local ./build.sh --remote web-01 --login-shell=false

# Force UTF-8 output + long timeout
alogin ssh run-local ./report.py --remote web-01 --force-utf8 --timeout 300

# Keep the uploaded file on the remote host after execution
alogin ssh run-local ./debug.sh --remote web-01 --keep
```

Interpreter auto-detection order:
1. Shebang line (`#!/usr/bin/env python3`, `#!/bin/bash`, …)
2. File extension (`.py` → `python3`, `.rb` → `ruby`, `.js` → `node`, `.pl` → `perl`)
3. Default: `bash`

### [Run Script (run-script)](https://github.com/emusal/alogin2#ssh--remote-connectivity)

Upload a script given as a **string** (via `--content` or stdin), execute it on the remote host, and delete the temp file.
Unlike `run-local` (which takes a local file path), `run-script` is ideal for dynamically generated scripts or piped input.
This is the CLI equivalent of the MCP `run_script` tool.

```bash
# Pipe script content from stdin
echo "df -h && uptime" | alogin ssh run-script --remote web-01

# Pass content directly via --content (use \n for newlines)
alogin ssh run-script --remote web-01 --content "#!/bin/bash\ndf -h\nuptime"

# Explicit interpreter
alogin ssh run-script --remote web-01 --interpreter python3 --content "import sys; print(sys.version)"

# Pristine environment (skip ~/.bash_profile)
alogin ssh run-script --remote web-01 --login-shell=false --content "node --version"

# Force UTF-8 output + long timeout
alogin ssh run-script --remote web-01 --force-utf8 --timeout 300 --content "cat /etc/os-release"
```

**When to use `run-script` vs `run-local`:**

| | `run-local` | `run-script` |
|---|---|---|
| Input | Local file path | String (`--content`) or stdin |
| Use case | Pre-existing scripts | Dynamically generated content, pipes |
| MCP equivalent | — | `run_script` tool |

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

# Trust window — auto-approve HITL for a period of time (no token needed per-request)
alogin agent trust --duration 1h                          # global: all agents & servers
alogin agent trust --duration 30m --agent claude-dev      # only requests from claude-dev
alogin agent trust --duration 2h --server 3               # only requests targeting server 3
alogin agent trust-list                                    # show active trust windows
alogin agent untrust                                       # revoke global window immediately
alogin agent untrust --agent claude-dev                    # revoke agent-scoped window

# Policy dry-run — check if a command would be allowed before running it
alogin agent policy dry-run --cmd "rm -rf /"
alogin agent policy dry-run --cmd "df -h" --cmd "uptime"
alogin agent policy dry-run --cmd "shutdown now" --agent claude-dev --server 3
alogin agent policy dry-run --cmd "rm -rf /" --json   # machine-readable output

# Per-server policy and system prompt overrides
alogin agent server-policy set <id> --file policy.yaml
alogin agent server-policy show <id>
alogin agent server-prompt set <id> --text "Only run read-only commands."

# Per-server agent memory notes (append-only, timestamped)
alogin agent server-memory add  <id> --text "nginx config is at /etc/nginx/nginx.conf"
alogin agent server-memory list <id>
alogin agent server-memory del  <id> <note-id>

> **IMPORTANT: Server Prompts**
> The `server-prompt` contains critical, server-specific operational instructions and restrictions.
> **Before connecting to any server to perform tasks, you MUST read its `server-prompt`** (via `alogin server show <id>`) and strictly adhere to those instructions during your session.

> **Agent Memory — Persistent Knowledge Per Server**
>
> Agent memory is a shared, append-only log that any agent can write to and read from across sessions.
> Use it to eliminate redundant exploration and enable multi-agent collaboration.
>
> **Session start — always recall first:**
> ```
> get_memory(server_id=<id>)   # MCP
> alogin agent server-memory list <id>   # CLI
> ```
>
> **Record findings during a session:**
> ```
> save_memory(server_id=<id>, content="log path: /var/log/messages, not /var/log/syslog")
> save_memory(server_id=<id>, content="baseline disk usage ~40%; alert if >70%")
> save_memory(server_id=<id>, content="python3.11 at /usr/local/bin/python3, venv in ~/app/.venv")
> ```
>
> **Three patterns to apply:**
>
> 1. **Dynamic profiling** — After exploring a server (log paths, installed tools, service layout), save the findings. Next session starts with full context, zero exploration turns.
>
> 2. **State drift detection** — Save baselines (disk %, process counts, config checksums). On subsequent sessions, compare current state against the recorded baseline and flag anomalies proactively.
>
> 3. **Multi-agent handoff** — Notes written by one agent (e.g. a discovery agent) are immediately available to the next (e.g. a security audit agent). No duplicate work; no knowledge lost between agents.
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

| Command                          | `--format json` output                                   |
| -------------------------------- | -------------------------------------------------------- |
| `server list`                    | array of server objects                                  |
| `server show <host>`             | single server object (incl. policy_yaml, system_prompt)  |
| `server alias list`              | array of alias objects                                   |
| `server alias show <name>`       | single alias object                                      |
| `net gateway list`               | array of gateway objects                                 |
| `net gateway show <name>`        | gateway object with hops array                           |
| `net tunnel list`                | array of tunnel objects with running status              |
| `net hosts list`                 | array of host mapping objects                            |
| `net hosts show <hostname>`      | single host mapping object                               |
| `net profile list`               | array of profile objects                                 |
| `ssh cluster list`               | array of cluster objects                                 |
| `ssh cluster <name> --cmd <cmd>` | array of `{host, output, exit_code, error}`              |
| `agent audit list`               | array of audit entry objects                             |
| `agent audit tail`               | newline-delimited JSON stream                            |
| `agent policy dry-run --json`    | `{action, commands, agent_id, server_id, policy, rule?}` |
| `app list`                       | array of app binding objects                             |
| `app show <name>`                | single binding object                                    |
| `ssh session job list --json`    | array of bg_job objects                                  |
| `ssh session job status --json`  | single bg_job object                                     |

```bash
# Examples
alogin server list --format json | jq '.[].host'
alogin ssh cluster prod --cmd "uptime" --format json | jq '.[] | {host, output}'
alogin agent audit list --since 24h --format json | jq '.[] | select(.policy_action == "require_approval")'
```
