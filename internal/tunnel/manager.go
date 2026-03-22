// Package tunnel manages persistent SSH port-forward tunnels via tmux sessions.
package tunnel

import (
	"fmt"
	"os"
	"os/exec"
)

// SessionName returns the tmux session name for the given tunnel name.
func SessionName(name string) string {
	return "alogin-tunnel-" + name
}

// IsRunning reports whether a tmux session for the named tunnel exists.
func IsRunning(name string) bool {
	return exec.Command("tmux", "has-session", "-t", SessionName(name)).Run() == nil
}

// Start spawns `{binPath} tunnel run {name}` in a detached tmux session.
// binPath should be the path to the current alogin binary (os.Executable()).
func Start(name, binPath string) error {
	if IsRunning(name) {
		return fmt.Errorf("tunnel %q is already running", name)
	}
	sess := SessionName(name)
	cmd := exec.Command("tmux", "new-session", "-d", "-s", sess, binPath, "tunnel", "run", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("start tunnel %q: %w", name, err)
	}
	return nil
}

// Stop kills the tmux session for the named tunnel.
func Stop(name string) error {
	if !IsRunning(name) {
		return fmt.Errorf("tunnel %q is not running", name)
	}
	if err := exec.Command("tmux", "kill-session", "-t", SessionName(name)).Run(); err != nil {
		return fmt.Errorf("stop tunnel %q: %w", name, err)
	}
	return nil
}
