package ssh

import (
	"context"
	"fmt"
	"time"

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

	// Close the SSH channel when the deadline fires, causing CombinedOutput to
	// unblock and the remote process to receive SIGHUP.
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

// ManagedSession is kept as a thin wrapper so callers that need to reuse a
// single SSH *connection* (for cwd/env persistence via explicit `cd && cmd`
// chaining) can do so without reopening the TCP+SSH handshake each time.
// Each Exec call opens a new SSH *session* (channel) on the same connection.
type ManagedSession struct {
	client *Client
}

// NewManagedSession returns a ManagedSession bound to client.
// loginShell is accepted for API compatibility but ignored — each Exec spawns
// `bash -c` directly, so profile sourcing is controlled by the command itself.
func NewManagedSession(client *Client, _ bool) (*ManagedSession, error) {
	return &ManagedSession{client: client}, nil
}

// Exec runs cmd on the connection, enforcing timeout via the remote `timeout`
// utility. ctx is used only as a parent for the safety deadline.
func (ms *ManagedSession) Exec(ctx context.Context, cmd string, timeout time.Duration) (ExecResult, error) {
	return ExecOnSession(ctx, ms.client, cmd, timeout)
}

// Close is a no-op; the underlying client is closed by the caller (ChainedClient).
func (ms *ManagedSession) Close() error { return nil }
