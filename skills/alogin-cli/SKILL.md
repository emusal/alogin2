---
name: alogin-cli
description: >
  alogin CLI-only remote server operations for environments where MCP tools
  are not available (CI pipelines, terminal agents, raw shell contexts).
  Covers persistent sessions, cluster execution, file transfer, background
  jobs, gateways, tunnels, and app bindings — all via the alogin CLI.
  Does NOT include admin operations (approve/deny, vault read, policy set).
license: Apache-2.0
metadata:
  openclaw:
    requires:
      bins: [alogin]
    homepage: https://github.com/emusal/alogin2
---

# alogin-cli — CLI-Only Agent Skill

Use this skill in environments where MCP tools are not available: CI pipelines,
raw terminal agents, shell scripts, or any context that invokes the `alogin`
binary directly.

If MCP tools are available, use the `alogin` skill instead — it prefers
`remote_shell`, `exec_command`, and other MCP tools that carry richer audit
metadata and handle reconnection automatically.

Common triggers: SSH, remote shell, server login, bastion, health check,
run command, cluster, SFTP, background job, tunnel, `alogin`.

---

## Standard Workflow

```
1. DISCOVER  — alogin server list --format json
2. RECALL    — alogin agent server-memory list <id>
3. SHELL     — alogin ssh session start → reuse session ID for every command
4. INSPECT   — alogin ssh connect <host> --cmd "..." for read-only pre-check
5. ACT       — alogin ssh session exec for sequences
6. VERIFY    — read-only command after change
7. SAVE      — alogin agent server-memory add <id> --text "..."
8. CLEANUP   — alogin ssh session stop <session-id>
```

Always prefer **sessions** over one-shot `--cmd`. Sessions keep working
directory, environment variables, and shell state across commands.

---

## Server Registry

```bash
alogin server list                                      # table
alogin server list --format json                        # machine-readable

alogin server show <host>                               # human-readable
alogin server show <host> --format json                 # includes system_prompt

alogin server add --host 10.0.0.10 --user admin
alogin server add --host bastion --user ops \
  --auth-method key --identity-file ~/.ssh/id_ed25519

# Aliases
alogin server alias add prod admin@prod-db
alogin server alias list --format json
```

`auth_method`:
- `password` (default) — uses vault password, falls back to SSH agent / default keys
- `key` — SSH public key only; never sends vault password

**Read the server's `system_prompt` before acting:**

```bash
alogin server show <id> --format json | jq '.system_prompt'
```

The `system_prompt` contains server-specific operational restrictions.
Adhere to them throughout the session.

---

## SSH Sessions (Stateful — Preferred)

A session holds a single persistent bash process on the remote server,
backed by a tmux session. Shell state (cwd, exports) carries across calls.

```bash
# Start a session — prints session ID
id=$(alogin ssh session start web-01)

# Commands share state
alogin ssh session exec "$id" "cd /var/log"
alogin ssh session exec "$id" "pwd"              # → /var/log
alogin ssh session exec "$id" "export FOO=bar"
alogin ssh session exec "$id" "echo \$FOO"       # → bar

# Reuse an existing session
alogin ssh session list
alogin ssh session exec "$existing_id" "uptime"

# Stop when done
alogin ssh session stop "$id"
```

### Timeout options

```bash
# Wall-clock limit (default 30 s)
alogin ssh session exec --timeout 300 "$id" "make build"

# Kill if silent for N seconds (detect hangs / interactive prompts)
alogin ssh session exec --timeout 300 --idle-timeout 60 "$id" "some-cmd"
```

If a command times out, the session remains usable — cwd and env are preserved.

### UTF-8 / multi-byte output

```bash
alogin ssh session exec --force-utf8 "$id" "cat /etc/os-release"
alogin ssh connect web-01 --cmd "hostname" --force-utf8
```

### Login shell

Sessions start `bash -l` by default (sources `~/.bash_profile`, loads
`nvm`, `pyenv`, `rbenv`, conda PATH extensions).

```bash
# Pristine environment — skip user profile
id=$(alogin ssh session start web-01 --no-login-shell)
```

---

## One-Shot Command (Stateless)

Use only for a single independent command where no follow-up is expected.

```bash
alogin ssh connect web-01 --cmd "df -h"
alogin ssh connect user@host --cmd "hostname" --force-utf8
```

---

## Background Jobs (bg-exec)

For long-running commands (package installs, migrations, backups, Playwright):
fire-and-poll instead of blocking `exec`.

```bash
id=$(alogin ssh session start web-01)

# Fire-and-forget — prints job ID immediately
job=$(alogin ssh session bg-exec "$id" "apt-get upgrade -y")
job=$(alogin ssh session bg-exec "$id" "backup.sh" --timeout 7200)

# Poll status
alogin ssh session job status "$job"           # pending | running | done | failed
alogin ssh session job status "$job" --json

# Stream output until done (exits with summary line)
alogin ssh session job logs --follow "$job"

# Fetch output at any time
alogin ssh session job logs "$job"

# Manage jobs
alogin ssh session job list
alogin ssh session job list --session "$id" --json
alogin ssh session job cancel "$job"
alogin ssh session job delete "$job"
alogin ssh session job purge            # delete done/failed/cancelled
alogin ssh session job purge --all      # delete everything including running
```

---

## Cluster Execution

```bash
# Setup
alogin ssh cluster add web-cluster 10.0.1.1 10.0.1.2
alogin ssh cluster list --format json

# Run on all members in parallel
alogin ssh cluster web-cluster --cmd "uptime"
alogin ssh cluster web-cluster --cmd "df -h" --format json
```

Returns: `[{host, output, exit_code, error}]`

---

## File Transfer (SCP)

```bash
# Upload
alogin scp push ./deploy.tar.gz web-01:/opt/releases/
alogin scp push ./script.py admin@web-01:/tmp/
alogin scp push --recursive ./dist/ web-01:/var/www/app/

# Download
alogin scp pull web-01:/var/log/app.log ./
alogin scp pull admin@web-01:/etc/nginx/nginx.conf ./nginx.conf.bak
alogin scp pull -r web-01:/var/log/nginx/ ./logs/
```

Destination path ending with `/` appends the source filename automatically.

---

## Run Scripts on Remote Host

### From a local file (`run-local`)

```bash
alogin ssh run-local ./deploy.sh --remote web-01
alogin ssh run-local ./check.py --remote web-01 --interpreter python3
alogin ssh run-local ./setup.sh --remote web-01 -- --env prod --version 1.2
alogin ssh run-local ./build.sh --remote web-01 --login-shell=false
alogin ssh run-local ./report.py --remote web-01 --force-utf8 --timeout 300
alogin ssh run-local ./debug.sh --remote web-01 --keep   # keep temp file
```

Interpreter auto-detection: shebang → extension (`.py`→`python3`, `.rb`→`ruby`,
`.js`→`node`, `.pl`→`perl`) → default `bash`.

### From string / stdin (`run-script`)

```bash
# Pipe from stdin
echo "df -h && uptime" | alogin ssh run-script --remote web-01

# Pass content directly
alogin ssh run-script --remote web-01 \
  --content "#!/bin/bash\ndf -h\nuptime"

# Explicit interpreter
alogin ssh run-script --remote web-01 \
  --interpreter python3 \
  --content "import sys; print(sys.version)"
```

---

## Gateways, Profiles, and Tunnels

### Multi-hop gateways

```bash
# Register hops
alogin server add --host bastion.ext.com
alogin server add --host internal-jump

# Define gateway route
alogin net gateway add secure-zone bastion.ext.com internal-jump
alogin net gateway list --format json
alogin net gateway show secure-zone --format json

# Route a server via the gateway
alogin server add --host prod-sql --gateway secure-zone
alogin ssh connect prod-sql
```

### Profiles (global gateway activation)

```bash
alogin net profile add office --gateway secure-zone --desc "Office network"
alogin net profile use office
alogin net profile use none    # disable
```

### Persistent tunnels

```bash
alogin net tunnel add db-proxy \
  --server prod-db --local-port 5432 --remote-port 5432

alogin net tunnel list --format json
alogin net tunnel start db-proxy
alogin net tunnel status db-proxy
alogin net tunnel stop db-proxy
```

### Local DNS overrides

```bash
alogin net hosts list --format json
alogin net hosts show prod-db --format json
```

---

## App Bindings (DB clients, container shells)

```bash
alogin app add --name prod-mysql --server prod-db --app mariadb
alogin app list --format json
alogin app connect prod-mysql
alogin app connect prod-mysql --cmd "SHOW DATABASES"
alogin app show prod-mysql
alogin app delete prod-mysql
alogin app plugin list --format json
```

Credentials are injected via PTY automation — never exposed in arguments or logs.

---

## Server Memory

Shared, append-only operational notes per server. Recall at session start;
save findings before closing.

```bash
# Recall at start of every session
alogin agent server-memory list <server-id>

# Save findings
alogin agent server-memory add <server-id> \
  --text "nginx config: /etc/nginx/nginx.conf"
alogin agent server-memory add <server-id> \
  --text "python3.11 at /usr/local/bin/python3, venv in ~/app/.venv"

# Delete a note
alogin agent server-memory del <server-id> <note-id>
```

Three patterns:
1. **Dynamic profiling** — save log paths, tool locations, service layout after first explore
2. **State drift detection** — save baselines (disk %, process counts); compare next session
3. **Multi-agent handoff** — notes written by one agent are immediately available to the next

---

## JSON Output Reference

```bash
alogin server list --format json | jq '.[].host'
alogin server show <host> --format json
alogin server alias list --format json
alogin net gateway list --format json
alogin net tunnel list --format json
alogin net profile list --format json | jq '.[] | select(.active == true)'
alogin ssh cluster list --format json
alogin ssh cluster prod --cmd "uptime" --format json \
  | jq '.[] | {host, output}'
alogin ssh session job list --json
alogin ssh session job status "$job" --json
alogin app list --format json
```

---

## Safety Rules

- Prefer sessions over one-shot `--cmd`.
- Always read `server show <id> --format json` and check `system_prompt`
  before connecting to act.
- Do not run destructive commands (`rm -rf`, `shutdown`, `DROP TABLE`) without
  explicit user confirmation — HITL policy will block or queue them.
- If a server `device_type` is `router`, `switch`, or `firewall`, do not
  assume standard Linux commands work.
- Use `bg-exec` for long-running commands; never guess at `--timeout` for
  unknown-duration operations.
