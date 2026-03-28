package plugin

import "context"

// RemoteRunner abstracts the ability to execute commands on a remote host.
// This interface is the coupling boundary between internal/plugin and the SSH
// package. Callers (internal/cli, internal/mcp) implement thin adapters that
// wrap *ssh.Client without creating a circular import.
type RemoteRunner interface {
	// Run executes a command non-interactively and returns combined output,
	// exit code, and any transport-level error.
	Run(ctx context.Context, cmd string) (output string, exitCode int, err error)

	// RunPTY executes a command in a PTY session with Expect-Send automation.
	// Each PTYRule whose Pattern is found in the output triggers the corresponding
	// Send to be written to stdin. Returns captured output.
	RunPTY(ctx context.Context, cmd string, rules []PTYRule) (string, error)

	// RunInteractive executes a command with a PTY attached to the current
	// terminal's stdin/stdout/stderr. Used for interactive sessions (no --cmd).
	// rules are applied first (Expect-Send automation), then stdin is handed to
	// the user for the rest of the session.
	// env contains additional environment variables to set on the remote session.
	RunInteractive(ctx context.Context, cmd string, rules []PTYRule, env map[string]string) error
}

// PTYRule is one Expect-Send pair for PTY-based automation.
type PTYRule struct {
	Pattern        string // substring to match in PTY output
	Send           string // already-expanded value to write to stdin
	SendNewline    bool   // append \n after Send
	EchoSuppressed bool   // when true, caller should suppress terminal echo while sending
}
