package mcp

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/emusal/alogin2/internal/policy"
	internalssh "github.com/emusal/alogin2/internal/ssh"
	"github.com/google/uuid"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	gossh "golang.org/x/crypto/ssh"
)

// remoteShellSession holds one persistent SSH connection for a remote_shell session.
type remoteShellSession struct {
	chain    *internalssh.ChainedClient
	client   *internalssh.Client
	serverID int64
}

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

			sessionID = uuid.New().String()
			sess = &remoteShellSession{
				chain:    chain,
				client:   chain.Terminal(),
				serverID: serverID,
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

		// Handle "exit": close connection and remove from store.
		if strings.TrimSpace(command) == "exit" {
			globalShellStore.delete(sessionID)
			sess.chain.CloseAll()
			return toolJSON(map[string]any{
				"session_id": sessionID,
				"status":     "closed",
			})
		}

		// No command: session established, return ID.
		if strings.TrimSpace(command) == "" {
			status := "ready"
			if isNew {
				status = "created"
			}
			return toolJSON(map[string]any{
				"session_id": sessionID,
				"status":     status,
			})
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

		var results []CommandResult
		var execErr error
		if pty {
			results, execErr = runPTYCommand(sess.client, command, time.Duration(timeoutSec)*time.Second)
		} else {
			results, execErr = runBatch(sess.client, []string{command})
		}
		if execErr != nil {
			return toolError(fmt.Sprintf("exec: %v", execErr)), nil
		}

		return toolJSON(map[string]any{
			"session_id": sessionID,
			"results":    results,
		})
	}
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
