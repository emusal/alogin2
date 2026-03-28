package mcp

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/emusal/alogin2/internal/db"
	"github.com/emusal/alogin2/internal/plugin"
	internalssh "github.com/emusal/alogin2/internal/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// mcpRunner adapts an established SSH chain to the plugin.RemoteRunner interface.
// It is used by the alogin_exec_app MCP tool handler.
type mcpRunner struct {
	chain  *internalssh.ChainedClient
	client *internalssh.Client
}

// newMCPRunner dials the server (following its gateway chain) and returns a runner.
// The caller is responsible for calling close() when done.
func newMCPRunner(ctx context.Context, d Deps, serverID int64) (*mcpRunner, error) {
	srv, err := d.DB.Servers.GetByID(ctx, serverID)
	if err != nil || srv == nil {
		return nil, fmt.Errorf("server %d not found", serverID)
	}
	hops, err := buildHopChain(ctx, d.DB, d.Vault, srv)
	if err != nil {
		return nil, fmt.Errorf("build hop chain: %w", err)
	}
	chain, err := internalssh.DialChain(hops)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	return &mcpRunner{chain: chain, client: chain.Terminal()}, nil
}

func (r *mcpRunner) close() {
	r.chain.CloseAll()
}

// RunInteractive is not supported in MCP context.
func (r *mcpRunner) RunInteractive(_ context.Context, _ string, _ []plugin.PTYRule, _ map[string]string) error {
	return fmt.Errorf("interactive plugin sessions are not supported in MCP context; use --cmd to pass a command")
}

// Run executes cmd non-interactively.
func (r *mcpRunner) Run(_ context.Context, cmd string) (string, int, error) {
	out, err := r.client.Run(cmd)
	if err != nil {
		if exitErr, ok := err.(*gossh.ExitError); ok {
			return string(out), exitErr.ExitStatus(), nil
		}
		return string(out), -1, err
	}
	return string(out), 0, nil
}

// RunPTY runs cmd in a PTY session and applies Expect-Send rules.
func (r *mcpRunner) RunPTY(_ context.Context, cmd string, rules []plugin.PTYRule) (string, error) {
	// Convert plugin.PTYRule → mcp.ExpectRule for the existing runPTY infrastructure.
	// EchoSuppressed is handled implicitly — the value is already resolved.
	mcpRules := make([]ExpectRule, 0, len(rules))
	for _, rule := range rules {
		send := rule.Send
		if rule.SendNewline {
			send += "\n"
		}
		mcpRules = append(mcpRules, ExpectRule{Pattern: rule.Pattern, Send: send})
	}

	// Use a fresh PTY session with our own expect writer (same pattern as runPTY).
	sess, err := r.client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()

	if err := sess.RequestPty("xterm", 24, 80, gossh.TerminalModes{
		gossh.ECHO:          1,
		gossh.TTY_OP_ISPEED: 14400,
		gossh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		return "", fmt.Errorf("request pty: %w", err)
	}

	stdinPipe, err := sess.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("stdin pipe: %w", err)
	}

	ew := &mcpPluginExpectWriter{
		rules: mcpRules,
		used:  make([]bool, len(mcpRules)),
		stdin: stdinPipe,
	}
	sess.Stdout = ew
	sess.Stderr = ew

	if err := sess.Start(cmd); err != nil {
		return "", fmt.Errorf("start command: %w", err)
	}
	_ = sess.Wait()

	out := ew.buf.String()
	if len(out) > maxOutputBytes {
		out = out[:maxOutputBytes] + "\n[output truncated]"
	}
	return out, nil
}

// mcpPluginExpectWriter is a local copy of expectWriter tailored for plugin PTY rules.
type mcpPluginExpectWriter struct {
	buf   bytes.Buffer
	rules []ExpectRule
	used  []bool
	stdin interface{ Write([]byte) (int, error) }
	mu    sync.Mutex
}

func (w *mcpPluginExpectWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buf.Write(p)
	recent := w.buf.String()
	for i, rule := range w.rules {
		if !w.used[i] && strings.Contains(recent, rule.Pattern) {
			_, _ = w.stdin.Write([]byte(rule.Send))
			w.used[i] = true
		}
	}
	return n, err
}

// buildMCPAuditEntry constructs a db.AuditEntry for a plugin_exec event.
func buildPluginAuditEntry(serverID int64, serverHost string, sess *plugin.Session) db.AuditEntry {
	id := serverID
	return db.AuditEntry{
		Event:          "plugin_exec",
		ServerID:       &id,
		ServerHost:     serverHost,
		PluginName:     sess.Plugin.Name,
		PluginVars:     sess.AuditVars(),
		PluginStrategy: sess.Strategy.Kind,
	}
}

// Ensure mcpRunner satisfies plugin.RemoteRunner at compile time.
var _ plugin.RemoteRunner = (*mcpRunner)(nil)
