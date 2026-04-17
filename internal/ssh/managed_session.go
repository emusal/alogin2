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
// bash is started with --norc --noprofile to suppress banner output that
// could interfere with sentinel detection. stderr is merged into stdout at
// the shell level (2>&1) to preserve write ordering across the single pipe.
//
// When loginShell is true, bash is invoked as "bash -l" instead, which loads
// ~/.bash_profile and ~/.profile so PATH, nvm, pyenv, etc. are available.
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

	bashCmd := "bash --norc --noprofile 2>&1"
	if loginShell {
		bashCmd = "bash -l 2>&1"
	}
	if err := sess.Start(bashCmd); err != nil {
		stdin.Close()
		sess.Close()
		return nil, fmt.Errorf("start bash: %w", err)
	}

	return &ManagedSession{
		sshSess: sess,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdoutPipe),
	}, nil
}

// Exec runs cmd inside the persistent bash process and returns its output and
// exit code. If timeout is 0 or negative the default (30 s) is used.
//
// On timeout, SIGINT (Ctrl-C) is sent to interrupt the running command and
// exit code 130 is returned. The session remains usable after a timeout.
func (ms *ManagedSession) Exec(ctx context.Context, cmd string, timeout time.Duration) (ExecResult, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Write the command followed by the sentinel emitter.
	// printf guarantees the sentinel is always a bare integer — no risk of
	// collision with command output that happens to start with the prefix.
	payload := cmd + "\n" + `printf '__ALOGIN_EXIT_%d\n' $?` + "\n"
	if _, err := io.WriteString(ms.stdin, payload); err != nil {
		return ExecResult{}, fmt.Errorf("write command: %w", err)
	}

	type readResult struct {
		output   string
		exitCode int
		err      error
	}
	ch := make(chan readResult, 1)

	go func() {
		var sb strings.Builder
		for {
			line, err := ms.stdout.ReadString('\n')
			if err != nil {
				ch <- readResult{err: fmt.Errorf("read stdout: %w", err)}
				return
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

	select {
	case <-ctx.Done():
		// Send Ctrl-C to interrupt the running command, then emit a synthetic
		// sentinel so the reader goroutine can drain cleanly on the next call.
		_, _ = io.WriteString(ms.stdin, "\x03\n"+`printf '__ALOGIN_EXIT_130\n'`+"\n")
		return ExecResult{ExitCode: 130, Output: "[timeout: command interrupted]"}, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return ExecResult{}, r.err
		}
		return ExecResult{Output: r.output, ExitCode: r.exitCode}, nil
	}
}

// Close terminates the bash process and closes the underlying SSH session.
func (ms *ManagedSession) Close() error {
	_, _ = io.WriteString(ms.stdin, "exit\n")
	_ = ms.stdin.Close()
	_ = ms.sshSess.Wait()
	return ms.sshSess.Close()
}
