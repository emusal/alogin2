# VSCode Extension — alogin Skills

**Location**: `vscode-extension/`  
**Extension ID**: `emusal.alogin-skills`  
**Min VS Code**: 1.85.0  
**Entry point**: `dist/extension.js` (built from `src/extension.ts` via esbuild)

---

## Purpose

A VSCode/Cursor UI layer that surfaces alogin's infrastructure automation capabilities as sidebar skills and Cursor AI-accessible MCP tools. All communication goes through the `alogin agent mcp` subprocess (JSON-RPC 2.0 over stdio).

---

## Architecture

```
VSCode Extension (TypeScript)
  ├── src/extension.ts            — activate(), warm-start MCP, register commands
  ├── src/utils/aloginMcp.ts      — MCP client singleton (spawns alogin agent mcp)
  ├── src/commands/               — one file per skill
  ├── src/providers/              — activity bar sidebar tree view
  ├── src/webviews/               — rich HTML panels (health report, HITL, skill builder)
  └── src/utils/                  — custom skill storage, Cursor registration, i18n

alogin binary ($PATH or alogin.mcpServerPath setting)
  └── alogin agent mcp            — 26 MCP tools over stdio JSON-RPC 2.0
```

---

## MCP Client (`src/utils/aloginMcp.ts`)

- Singleton that spawns `alogin agent mcp` as a child process
- Routes JSON-RPC requests by ID; 30 s timeout per call
- Convenience wrappers: `listServers()`, `inspectNode()`, `logAnalyzer()`, `execCommand()`, `policyDryRun()`
- Lazy-connects on first use; warm-started on extension activation

---

## Built-in Skills

Seeded in `src/utils/customSkills.ts` (`SEED_SKILLS` array). Idempotent — only inserted once.

| Skill | File | Description |
|-------|------|-------------|
| Post-Deploy Health Check | `healthCheck.ts` | `inspect_node` + HTTP `/health` probe + log analysis → rich webview dashboard |
| Smart Rollback | `smartRollback.ts` | `policy_dry_run` → HITL approval webview with countdown → execute rollback |
| Log & Error Triage | `logTriage.ts` | Parallel `inspect_node` + `log_analyzer` → regex error detection → output channel |
| Infra Drift Detector | `driftDetector.ts` | Snapshot server state → save baseline → VS Code native diff against current |
| Security Quick Scan | `securityScan.ts` | Auth logs, sudoers, open ports, SUID files → annotated output channel |
| One-Click Deploy + Smoke | `deploySmoke.ts` | User enters deploy command → policy check → execute → HTTP smoke test |

---

## Custom Skills

- Stored in `vscode.ExtensionContext.globalState` (not on disk)
- Two types: `exec_command` (list of shell commands) or `run_script` (bash script)
- Properties: title, description, icon, intent, `requireApproval`
- Support single-server and cluster (parallel) execution
- CRUD via skill builder webview (`src/webviews/SkillBuilderWebview.ts`)

---

## Server / Cluster Picker (`src/commands/serverPicker.ts`)

- Fetches server list via `list_servers` MCP tool
- Single-server: QuickPick with fuzzy search on host/note/tags
- Cluster: three modes — full fleet, by group, or hand-selected multi-pick

---

## HITL Flow (`src/webviews/RollbackWebview.ts`)

1. Skill calls `policy_dry_run` before any destructive operation
2. If policy returns `require_approval`, opens HITL webview with countdown (default 120 s, configurable via `alogin.hitlTimeoutSec`)
3. User approves → `alogin agent approve <token>` via terminal
4. User denies or timer expires → `alogin agent deny <token>`

---

## Cursor Integration (`src/utils/mcpRegistration.ts`)

Auto-registers `alogin` as a Cursor MCP server on activation (no-op in vanilla VS Code):

```json
"cursor.mcpServers": {
  "alogin": {
    "command": "alogin",
    "args": ["agent", "mcp"]
  }
}
```

Cursor's Claude agent can then call `@alogin` and all 26 alogin tools natively.

---

## Configuration Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `alogin.mcpServerPath` | `"alogin"` | Path to alogin binary |
| `alogin.defaultModel` | `"claude-sonnet-4-6"` | AI model for Cursor MCP |
| `alogin.endpoint` | `""` | Custom API endpoint |
| `alogin.telemetry` | `true` | Opt-in usage telemetry |
| `alogin.hitlTimeoutSec` | `120` | HITL approval countdown (10–3600 s) |

---

## Output Modes

| Mode | Used by |
|------|---------|
| Output channel (searchable, persistent) | Log triage, security scan |
| Webview (rich HTML dashboard) | Health report, HITL approval |
| VS Code diff editor | Drift detector |
| Status bar notifications | Deploy smoke test, errors |

---

## Build

```bash
cd vscode-extension
npm install
npm run compile        # esbuild → dist/extension.js
npm run watch          # dev watch mode
```

---

## Internationalization (`src/utils/i18n.ts`)

Auto-detects `vscode.env.language`; supports `ko` (Korean) and `en` (fallback). Simple key-value map — no pluralization.

---

## Key Files

| File | Purpose |
|------|---------|
| `src/extension.ts` | Activation, command registration, sidebar setup |
| `src/utils/aloginMcp.ts` | MCP client (JSON-RPC 2.0 over stdio) |
| `src/utils/customSkills.ts` | Skill storage + `SEED_SKILLS` built-in definitions |
| `src/utils/mcpRegistration.ts` | Cursor MCP server registration |
| `src/commands/commands.ts` | Command registration hub |
| `src/commands/serverPicker.ts` | Server / cluster selection QuickPick UI |
| `src/providers/SkillsTreeProvider.ts` | Activity bar tree view data provider |
| `src/webviews/HealthReportWebview.ts` | Health dashboard (CPU, mem, disk, alerts) |
| `src/webviews/RollbackWebview.ts` | HITL approval panel with countdown |
| `src/webviews/SkillBuilderWebview.ts` | Custom skill creation / editing UI |
| `package.json` | Extension manifest + Cursor MCP config |
