package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	gossh "golang.org/x/crypto/ssh"
)

// ExecResult is the output of a single command run via ExecOnSession.
type ExecResult struct {
	Output   string
	ExitCode int
}

// ExecOnSession runs cmd on client in a fresh SSH session. Timeout is enforced
// Go-side: a context deadline closes the SSH channel, which sends SIGHUP to the
// remote process. Each call is fully independent — no shared bash process.
func ExecOnSession(ctx context.Context, client *Client, cmd string, timeout time.Duration) (ExecResult, error) {
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	sess, err := client.inner.NewSession()
	if err != nil {
		return ExecResult{}, fmt.Errorf("new session: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	go func() {
		<-ctx.Done()
		_ = sess.Close()
	}()

	out, err := sess.CombinedOutput(cmd)

	const maxBytes = 64 * 1024
	if len(out) > maxBytes {
		out = append(out[:maxBytes], []byte("\n[output truncated]")...)
	}

	if err != nil {
		if ctx.Err() != nil {
			return ExecResult{Output: string(out), ExitCode: 130}, nil
		}
		if exitErr, ok := err.(*gossh.ExitError); ok {
			return ExecResult{Output: string(out), ExitCode: exitErr.ExitStatus()}, nil
		}
		return ExecResult{Output: string(out)}, fmt.Errorf("exec: %w", err)
	}
	return ExecResult{Output: string(out), ExitCode: 0}, nil
}

// ManagedSession holds a single persistent bash process on the remote host.
// cwd, environment variables, and shell state are preserved across Exec calls
// because all commands run inside the same bash process via its stdin pipe.
//
// Protocol: each Exec wraps the user command as:
//
//	{ user command ; } ; printf '\n%s %d\n' <sentinel> $?
//
// Exec reads lines until it sees a line beginning with the sentinel UUID,
// then parses the exit code from the same line. The sentinel is a per-session
// UUID so it cannot appear in normal command output.
type ManagedSession struct {
	sess     *gossh.Session
	stdin    io.WriteCloser
	outBuf   *lineBuffer
	sentinel string // unique per-session, never appears in normal output
}

// NewManagedSession opens a single SSH session and starts a persistent bash
// process inside it. loginShell controls whether bash is started with -l.
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

	lb := newLineBuffer()
	sess.Stdout = lb
	sess.Stderr = lb

	bashCmd := "bash --norc --noprofile"
	if loginShell {
		bashCmd = "bash -l"
	}
	if err := sess.Start(bashCmd); err != nil {
		stdin.Close()
		sess.Close()
		return nil, fmt.Errorf("start bash: %w", err)
	}

	ms := &ManagedSession{
		sess:     sess,
		stdin:    stdin,
		outBuf:   lb,
		sentinel: uuid.New().String(),
	}

	// Suppress PS1 so prompt text doesn't contaminate command output.
	// Drain any startup noise (profile output) with a sentinel probe.
	initCmd := fmt.Sprintf("PS1=''; PS2=''; export PS1 PS2; printf '\\n%s 0\\n'\n", ms.sentinel)
	if _, err := io.WriteString(stdin, initCmd); err != nil {
		stdin.Close()
		sess.Close()
		return nil, fmt.Errorf("init bash: %w", err)
	}
	// Wait for the init sentinel — confirms bash is ready and profile is done.
	if err := ms.drainUntilSentinel(10 * time.Second); err != nil {
		stdin.Close()
		sess.Close()
		return nil, fmt.Errorf("bash init timeout: %w", err)
	}

	return ms, nil
}

// drainUntilSentinel discards all output until a sentinel line appears.
// Used during initialisation to flush bash startup/profile output.
func (ms *ManagedSession) drainUntilSentinel(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		line, ok := ms.outBuf.readLine(100 * time.Millisecond)
		if !ok {
			continue
		}
		if strings.HasPrefix(line, ms.sentinel) {
			return nil
		}
	}
	return fmt.Errorf("timed out waiting for bash init")
}

// Exec sends cmd to the persistent bash process and waits for its output.
//
// The command is wrapped as:
//
//	{ <cmd> ; } ; printf '\n<sentinel> %d\n' $?
//
// printf is used instead of echo to guarantee a newline before the sentinel
// even when the command output does not end with one. Exec reads until the
// sentinel line appears, then returns the collected output and exit code.
func (ms *ManagedSession) Exec(ctx context.Context, cmd string, timeout time.Duration) (ExecResult, error) {
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	// Wrap in a group so that the sentinel is always printed after the command
	// completes, regardless of how the command exits.
	// We print a literal newline before the sentinel so that commands whose last
	// line has no trailing newline don't run the sentinel onto the same line.
	wrapped := fmt.Sprintf("{ %s\n} ; printf '\\n%s %%d\\n' $?\n", cmd, ms.sentinel)
	if _, err := io.WriteString(ms.stdin, wrapped); err != nil {
		return ExecResult{}, fmt.Errorf("write command: %w", err)
	}

	deadline := time.Now().Add(timeout)
	var buf bytes.Buffer
	exitCode := 0
	const maxBytes = 64 * 1024
	// firstLine skips the leading empty line that printf '\n...' produces.
	firstLineSkipped := false

	for {
		if time.Now().After(deadline) {
			_, _ = io.WriteString(ms.stdin, "\x03") // Ctrl-C
			time.Sleep(100 * time.Millisecond)
			ms.outBuf.drain()
			return ExecResult{Output: strings.TrimRight(buf.String(), "\n"), ExitCode: 130}, nil
		}

		select {
		case <-ctx.Done():
			_, _ = io.WriteString(ms.stdin, "\x03")
			time.Sleep(100 * time.Millisecond)
			ms.outBuf.drain()
			return ExecResult{Output: strings.TrimRight(buf.String(), "\n"), ExitCode: 130}, nil
		default:
		}

		line, ok := ms.outBuf.readLine(50 * time.Millisecond)
		if !ok {
			continue
		}

		// Sentinel line: "<uuid> <exitcode>"
		if strings.HasPrefix(line, ms.sentinel) {
			rest := strings.TrimPrefix(line, ms.sentinel)
			rest = strings.TrimSpace(rest)
			fmt.Sscanf(rest, "%d", &exitCode)
			break
		}

		// Skip the leading blank line injected by printf '\n...'
		if !firstLineSkipped && line == "" {
			firstLineSkipped = true
			continue
		}

		if buf.Len() < maxBytes {
			remaining := maxBytes - buf.Len()
			chunk := line + "\n"
			if len(chunk) > remaining {
				buf.WriteString(line[:remaining])
				buf.WriteString("\n[output truncated]")
			} else {
				buf.WriteString(chunk)
			}
		}
	}

	return ExecResult{Output: strings.TrimRight(buf.String(), "\n"), ExitCode: exitCode}, nil
}

// Close terminates the bash process and the underlying SSH session.
func (ms *ManagedSession) Close() error {
	_, _ = io.WriteString(ms.stdin, "exit\n")
	time.Sleep(50 * time.Millisecond)
	_ = ms.stdin.Close()
	return ms.sess.Close()
}

// lineBuffer is a concurrent, line-oriented buffer that bridges the bash
// stdout/stderr goroutine with the Exec polling loop.
type lineBuffer struct {
	ch chan string
}

func newLineBuffer() *lineBuffer {
	return &lineBuffer{ch: make(chan string, 4096)}
}

// Write implements io.Writer. It splits p on newlines and queues each line.
// Partial lines (no trailing newline) are queued as-is so no output is lost.
func (lb *lineBuffer) Write(p []byte) (int, error) {
	text := string(p)
	for {
		idx := strings.IndexByte(text, '\n')
		if idx < 0 {
			if text != "" {
				lb.enqueue(text)
			}
			break
		}
		line := strings.TrimRight(text[:idx], "\r")
		lb.enqueue(line)
		text = text[idx+1:]
	}
	return len(p), nil
}

func (lb *lineBuffer) enqueue(line string) {
	select {
	case lb.ch <- line:
	default:
		// Channel full: drop oldest to make room.
		select {
		case <-lb.ch:
		default:
		}
		lb.ch <- line
	}
}

// readLine returns the next line, waiting up to d. Returns ("", false) on timeout.
func (lb *lineBuffer) readLine(d time.Duration) (string, bool) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case line := <-lb.ch:
		return line, true
	case <-t.C:
		return "", false
	}
}

// drain discards all buffered lines.
func (lb *lineBuffer) drain() {
	for {
		select {
		case <-lb.ch:
		default:
			return
		}
	}
}
