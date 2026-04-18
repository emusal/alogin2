// Package mcp implements the alogin MCP (Model Context Protocol) server.
package mcp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/emusal/alogin2/internal/db"
	"github.com/emusal/alogin2/internal/model"
	internalssh "github.com/emusal/alogin2/internal/ssh"
	"github.com/emusal/alogin2/internal/vault"
	gossh "golang.org/x/crypto/ssh"
)

const (
	maxOutputBytes = 64 * 1024 // 64 KB per command
	defaultTimeout = 30 * time.Second
)

// CommandResult holds the output of a single command execution.
type CommandResult struct {
	Command  string `json:"command"`
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// ExecRequest describes a single-server exec request.
type ExecRequest struct {
	ServerID   int64
	Commands   []string
	Expect     []ExpectRule
	TimeoutSec int
}

// ExpectRule is a pattern→response pair for interactive mode.
type ExpectRule struct {
	Pattern string `json:"pattern"`
	Send    string `json:"send"`
}

// expectWriter forwards stdout to a buffer while matching expect rules.
type expectWriter struct {
	buf   bytes.Buffer
	rules []ExpectRule
	used  []bool
	stdin io.Writer
	mu    sync.Mutex
}

func (w *expectWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buf.Write(p)
	recent := w.buf.String()
	for i, rule := range w.rules {
		if !w.used[i] && strings.Contains(recent, rule.Pattern) {
			_, _ = w.stdin.Write([]byte(rule.Send + "\n"))
			w.used[i] = true
		}
	}
	return n, err
}

// ExecOnServer is the exported entry point for executing commands on a server.
// It is used by both the MCP tool handlers and the CLI cluster --cmd flag.
func ExecOnServer(ctx context.Context, database *db.DB, vlt vault.Vault, req ExecRequest) ([]CommandResult, error) {
	return execOnServer(ctx, database, vlt, req)
}

// execOnServer connects to a server (following the active profile's gateway chain)
// and runs commands.
func execOnServer(ctx context.Context, database *db.DB, vlt vault.Vault, req ExecRequest) ([]CommandResult, error) {
	srv, err := database.Servers.GetByID(ctx, req.ServerID)
	if err != nil || srv == nil {
		return nil, fmt.Errorf("server %d not found", req.ServerID)
	}

	timeout := defaultTimeout
	if req.TimeoutSec > 0 {
		timeout = time.Duration(req.TimeoutSec) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	hops, err := buildHopChain(ctx, database, vlt, srv)
	if err != nil {
		return nil, err
	}

	chain, err := internalssh.DialChain(hops)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	defer chain.CloseAll()

	client := chain.Terminal()

	if len(req.Expect) > 0 {
		return runPTY(client, req.Commands, req.Expect)
	}
	return runBatch(client, req.Commands)
}

// runBatch runs each command in its own session (non-interactive).
func runBatch(client *internalssh.Client, commands []string) ([]CommandResult, error) {
	var results []CommandResult
	for _, cmd := range commands {
		out, err := runOneCommand(client.Inner(), cmd)
		exitCode := 0
		errStr := ""
		if err != nil {
			if exitErr, ok := err.(*gossh.ExitError); ok {
				exitCode = exitErr.ExitStatus()
			} else {
				errStr = err.Error()
			}
		}
		if len(out) > maxOutputBytes {
			out = append(out[:maxOutputBytes], []byte("\n[output truncated]")...)
		}
		results = append(results, CommandResult{
			Command:  cmd,
			Output:   string(out),
			ExitCode: exitCode,
			Error:    errStr,
		})
	}
	return results, nil
}

// runOneCommand opens a fresh session and runs a single command.
func runOneCommand(inner *gossh.Client, cmd string) ([]byte, error) {
	sess, err := inner.NewSession()
	if err != nil {
		return nil, err
	}
	defer sess.Close()
	return sess.CombinedOutput(cmd)
}

// runPTY runs all commands as a single joined command in one PTY session.
func runPTY(client *internalssh.Client, commands []string, rules []ExpectRule) ([]CommandResult, error) {
	sess, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer sess.Close()

	if err := sess.RequestPty("xterm", 24, 80, gossh.TerminalModes{
		gossh.ECHO:          1,
		gossh.TTY_OP_ISPEED: 14400,
		gossh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		return nil, fmt.Errorf("request pty: %w", err)
	}

	stdinPipe, err := sess.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	ew := &expectWriter{
		rules: rules,
		used:  make([]bool, len(rules)),
		stdin: stdinPipe,
	}
	sess.Stdout = ew
	sess.Stderr = ew

	joined := strings.Join(commands, " && ")
	if err := sess.Start(joined); err != nil {
		return nil, fmt.Errorf("start command: %w", err)
	}
	_ = sess.Wait()

	out := ew.buf.Bytes()
	if len(out) > maxOutputBytes {
		out = append(out[:maxOutputBytes], []byte("\n[output truncated]")...)
	}

	return []CommandResult{{
		Command: joined,
		Output:  string(out),
	}}, nil
}

// buildHopChain constructs the SSH hop list for a server using the active profile.
// Always follows the active profile's gateway — MCP context doesn't support per-call overrides.
// Loop prevention: if the destination is one of the gateway hop servers, connect directly.
func buildHopChain(ctx context.Context, database *db.DB, vlt vault.Vault, srv *model.Server) ([]internalssh.HopConfig, error) {
	var hops []internalssh.HopConfig

	activeProfile, err := database.Profiles.GetActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("get active profile: %w", err)
	}

	if activeProfile != nil && activeProfile.GatewayRouteID != nil {
		gw, err := database.Gateways.GetByID(ctx, *activeProfile.GatewayRouteID)
		if err != nil || gw == nil {
			return nil, fmt.Errorf("profile gateway route not found: %w", err)
		}

		// Loop prevention: if destination is one of the gateway hops, connect directly.
		isGWHop := false
		for _, h := range gw.Hops {
			if h.ServerID == srv.ID {
				isGWHop = true
				break
			}
		}

		if !isGWHop {
			for _, h := range gw.Hops {
				hopSrv, err := database.Servers.GetByID(ctx, h.ServerID)
				if err != nil || hopSrv == nil {
					return nil, fmt.Errorf("gateway hop server %d not found", h.ServerID)
				}
				pwd, _ := vlt.Get(hopSrv.User + "@" + hopSrv.Host)
				hops = append(hops, internalssh.HopConfig{
					Host:       database.Hosts.Resolve(ctx, hopSrv.Host),
					Port:       hopSrv.EffectivePort(),
					User:       hopSrv.User,
					Password:   pwd,
					AuthMethod: hopSrv.AuthMethod,
					KeyPath:    hopSrv.IdentityFile,
				})
			}
		}
	}

	// Server-level internal gateway hops (applied after profile gateway, before destination).
	// Dedup: if server gateway is the same route as the profile gateway, skip it.
	if srv.GatewayRouteID != nil && (activeProfile == nil ||
		activeProfile.GatewayRouteID == nil ||
		*srv.GatewayRouteID != *activeProfile.GatewayRouteID) {
		srvGW, err := database.Gateways.GetByID(ctx, *srv.GatewayRouteID)
		if err != nil || srvGW == nil {
			return nil, fmt.Errorf("server gateway route %d not found", *srv.GatewayRouteID)
		}
		for _, h := range srvGW.Hops {
			if h.ServerID == srv.ID {
				continue
			}
			hopSrv, err := database.Servers.GetByID(ctx, h.ServerID)
			if err != nil || hopSrv == nil {
				return nil, fmt.Errorf("server gateway hop %d not found", h.ServerID)
			}
			pwd, _ := vlt.Get(hopSrv.User + "@" + hopSrv.Host)
			hops = append(hops, internalssh.HopConfig{
				Host:       database.Hosts.Resolve(ctx, hopSrv.Host),
				Port:       hopSrv.EffectivePort(),
				User:       hopSrv.User,
				Password:   pwd,
				AuthMethod: hopSrv.AuthMethod,
				KeyPath:    hopSrv.IdentityFile,
			})
		}
	}

	pwd, _ := vlt.Get(srv.User + "@" + srv.Host)
	hops = append(hops, internalssh.HopConfig{
		Host:       database.Hosts.Resolve(ctx, srv.Host),
		Port:       srv.EffectivePort(),
		User:       srv.User,
		Password:   pwd,
		AuthMethod: srv.AuthMethod,
		KeyPath:    srv.IdentityFile,
	})
	return hops, nil
}
