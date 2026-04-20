//go:build !windows

package cli

import (
	"fmt"
	"os/exec"
	"strings"
)

const sessionTmuxPrefix = "alogin-session-"
const jobTmuxPrefix = "alogin-job-"

func sessionTmuxName(id string) string { return sessionTmuxPrefix + id }
func jobTmuxName(id string) string     { return jobTmuxPrefix + id }

// sessionIsRunning reports whether the session bridge process is alive.
func sessionIsRunning(id string) bool {
	return exec.Command("tmux", "has-session", "-t", sessionTmuxName(id)).Run() == nil
}

// sessionSpawn starts the session bridge as a detached tmux session.
func sessionSpawn(id, binPath, hostArg string) error {
	c := exec.Command("tmux", "new-session", "-d", "-s", sessionTmuxName(id),
		binPath, "ssh", "session", "run", id, hostArg)
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("start tmux session: %s", out)
	}
	return nil
}

// sessionKill terminates the session bridge tmux session.
func sessionKill(id string) error {
	return exec.Command("tmux", "kill-session", "-t", sessionTmuxName(id)).Run()
}

// sessionListIDs returns all active session IDs managed by alogin.
func sessionListIDs() ([]string, error) {
	out, err := exec.Command("tmux", "ls", "-F", "#{session_name}").Output()
	if err != nil {
		return nil, nil // no sessions
	}
	var ids []string
	for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.HasPrefix(name, sessionTmuxPrefix) {
			ids = append(ids, strings.TrimPrefix(name, sessionTmuxPrefix))
		}
	}
	return ids, nil
}

// jobSpawn starts a background job as a detached tmux session.
func jobSpawn(jobID, binPath string, timeoutSec int) (string, error) {
	tmuxArgs := []string{
		"new-session", "-d", "-s", jobTmuxName(jobID),
		binPath, "ssh", "session", "job-run", jobID,
		fmt.Sprintf("--timeout=%d", timeoutSec),
	}
	out, err := exec.Command("tmux", tmuxArgs...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("spawn tmux: %s", out)
	}
	return "", nil
}

// jobKill terminates the job tmux session.
func jobKill(jobID string) {
	_ = exec.Command("tmux", "kill-session", "-t", jobTmuxName(jobID)).Run()
}
