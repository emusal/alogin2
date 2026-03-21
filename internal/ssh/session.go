package ssh

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// ShellOptions configures an interactive PTY session.
type ShellOptions struct {
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Env     map[string]string // extra env vars to set (e.g. LC_ALL)
	Command string            // if non-empty, run instead of interactive shell
}

// Shell starts an interactive PTY session on the client.
// It blocks until the remote session exits.
// SIGWINCH is caught and forwarded as SSH window-change requests.
func (c *Client) Shell(opts ShellOptions) error {
	sess, err := c.inner.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()

	// Get current terminal size
	fd := int(os.Stdin.Fd())
	w, h, err := term.GetSize(fd)
	if err != nil {
		w, h = 80, 24
	}

	// Request PTY
	termType := os.Getenv("TERM")
	if termType == "" {
		termType = "xterm-256color"
	}
	if err := sess.RequestPty(termType, h, w, gossh.TerminalModes{
		gossh.ECHO:          1,
		gossh.TTY_OP_ISPEED: 14400,
		gossh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		return fmt.Errorf("request pty: %w", err)
	}

	// Set environment variables
	for k, v := range opts.Env {
		if err := sess.Setenv(k, v); err != nil {
			// Ignore; many servers disallow arbitrary env vars
			_ = err
		}
	}

	stdin := opts.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	sess.Stdin = stdin
	sess.Stdout = stdout
	sess.Stderr = stderr

	// Put local terminal into raw mode
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		oldState, err := term.MakeRaw(int(f.Fd()))
		if err == nil {
			defer term.Restore(int(f.Fd()), oldState)
		}
	}

	// Forward SIGWINCH → SSH window-change
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-sigCh:
				nw, nh, err := term.GetSize(fd)
				if err == nil {
					_ = sess.WindowChange(nh, nw)
				}
			case <-done:
				return
			}
		}
	}()

	// Start shell or command
	var startErr error
	if opts.Command != "" {
		startErr = sess.Start(opts.Command)
	} else {
		startErr = sess.Shell()
	}
	if startErr != nil {
		signal.Stop(sigCh)
		return fmt.Errorf("start session: %w", startErr)
	}

	waitErr := sess.Wait()
	signal.Stop(sigCh)
	return waitErr
}

// ensure pty import used (it provides pty.Open for Web UI PTY in Phase 4)
var _ = pty.Open
