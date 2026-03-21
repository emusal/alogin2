package cluster

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// openTmux opens all hosts in a new tmux session with tiled panes.
// synchronize-panes is enabled only after a short delay so that each pane's
// alogin process can complete password injection before keystrokes are broadcast.
func openTmux(ctx context.Context, name string, hosts []HostEntry, tileX int, binPath string) error {
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux not found; install tmux or use --mode terminal/iterm")
	}

	sessionName := "alogin-" + name

	// Create new detached session with the first host
	firstCmd := buildConnCmd(binPath, hosts[0])
	if err := tmuxRun("new-session", "-d", "-s", sessionName, "-x", "220", "-y", "50", firstCmd); err != nil {
		// Session may already exist
		_ = err
	}

	// Add remaining hosts as split panes
	for i := 1; i < len(hosts); i++ {
		cmd := buildConnCmd(binPath, hosts[i])
		if err := tmuxRun("split-window", "-t", sessionName, cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: split-window for %s: %v\n", hosts[i].Host, err)
		}
		_ = tmuxRun("select-layout", "-t", sessionName, "tiled")
	}

	_ = tmuxRun("select-layout", "-t", sessionName, "tiled")

	// Enable synchronize-panes after a delay so all sessions finish connecting
	// (and password injection completes) before keystrokes are broadcast.
	// Using tmux run-shell lets the delay happen in the background while we attach.
	delay := syncDelay(len(hosts))
	_ = tmuxRun("run-shell", "-t", sessionName,
		fmt.Sprintf("sleep %d && tmux set-window-option -t %s synchronize-panes on", delay, sessionName))

	// Attach to session
	cmd := exec.CommandContext(ctx, "tmux", "attach-session", "-t", sessionName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// syncDelay returns seconds to wait before enabling synchronize-panes.
// Give more time for larger clusters.
func syncDelay(hostCount int) int {
	if hostCount <= 4 {
		return 5
	}
	if hostCount <= 10 {
		return 8
	}
	return 12
}

func buildSSHCmd(h HostEntry) string {
	args := []string{"ssh"}

	if len(h.Hops) > 0 {
		var jumps string
		for i, hop := range h.Hops {
			if i > 0 {
				jumps += ","
			}
			jumps += fmt.Sprintf("%s@%s:%d", hop.User, hop.Host, hop.Port)
		}
		args = append(args, "-J", jumps)
	}

	args = append(args, "-p", strconv.Itoa(h.Port))
	args = append(args, fmt.Sprintf("%s@%s", h.User, h.Host))

	result := ""
	for i, a := range args {
		if i > 0 {
			result += " "
		}
		result += a
	}
	return result
}

func tmuxRun(args ...string) error {
	cmd := exec.Command("tmux", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
