package ssh

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

const managedSentinelPrefix = "__ALOGIN_EXIT_"

// ManagedSession holds a persistent bash process on a remote server.
// All commands run in the same bash process, so cwd, env variables, and
// shell state persist across Exec calls.
type ManagedSession struct {
	sshSess *gossh.Session
	stdin   io.WriteCloser
	stdout  *bufio.Reader
}

// ExecResult is the output of a single command run via ManagedSession.Exec.
type ExecResult struct {
	Output   string
	ExitCode int
}

// NewManagedSession starts a persistent bash process on the remote server
// attached to client and returns a ready ManagedSession.
//
// By default (loginShell=true) bash is invoked as "bash -l" so ~/.bash_profile
// and ~/.profile are sourced — PATH, nvm, pyenv, rbenv, conda, etc. are all
// available without extra setup. This matches what a human gets when logging in
// via SSH, which is the correct baseline for agent-driven automation.
//
// Pass loginShell=false only when a pristine environment is required (e.g.
// CI-style scripts that must not be affected by user profile customisations).
// In that mode bash runs with --norc --noprofile; banner/profile output is
// suppressed so it cannot interfere with sentinel detection.
func NewManagedSession(client *Client, loginShell bool) (*ManagedSession, error) {
	sess, err := client.inner.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new ssh session: %w", err)
	}

	stdin, err := sess.StdinPipe()
	if err != nil {
		sess.Close()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdoutPipe, err := sess.StdoutPipe()
	if err != nil {
		stdin.Close()
		sess.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	bashCmd := "bash -l 2>/dev/null"
	if !loginShell {
		// Pristine mode: no profile sourcing. stderr merged into stdout so the
		// sentinel parser sees a single ordered stream.
		bashCmd = "bash --norc --noprofile 2>&1"
	}
	if err := sess.Start(bashCmd); err != nil {
		stdin.Close()
		sess.Close()
		return nil, fmt.Errorf("start bash: %w", err)
	}

	ms := &ManagedSession{
		sshSess: sess,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdoutPipe),
	}

	if loginShell { //nolint:gocritic // loginShell=true is the new default; flush is always needed
		// Flush banner/profile output that arrived before the first command.
		// Emit a sentinel immediately and drain stdout until it appears,
		// discarding everything that came from ~/.bash_profile startup.
		flushPayload := `printf '__ALOGIN_EXIT_0\n'` + "\n"
		if _, err := io.WriteString(stdin, flushPayload); err != nil {
			_ = sess.Close()
			return nil, fmt.Errorf("flush banner: %w", err)
		}
		for {
			line, err := ms.stdout.ReadString('\n')
			if err != nil {
				_ = sess.Close()
				return nil, fmt.Errorf("flush banner read: %w", err)
			}
			if strings.HasPrefix(strings.TrimRight(line, "\r\n"), managedSentinelPrefix) {
				break
			}
		}
	}

	return ms, nil
}

// Exec runs cmd inside the persistent bash process and returns its output and
// exit code. If timeout is 0 or negative the default (120 s) is used.
//
// Timeout is activity-based (idle): the timer resets each time a new line
// arrives on stdout. This prevents spurious timeouts for commands that produce
// steady output (e.g. large apt-get upgrades, long builds). A hard ceiling of
// 10× the idle timeout is enforced so a completely silent command cannot run
// forever.
//
// On timeout, SIGINT (Ctrl-C) is sent to interrupt the running command and
// exit code 130 is returned. The session remains usable after a timeout.
func (ms *ManagedSession) Exec(ctx context.Context, cmd string, timeout time.Duration) (ExecResult, error) {
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	// Hard ceiling: cap at 10× the idle timeout so silent commands don't hang.
	maxTimeout := timeout * 10
	ctx, cancel := context.WithTimeout(ctx, maxTimeout)
	defer cancel()

	// Write the command followed by the sentinel emitter.
	// printf guarantees the sentinel is always a bare integer — no risk of
	// collision with command output that happens to start with the prefix.
	// The { ... } 2>&1 wrapper ensures stderr from the user's command is merged
	// into the stdout pipe that we read, regardless of how the bash process
	// itself was started (--norc or -l). Without this, stderr would be silently
	// lost on login-shell sessions (where bash starts with 2>/dev/null).
	payload := "{ " + cmd + "; } 2>&1\n" + `printf '__ALOGIN_EXIT_%d\n' $?` + "\n"
	if _, err := io.WriteString(ms.stdin, payload); err != nil {
		return ExecResult{}, fmt.Errorf("write command: %w", err)
	}

	type readResult struct {
		output   string
		exitCode int
		err      error
	}
	ch := make(chan readResult, 1)

	// resetC carries idle-timer resets from the reader goroutine to the
	// select loop below. Buffered so the goroutine never blocks on a send.
	resetC := make(chan struct{}, 1)

	go func() {
		var sb strings.Builder
		for {
			line, err := ms.stdout.ReadString('\n')
			if err != nil {
				ch <- readResult{err: fmt.Errorf("read stdout: %w", err)}
				return
			}
			// Signal activity so the idle timer resets.
			select {
			case resetC <- struct{}{}:
			default:
			}
			trimmed := strings.TrimRight(line, "\r\n")
			if strings.HasPrefix(trimmed, managedSentinelPrefix) {
				code, parseErr := strconv.Atoi(strings.TrimPrefix(trimmed, managedSentinelPrefix))
				if parseErr != nil {
					code = -1
				}
				out := sb.String()
				const maxBytes = 64 * 1024
				if len(out) > maxBytes {
					out = out[:maxBytes] + "\n[output truncated]"
				}
				ch <- readResult{output: out, exitCode: code}
				return
			}
			sb.WriteString(line)
		}
	}()

	idleTimer := time.NewTimer(timeout)
	defer idleTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			// Hard ceiling hit or parent context cancelled.
			_, _ = io.WriteString(ms.stdin, "\x03\n"+`printf '__ALOGIN_EXIT_130\n'`+"\n")
			return ExecResult{ExitCode: 130, Output: "[timeout: command interrupted]"}, ctx.Err()
		case <-idleTimer.C:
			// No stdout activity for `timeout` duration.
			_, _ = io.WriteString(ms.stdin, "\x03\n"+`printf '__ALOGIN_EXIT_130\n'`+"\n")
			return ExecResult{ExitCode: 130, Output: "[idle timeout: no output for " + timeout.String() + "]"}, context.DeadlineExceeded
		case <-resetC:
			// stdout activity — reset the idle timer.
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(timeout)
		case r := <-ch:
			if r.err != nil {
				return ExecResult{}, r.err
			}
			return ExecResult{Output: r.output, ExitCode: r.exitCode}, nil
		}
	}
}

// Close terminates the bash process and closes the underlying SSH session.
func (ms *ManagedSession) Close() error {
	_, _ = io.WriteString(ms.stdin, "exit\n")
	_ = ms.stdin.Close()
	_ = ms.sshSess.Wait()
	return ms.sshSess.Close()
}
