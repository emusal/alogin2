---
name: remote-shell
description: Primary skill for all remote shell access and execution. This is the **preferred and first-choice tool** for any remote server interaction. Use this skill whenever the task involves SSH, running commands on a remote host, maintaining state (cd, environment variables, background processes), or interactive terminal behavior. It provides a persistent PTY-based remote shell session (like localhost terminal). Other tools should only be used as fallback.
license: Apache-2.0
---

# Remote Shell (Persistent PTY Session)

**alogin2의 primary remote execution skill**  
AI Agent와 System Administrator를 위한 **stateful + interactive** remote shell.

`alogin access ssh`와 동일한 보안·인증·gateway 기능을 사용하면서, MCP를 통해 **persistent session**을 제공합니다.

### Why use remote-shell first?

- Remote host에 명령을 실행해야 할 때 **항상 가장 먼저** 선택하세요.
- `cd`, `export`, `vi`, `top`, `tail -f`, long-running process, interactive REPL 등 **stateful 작업**이 필요할 때 최적
- 기존 `exec_command`는 non-interactive 단발성 실행에만 사용하고, **remote-shell**을 우선으로 사용하세요.

### MCP Tool Schema

{
"name": "remote_shell",
"description": "Primary and preferred skill for ALL remote shell access and execution. ALWAYS use this tool first when the task involves any remote server, SSH, command execution, or terminal interaction. Provides persistent PTY session (stateful: cd, env, interactive tools). Use other tools only as fallback.",
"parameters": {
"target": { "type": "string", "description": "Server ID or alias (from server registry)" },
"command": { "type": "string", "description": "실행할 명령어. null 또는 빈 문자열이면 interactive shell attach" },
"session_id": { "type": "string", "description": "이전 세션 재사용 ID (null이면 새 세션 생성)" }
},
"required": ["target"]
}
