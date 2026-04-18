package mcp

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emusal/alogin2/internal/policy"
	internalssh "github.com/emusal/alogin2/internal/ssh"
	"github.com/google/uuid"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	gossh "golang.org/x/crypto/ssh"
)

// cmdRecord tracks a single command execution in a session for debugging.
type cmdRecord struct {
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
	Failed   bool   `json:"failed"`
}

// remoteShellSession holds one persistent SSH connection and its managed bash
// process for a remote_shell session.
type remoteShellSession struct {
	chain      *internalssh.ChainedClient
	managed    *internalssh.ManagedSession
	serverID   int64
	loginShell bool
	// hops is stored so the session can reconnect after an i/o timeout.
	hops    []internalssh.HopConfig
	history []cmdRecord // last N commands (capped at historyMax)
	lastPwd string      // last known pwd (updated on every exec)
}

const historyMax = 20

// remoteShellStore is a process-global in-memory map of active sessions.
// MCP runs as a single persistent process, so this lives for the process lifetime.
type remoteShellStore struct {
	mu       sync.RWMutex
	sessions map[string]*remoteShellSession
}

var globalShellStore = &remoteShellStore{
	sessions: make(map[string]*remoteShellSession),
}

func (s *remoteShellStore) get(id string) (*remoteShellSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	return sess, ok
}

func (s *remoteShellStore) put(id string, sess *remoteShellSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = sess
}

func (s *remoteShellStore) delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

// newRemoteShellHandler returns the handler closure for the remote_shell MCP tool.
func newRemoteShellHandler(d Deps) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()

		// Parse parameters.
		serverID, err := parseID(req, "target")
		if err != nil {
			return toolError(err.Error()), nil
		}
		command, _ := args["command"].(string)
		sessionID, _ := args["session_id"].(string)
		agentID, _ := args["agent_id"].(string)
		intent, _ := args["intent"].(string)
		timeoutSec, _ := args["timeout_sec"].(float64)
		if timeoutSec <= 0 {
			timeoutSec = 120
		}
		loginShell := true
		if v, ok := args["login_shell"]; ok {
			loginShell, _ = v.(bool)
		}
		preflight, _ := args["preflight"].(bool)

		// Resolve or create session.
		var sess *remoteShellSession
		isNew := false

		if sessionID == "" {
			// Create a new session: fetch server, audit, policy-check, dial.
			srv, err := d.DB.Servers.GetByID(ctx, serverID)
			if err != nil || srv == nil {
				return toolError(fmt.Sprintf("server %d not found", serverID)), nil
			}

			cmdList := []string{}
			if command != "" {
				cmdList = []string{command}
			}

			ev := auditEvent{
				Event:      "remote_shell",
				AgentID:    agentID,
				ServerID:   serverID,
				ServerHost: srv.Host,
				Commands:   cmdList,
				Intent:     intent,
				TimeoutSec: int(timeoutSec),
			}
			writeAudit(d.AuditLog, ev)
			writeAuditDB(ctx, d, ev)

			eng, err := policy.ResolveFor(d.Policy, srv.PolicyYAML)
			if err != nil {
				return toolError(fmt.Sprintf("per-server policy error: %v", err)), nil
			}
			if blocked := checkPolicy(ctx, d, eng, policy.CheckRequest{
				AgentID:  agentID,
				Commands: cmdList,
				ServerID: serverID,
			}); blocked != nil {
				return blocked, nil
			}

			hops, err := buildHopChain(ctx, d.DB, d.Vault, srv)
			if err != nil {
				return toolError(fmt.Sprintf("build hop chain: %v", err)), nil
			}
			chain, err := internalssh.DialChain(hops)
			if err != nil {
				return toolError(fmt.Sprintf("dial: %v", err)), nil
			}

			managed, err := internalssh.NewManagedSession(chain.Terminal(), loginShell)
			if err != nil {
				chain.CloseAll()
				return toolError(fmt.Sprintf("start managed session: %v", err)), nil
			}
			sessionID = uuid.New().String()
			sess = &remoteShellSession{
				chain:      chain,
				managed:    managed,
				serverID:   serverID,
				loginShell: loginShell,
				hops:       hops,
			}
			globalShellStore.put(sessionID, sess)
			isNew = true
		} else {
			// Reuse existing session.
			var ok bool
			sess, ok = globalShellStore.get(sessionID)
			if !ok {
				return toolError(fmt.Sprintf("session %q not found or expired", sessionID)), nil
			}
		}

		// Handle "exit": close managed bash process, connection, remove from store.
		if strings.TrimSpace(command) == "exit" {
			globalShellStore.delete(sessionID)
			_ = sess.managed.Close()
			sess.chain.CloseAll()
			return toolJSON(map[string]any{
				"session_id": sessionID,
				"status":     "closed",
			})
		}

		// Handle "status": return session state without executing anything.
		if strings.TrimSpace(command) == "status" {
			pwd := sess.lastPwd
			if pwd == "" {
				if r, err := sess.managed.Exec(ctx, "pwd", 10*time.Second); err == nil {
					pwd = strings.TrimSpace(r.Output)
					sess.lastPwd = pwd
				}
			}
			return toolJSON(map[string]any{
				"session_id": sessionID,
				"pwd":        pwd,
				"history":    sess.history,
			})
		}

		// No command: session established — return ID plus current context.
		// For new sessions also include system_prompt and agent memories so the
		// agent has full server context without additional round trips.
		if strings.TrimSpace(command) == "" {
			status := "ready"
			if isNew {
				status = "created"
			}
			ctx2 := map[string]string{}
			if r, err := sess.managed.Exec(ctx, "whoami && pwd", 10*time.Second); err == nil {
				lines := strings.SplitN(strings.TrimSpace(r.Output), "\n", 2)
				if len(lines) >= 1 {
					ctx2["user"] = strings.TrimSpace(lines[0])
				}
				if len(lines) >= 2 {
					pwd := strings.TrimSpace(lines[1])
					ctx2["pwd"] = pwd
					sess.lastPwd = pwd
				}
			}
			resp := map[string]any{
				"session_id": sessionID,
				"status":     status,
				"context":    ctx2,
			}
			if isNew {
				// Attach server_prompt and memories once at session creation so
				// the agent can read operational instructions without extra calls.
				srvInfo, _ := d.DB.Servers.GetByID(ctx, serverID)
				if srvInfo != nil && srvInfo.SystemPrompt != "" {
					resp["system_prompt"] = srvInfo.SystemPrompt
				}
				memories, err := d.DB.AgentMemory.ListByServer(ctx, serverID)
				if err == nil && len(memories) > 0 {
					resp["memories"] = memories
				}
				if preflight {
					resp["preflight"] = runPreflight(ctx, sess)
				}
			}
			return toolJSON(resp)
		}

		pty, _ := args["pty"].(bool)

		// Execute command. For reused sessions, audit-log the command now.
		if !isNew {
			ev := auditEvent{
				Event:      "remote_shell",
				AgentID:    agentID,
				ServerID:   sess.serverID,
				Commands:   []string{command},
				Intent:     intent,
				TimeoutSec: int(timeoutSec),
			}
			writeAudit(d.AuditLog, ev)
			writeAuditDB(ctx, d, ev)
		}

		if pty {
			// PTY mode: for TUI programs (top, vi, etc.) that require a terminal.
			results, execErr := runPTYCommand(sess.chain.Terminal(), command, time.Duration(timeoutSec)*time.Second)
			if execErr != nil {
				return toolError(fmt.Sprintf("exec: %v", execErr)), nil
			}
			return toolJSON(map[string]any{
				"session_id": sessionID,
				"results":    results,
			})
		}

		// Stateful execution via persistent bash: cwd and env are preserved.
		// On i/o error (e.g. timeout dropped the TCP connection), attempt one
		// automatic reconnect so the agent can keep using the same session_id.
		result, execErr := sess.managed.Exec(ctx, command, time.Duration(timeoutSec)*time.Second)
		if execErr != nil {
			sess.appendHistory(command, -1)
			reconnected, reconnErr := reconnectSession(sess)
			if reconnErr != nil {
				// Both original exec and reconnect failed — give up.
				globalShellStore.delete(sessionID)
				return toolError(fmt.Sprintf("exec failed and reconnect failed: %v; reconnect: %v", execErr, reconnErr)), nil
			}
			// Retry the command on the fresh connection.
			result, execErr = reconnected.managed.Exec(ctx, command, time.Duration(timeoutSec)*time.Second)
			if execErr != nil {
				globalShellStore.delete(sessionID)
				return toolError(fmt.Sprintf("exec after reconnect: %v", execErr)), nil
			}
			sess.appendHistory(command, result.ExitCode)
			sess.updatePwd(ctx, command)
			return toolJSON(map[string]any{
				"session_id":  sessionID,
				"output":      result.Output,
				"exit_code":   result.ExitCode,
				"reconnected": true,
			})
		}
		sess.appendHistory(command, result.ExitCode)
		sess.updatePwd(ctx, command)
		return toolJSON(map[string]any{
			"session_id": sessionID,
			"output":     result.Output,
			"exit_code":  result.ExitCode,
		})
	}
}

// appendHistory appends a command record to the session history (capped at historyMax).
func (s *remoteShellSession) appendHistory(cmd string, exitCode int) {
	rec := cmdRecord{Command: cmd, ExitCode: exitCode, Failed: exitCode != 0}
	s.history = append(s.history, rec)
	if len(s.history) > historyMax {
		s.history = s.history[len(s.history)-historyMax:]
	}
}

// updatePwd refreshes lastPwd only when the command may have changed the directory.
// This avoids an extra round-trip on every exec — only cd-like commands trigger a refresh.
func (s *remoteShellSession) updatePwd(ctx context.Context, cmd string) {
	lower := strings.ToLower(strings.TrimSpace(cmd))
	if !strings.HasPrefix(lower, "cd") && !strings.Contains(lower, " cd ") && !strings.Contains(lower, ";cd") {
		return
	}
	if r, err := s.managed.Exec(ctx, "pwd", 5*time.Second); err == nil {
		s.lastPwd = strings.TrimSpace(r.Output)
	}
}

// preflightResult holds the output of an opt-in preflight check.
type preflightResult struct {
	DiskFreeGB  float64  `json:"disk_free_gb"`
	MemFreeGB   float64  `json:"mem_free_gb"`
	Warnings    []string `json:"warnings,omitempty"`
	CommonPaths []string `json:"common_paths,omitempty"` // paths that exist
}

// runPreflight executes a quick health check on a fresh session and returns a summary.
func runPreflight(ctx context.Context, sess *remoteShellSession) preflightResult {
	var pf preflightResult

	// Disk and memory snapshot
	if r, err := sess.managed.Exec(ctx,
		`df -P / | awk 'NR==2{print $4}' && free -b 2>/dev/null | awk '/^Mem:/{print $4}' || vm_stat | awk '/Pages free/{gsub(/\./,"",$3); print $3*4096}'`,
		10*time.Second); err == nil {
		lines := strings.Fields(strings.TrimSpace(r.Output))
		if len(lines) >= 1 {
			if v, err := strconv.ParseInt(lines[0], 10, 64); err == nil {
				pf.DiskFreeGB = float64(v*1024) / 1e9
			}
		}
		if len(lines) >= 2 {
			if v, err := strconv.ParseInt(lines[1], 10, 64); err == nil {
				pf.MemFreeGB = float64(v) / 1e9
			}
		}
	}

	if pf.DiskFreeGB > 0 && pf.DiskFreeGB < 1 {
		pf.Warnings = append(pf.Warnings, fmt.Sprintf("Low disk space: %.2f GB free on /", pf.DiskFreeGB))
	}
	if pf.MemFreeGB > 0 && pf.MemFreeGB < 0.25 {
		pf.Warnings = append(pf.Warnings, fmt.Sprintf("Low memory: %.2f GB free", pf.MemFreeGB))
	}

	// Check common operational paths
	checkPaths := []string{"/var/log", "/tmp", "/etc", "/opt", "/home"}
	script := "for p in " + strings.Join(checkPaths, " ") + "; do [ -d \"$p\" ] && echo \"$p\"; done"
	if r, err := sess.managed.Exec(ctx, script, 10*time.Second); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(r.Output), "\n") {
			if line != "" {
				pf.CommonPaths = append(pf.CommonPaths, line)
			}
		}
	}

	return pf
}

// reconnectSession tears down the stale SSH connection in sess and establishes
// a fresh one using the stored hop configuration. On success, sess is updated
// in-place in the global store and returned; on failure the caller should
// evict the session.
func reconnectSession(sess *remoteShellSession) (*remoteShellSession, error) {
	// Best-effort cleanup of the dead connection.
	_ = sess.managed.Close()
	sess.chain.CloseAll()

	chain, err := internalssh.DialChain(sess.hops)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	managed, err := internalssh.NewManagedSession(chain.Terminal(), sess.loginShell)
	if err != nil {
		chain.CloseAll()
		return nil, fmt.Errorf("managed session: %w", err)
	}
	sess.chain = chain
	sess.managed = managed
	return sess, nil
}

// runPTYCommand runs a single command in a PTY session with a timeout, collecting
// all output. Used for TUI commands (top, watch, vi, etc.) that require TERM to be set.
func runPTYCommand(client *internalssh.Client, command string, timeout time.Duration) ([]CommandResult, error) {
	sess, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer sess.Close()

	if err := sess.RequestPty("xterm-256color", 40, 220, gossh.TerminalModes{
		gossh.ECHO:          0,
		gossh.TTY_OP_ISPEED: 14400,
		gossh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		return nil, fmt.Errorf("request pty: %w", err)
	}

	var buf bytes.Buffer
	sess.Stdout = &buf
	sess.Stderr = &buf

	if err := sess.Start(command); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}

	done := make(chan error, 1)
	go func() { done <- sess.Wait() }()

	select {
	case <-time.After(timeout):
		_ = sess.Signal(gossh.SIGINT)
		<-done
	case <-done:
	}

	out := buf.Bytes()
	if len(out) > maxOutputBytes {
		out = append(out[:maxOutputBytes], []byte("\n[output truncated]")...)
	}
	return []CommandResult{{Command: command, Output: string(out)}}, nil
}
