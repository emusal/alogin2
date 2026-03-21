package ssh

// ShellChain implements v1-style multi-hop SSH (identical to conn.exp).
//
// Rather than using direct-tcpip (ProxyJump), which requires AllowTcpForwarding
// on every intermediate host, ShellChain connects to hop[0] via a normal SSH
// session and then runs "ssh -tt user@hop[N]" inside the shell of each hop.
// This works on any server where the user has shell access.
//
// Flow (mirrors v1 conn.exp):
//
//	  hop[0]: Dial → PTY shell → wait for prompt
//	  hop[1]: send "ssh -tt user@hop[1]" in shell → handle auth → wait for prompt
//	  ...
//	  hop[N]: same as above, then hand off to interactive session

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// prompt / auth patterns (mirrors v1 conn.exp expect patterns)
var (
	reShellPrompt = regexp.MustCompile(`[\]$#>%~]\s*$`)
	reShellPass   = regexp.MustCompile(`(?i)(password|암호|parol)[^:\n]*:\s*$`)
	reShellHK     = regexp.MustCompile(`Are you sure you want to continue connecting`)
	reShellDenied = regexp.MustCompile(`(?i)permission denied`)
	reShellRefuse = regexp.MustCompile(`(?i)connection refused`)
	reShellClosed = regexp.MustCompile(`(?i)connection closed by`)
	reShellNoRt   = regexp.MustCompile(`(?i)(no route to host|network is unreachable|host is down|name or service not known|could not resolve)`)

	// reANSI strips terminal escape sequences before pattern matching,
	// so prompts with colour codes (e.g. Oh My Zsh "➜  ~\x1b[0m") are recognised.
	reANSI = regexp.MustCompile(`\x1b(?:\[[0-9;?]*[a-zA-Z]|\][^\x07\x1b]*(?:\x07|\x1b\\)|[()][A-B0-2]|[=>MNOPQRSTUVWXYZ\\^_c])`)
)

// ShellChain opens a multi-hop SSH session using shell-level chaining.
// For a single hop it falls back to the regular Dial+Shell path.
// It blocks until the remote session exits.
func ShellChain(hops []HopConfig, opts ShellOptions) error {
	if len(hops) == 0 {
		return fmt.Errorf("no hops provided")
	}

	// Single hop: use the regular path (supports tunnels, etc.)
	if len(hops) == 1 {
		cl, err := Dial(hops[0])
		if err != nil {
			return err
		}
		defer cl.Close()
		return cl.Shell(opts)
	}

	// ── connect to the first hop ─────────────────────────────────────────────
	first, err := Dial(hops[0])
	if err != nil {
		return err
	}
	defer first.Close()

	sess, err := first.inner.NewSession()
	if err != nil {
		return fmt.Errorf("session on %s: %w", hops[0].Host, err)
	}
	defer sess.Close()

	// Request PTY on hop[0]
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
		gossh.ECHO:          1,
		gossh.TTY_OP_ISPEED: 14400,
		gossh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		return fmt.Errorf("pty on %s: %w", hops[0].Host, err)
	}

	stdinW, err := sess.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	defer stdinW.Close()

	stdoutR, err := sess.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := sess.Shell(); err != nil {
		return fmt.Errorf("shell on %s: %w", hops[0].Host, err)
	}

	// ── raw terminal ─────────────────────────────────────────────────────────
	oldState, rawErr := term.MakeRaw(fd)
	if rawErr == nil {
		defer term.Restore(fd, oldState)
	}

	out := opts.Stdout
	if out == nil {
		out = os.Stdout
	}

	// Background reader: copies every byte to out AND feeds a channel for
	// pattern matching during the hop-setup phase.
	cr := startShellReader(stdoutR, out)

	// ── wait for initial shell prompt on hop[0] ───────────────────────────────
	var acc bytes.Buffer
	if err := shellWaitPrompt(cr, &acc, 30*time.Second); err != nil {
		return fmt.Errorf("%s: waiting for shell: %w", hops[0].Host, err)
	}

	// ── chain through hops[1..N] via shell commands ───────────────────────────
	for i := 1; i < len(hops); i++ {
		hop := hops[i]
		// Build ssh command — identical to v1 conn.exp for non-first hops:
		//   send "$proto $user@$host -p $port\r"
		// -tt forces a PTY on the next hop; StrictHostKeyChecking=no mirrors
		// DialVia's InsecureIgnoreHostKey (inner hop hostnames are relative to
		// the proxy's network, so local known_hosts is meaningless).
		cmd := fmt.Sprintf("ssh -tt -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -p %d %s@%s\r",
			hop.Port, hop.User, hop.Host)
		if _, err := fmt.Fprint(stdinW, cmd); err != nil {
			return fmt.Errorf("hop %d send cmd: %w", i, err)
		}
		if err := shellConductHop(cr, &acc, hop.Password, 30*time.Second, stdinW); err != nil {
			return fmt.Errorf("hop %d (%s): %w", i, hop.Host, err)
		}
	}

	// ── apply locale env on the final hop ────────────────────────────────────
	for k, v := range opts.Env {
		fmt.Fprintf(stdinW, "export %s=%s\r", k, v)
	}

	// ── run a startup command if requested ───────────────────────────────────
	if opts.Command != "" {
		fmt.Fprintf(stdinW, "%s; exit\r", opts.Command)
	}

	// ── SIGWINCH forwarding ───────────────────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	defer signal.Stop(sigCh)
	go func() {
		for range sigCh {
			nw, nh, _ := term.GetSize(fd)
			_ = sess.WindowChange(nh, nw)
		}
	}()

	// ── interactive: forward local stdin → remote ─────────────────────────────
	stdinR := opts.Stdin
	if stdinR == nil {
		stdinR = os.Stdin
	}
	go io.Copy(stdinW, stdinR)

	return sess.Wait()
}

// shellWaitPrompt waits until the accumulated output ends with a shell prompt.
func shellWaitPrompt(cr *shellChunkReader, acc *bytes.Buffer, timeout time.Duration) error {
	_, err := cr.waitAny([]*regexp.Regexp{reShellPrompt}, acc, timeout)
	return err
}

// shellConductHop handles authentication after sending "ssh user@host" in a shell.
// Mirrors v1 conn.exp's expect loop for non-first hops.
func shellConductHop(cr *shellChunkReader, acc *bytes.Buffer, password string, timeout time.Duration, w io.Writer) error {
	patterns := []*regexp.Regexp{
		reShellPrompt, // 0 — reached shell: success
		reShellPass,   // 1 — password prompt
		reShellHK,     // 2 — host key question
		reShellDenied, // 3 — auth failure
		reShellRefuse, // 4 — connection refused
		reShellNoRt,   // 5 — unreachable
		reShellClosed, // 6 — connection closed (e.g. wrong password)
	}
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("timeout")
		}
		idx, err := cr.waitAny(patterns, acc, remaining)
		if err != nil {
			return err
		}
		switch idx {
		case 0:
			return nil // shell prompt — connected
		case 1:
			fmt.Fprintf(w, "%s\r", password)
		case 2:
			fmt.Fprintf(w, "yes\r")
		case 3:
			return fmt.Errorf("authentication failed (permission denied)")
		case 4:
			return fmt.Errorf("connection refused")
		case 5:
			return fmt.Errorf("host unreachable")
		case 6:
			return fmt.Errorf("connection closed by remote (wrong password?)")
		}
	}
}

// ── shellChunkReader ─────────────────────────────────────────────────────────

type shellChunkReader struct {
	ch <-chan []byte
}

// startShellReader launches a goroutine that reads r in chunks, writes each
// chunk to display (so the user sees the session), and sends it to a channel
// for pattern matching. The channel is closed when r returns an error (EOF).
func startShellReader(r io.Reader, display io.Writer) *shellChunkReader {
	ch := make(chan []byte, 64)
	go func() {
		defer close(ch)
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				if display != nil {
					_, _ = display.Write(chunk)
				}
				ch <- chunk
			}
			if err != nil {
				return
			}
		}
	}()
	return &shellChunkReader{ch: ch}
}

// waitAny accumulates data from the reader into acc until one of the patterns
// matches the accumulated buffer, then resets acc and returns the pattern index.
func (cr *shellChunkReader) waitAny(patterns []*regexp.Regexp, acc *bytes.Buffer, timeout time.Duration) (int, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case chunk, ok := <-cr.ch:
			if !ok {
				return -1, io.EOF
			}
			acc.Write(chunk)
			b := reANSI.ReplaceAll(acc.Bytes(), nil)
			for i, p := range patterns {
				if p.Match(b) {
					acc.Reset()
					return i, nil
				}
			}
		case <-timer.C:
			return -1, fmt.Errorf("timeout waiting for response")
		}
	}
}
