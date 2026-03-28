package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/emusal/alogin2/internal/plugin"
	internalssh "github.com/emusal/alogin2/internal/ssh"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// sshRunner adapts *internalssh.Client to the plugin.RemoteRunner interface.
type sshRunner struct {
	client *internalssh.Client
}

func newSSHRunner(c *internalssh.Client) plugin.RemoteRunner {
	return &sshRunner{client: c}
}

// RunInteractive runs cmd with a PTY. If rules are provided, Expect-Send
// automation is applied first (e.g. password prompt), then stdin is handed
// to the user for the rest of the interactive session.
func (r *sshRunner) RunInteractive(_ context.Context, cmd string, rules []plugin.PTYRule, env map[string]string) error {
	// No rules → delegate directly to Shell (simpler path).
	if len(rules) == 0 {
		return r.client.Shell(internalssh.ShellOptions{Command: cmd, Env: env})
	}

	sess, err := r.client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()

	fd := int(os.Stdin.Fd())
	w, h, err := term.GetSize(fd)
	if err != nil {
		w, h = 80, 24
	}
	termType := os.Getenv("TERM")
	if termType == "" {
		termType = "xterm-256color"
	}
	if err := sess.RequestPty(termType, h, w, gossh.TerminalModes{
		gossh.ECHO: 1, gossh.TTY_OP_ISPEED: 14400, gossh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		return err
	}

	for k, v := range env {
		_ = sess.Setenv(k, v)
	}

	stdinPipe, err := sess.StdinPipe()
	if err != nil {
		return err
	}

	// interactiveExpectWriter handles Expect-Send rules, then passes output
	// to os.Stdout. Once all rules are fired, stdin is bridged from os.Stdin.
	iew := &interactiveExpectWriter{
		rules:     rules,
		used:      make([]bool, len(rules)),
		stdin:     stdinPipe,
		realStdin: os.Stdin,
	}
	sess.Stdout = iew
	sess.Stderr = os.Stderr

	// Put terminal in raw mode so the interactive session feels native.
	if term.IsTerminal(fd) {
		oldState, err := term.MakeRaw(fd)
		if err == nil {
			defer term.Restore(fd, oldState)
		}
	}

	if err := sess.Start(cmd); err != nil {
		return err
	}

	// Bridge os.Stdin → remote stdin in a goroutine so the user can type
	// after the automated rules have fired.
	go func() { _, _ = io.Copy(stdinPipe, os.Stdin) }()

	return sess.Wait()
}

// Run executes cmd non-interactively and returns combined output + exit code.
func (r *sshRunner) Run(_ context.Context, cmd string) (string, int, error) {
	out, err := r.client.Run(cmd)
	if err != nil {
		if exitErr, ok := err.(*gossh.ExitError); ok {
			return string(out), exitErr.ExitStatus(), nil
		}
		return string(out), -1, err
	}
	return string(out), 0, nil
}

// RunPTY opens a new SSH session with a PTY, starts cmd, and applies
// Expect-Send rules by scanning stdout for each rule's Pattern.
func (r *sshRunner) RunPTY(_ context.Context, cmd string, rules []plugin.PTYRule) (string, error) {
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
		return "", err
	}

	stdinPipe, err := sess.StdinPipe()
	if err != nil {
		return "", err
	}

	ew := &pluginExpectWriter{
		rules: rules,
		used:  make([]bool, len(rules)),
		stdin: stdinPipe,
	}
	sess.Stdout = ew
	sess.Stderr = ew

	if err := sess.Start(cmd); err != nil {
		return "", err
	}
	_ = sess.Wait()

	return ew.buf.String(), nil
}

// ── interactiveExpectWriter ───────────────────────────────────────────────────

// interactiveExpectWriter writes remote PTY output to os.Stdout and fires
// Expect-Send rules once. It does not bridge stdin — that is handled separately
// via io.Copy(stdinPipe, os.Stdin) so both automation and user input coexist.
type interactiveExpectWriter struct {
	rules     []plugin.PTYRule
	used      []bool
	stdin     io.Writer
	realStdin *os.File
	mu        sync.Mutex
}

func (w *interactiveExpectWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	n, err := os.Stdout.Write(p)

	recent := string(p)
	for i, rule := range w.rules {
		if w.used[i] {
			continue
		}
		if strings.Contains(recent, rule.Pattern) {
			send := rule.Send
			if rule.SendNewline {
				send += "\n"
			}
			_, _ = w.stdin.Write([]byte(send))
			w.used[i] = true
		}
	}
	return n, err
}

// ── pluginExpectWriter (non-interactive RunPTY) ───────────────────────────────

type pluginExpectWriter struct {
	buf   bytes.Buffer
	rules []plugin.PTYRule
	used  []bool
	stdin interface{ Write([]byte) (int, error) }
	mu    sync.Mutex
}

func (w *pluginExpectWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buf.Write(p)
	recent := w.buf.String()
	for i, rule := range w.rules {
		if w.used[i] {
			continue
		}
		if strings.Contains(recent, rule.Pattern) {
			send := rule.Send
			if rule.SendNewline {
				send += "\n"
			}
			_, _ = w.stdin.Write([]byte(send))
			w.used[i] = true
		}
	}
	return n, err
}
