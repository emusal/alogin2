---
name: alogin-admin
description: >
  alogin administrator operations: HITL approval/denial, trust window
  management, vault credential access, agent policy configuration, and
  audit monitoring. For human operators only — must NOT be loaded into
  an AI agent's context or system prompt.
disable-model-invocation: true
license: Apache-2.0
metadata:
  openclaw:
    requires:
      bins: [alogin]
    homepage: https://github.com/emusal/alogin2
---

# alogin-admin — Administrator Skill

> **This skill is for human operators only.**
> `disable-model-invocation: true` prevents Claude from selecting this skill
> automatically. It must be invoked explicitly with `/alogin-admin`.
>
> **Do not include this skill in an AI agent's system prompt or skill list.**
> Commands here (approve, vault get, policy set) would allow an agent to
> bypass its own safety controls.

Use this skill for:
- Reviewing and acting on pending HITL approval requests
- Managing trust windows (time-bounded auto-approve)
- Reading or rotating vault credentials
- Configuring agent policies and server system prompts
- Monitoring the audit trail
- Validating and dry-running policies

---

## HITL Approval Workflow

When an agent's command matches a `require_approval` policy rule, execution
pauses and a token is written to the pending queue. The operator acts here.

```bash
# See all waiting requests
alogin agent pending

# Approve — agent resumes immediately
alogin agent approve <token>

# Deny — agent receives an error and stops
alogin agent deny <token>
```

Approval files live in `~/.config/alogin/hitl/{pending,approved,denied}/`.
Requests auto-deny after `hitl_timeout_sec` seconds (default 120).

---

## Trust Windows

Grant a time-bounded auto-approve window so every request in that period
does not require manual approval.

```bash
# Global: all agents and servers
alogin agent trust --duration 1h

# Scoped to one agent
alogin agent trust --duration 30m --agent claude-dev

# Scoped to one server (by server ID)
alogin agent trust --duration 2h --server 3

# List active windows
alogin agent trust-list

# Revoke immediately
alogin agent untrust                    # global
alogin agent untrust --agent claude-dev # agent-scoped
```

Use trust windows during supervised deployment windows, not as a permanent
bypass. Always revoke when the window is no longer needed.

---

## Vault — Credential Management

Vault stores encrypted passwords for servers. Access is restricted to the
local OS keychain (macOS Keychain, Linux Secret Service) or an age-encrypted
file.

```bash
# Read a stored password (prints to stdout — handle with care)
alogin vault get <user>@<host>

# Store or overwrite a password
alogin vault set <user>@<host>

# Delete a stored password
alogin vault delete <user>@<host>

# Rotate a server's vault password interactively
alogin server passwd <host>
```

> **Security note:** `vault get` prints the plaintext password to stdout.
> Do not run in shared terminal sessions or CI environments with log capture.

---

## Agent Policy Configuration

### Global policy

The global policy file is `~/.config/alogin/agent-policy.yaml`.

```bash
# Validate syntax and regex errors
alogin agent policy validate

# Show the currently loaded policy
alogin agent policy show
```

Policy structure:

```yaml
version: 1
default_action: allow        # allow | deny | require_approval
hitl_timeout_sec: 120
rules:
  - name: "block-destructive"
    match:
      commands: ["^rm\\s+-[rRfF]*[rR]", "^(shutdown|reboot|halt)$"]
      agent_id: ["claude-*"]
      server_ids: [3, 7]
      time_window: "22:00-06:00"   # UTC
    action: require_approval       # allow | deny | require_approval
```

Rules are evaluated top-to-bottom, first match wins. All `match` fields are
AND conditions; omitted fields are wildcards.

### Per-server policy

Overrides the global policy for one server (no fallback — full replacement).

```bash
# Set from file
alogin agent server-policy set <server-id> --file policy.yaml

# Set from stdin
cat <<'EOF' | alogin agent server-policy set <server-id> --stdin
version: 1
default_action: require_approval
rules:
  - name: allow-reads
    match:
      commands: ["^(cat|ls|ps|df|free|uptime|who)\\b"]
    action: allow
EOF

# View current per-server policy
alogin agent server-policy show <server-id>

# Remove per-server policy (reverts to global)
alogin agent server-policy clear <server-id>
```

### Per-server system prompt

The system prompt contains server-specific operational restrictions that
agents must read and follow before connecting.

```bash
alogin agent server-prompt set <server-id> --text "Only run read-only commands."
alogin server show <server-id> --format json   # system_prompt field in output
```

### Policy dry-run

Test whether a command would be allowed before an agent runs it.

```bash
alogin agent policy dry-run --cmd "rm -rf /tmp/old"
alogin agent policy dry-run --cmd "df -h" --cmd "uptime"
alogin agent policy dry-run --cmd "shutdown now" --agent claude-dev --server 3
alogin agent policy dry-run --cmd "DROP TABLE users" --json   # machine-readable
```

---

## Audit Monitoring

All `exec_command`, `exec_on_cluster`, `inspect_node`, and `remote_shell`
calls are appended to `~/.config/alogin/audit.jsonl`.

```bash
# Recent events (last 1 hour)
alogin agent audit list --since 1h

# JSON for filtering
alogin agent audit list --since 24h --format json \
  | jq '.[] | select(.policy_action == "require_approval")'

# Stream new events in real time
alogin agent audit tail --format json
```

Each audit entry contains: `timestamp`, `event`, `agent_id`, `server_id`,
`server_host`, `commands`, `intent`, `policy_action`.

---

## Built-in Destructive Command Patterns

Even without a policy file, these patterns always trigger `require_approval`:

| Pattern | Matches |
|---------|---------|
| `^rm\s` / `^rm$` | any rm |
| `\brm\s+-[rRfFi]*[rR]` | rm -rf variants |
| `^dd\s` | dd |
| `^mkfs` | mkfs |
| `^(shutdown\|reboot\|halt\|poweroff)` | system shutdown |
| `^(systemctl\|service)\s+(stop\|disable\|mask)` | service stop |
| `(?i)(^drop\|^truncate)\s+.*table` | DB table deletion |
| `^>\s` | file overwrite redirection |

---

## Common Admin Scenarios

### Supervised deployment window

```bash
# Open 2-hour trust window for the deploy agent on prod server (ID 5)
alogin agent trust --duration 2h --agent claude-deploy --server 5

# Monitor in real time during the window
alogin agent audit tail --format json

# Revoke when deployment is done
alogin agent untrust --agent claude-deploy
```

### Lock down a production server

```bash
cat <<'EOF' | alogin agent server-policy set 5 --stdin
version: 1
default_action: deny
rules:
  - name: allow-reads
    match:
      commands: ["^(cat|ls|ps|df|free|uptime|who|w|grep|find|stat)\\b"]
    action: allow
  - name: require-approval-writes
    match:
      commands: ["^(systemctl|service)\\s+(start|stop|restart)"]
    action: require_approval
EOF

alogin agent server-prompt set 5 --text \
  "Production DB. Read-only unless explicitly approved. No schema changes."
```

### Review pending requests and respond

```bash
alogin agent pending
# → TOKEN=abc123  agent=claude-dev  server=prod-db  cmd="systemctl restart nginx"

alogin agent approve abc123   # or: alogin agent deny abc123
```
