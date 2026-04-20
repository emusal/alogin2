//go:build windows

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// sessionTmuxName / jobTmuxName are kept for compatibility with shared code
// that prints session/job names, but they are not used for process management.
func sessionTmuxName(id string) string { return "alogin-session-" + id }
func jobTmuxName(id string) string     { return "alogin-job-" + id }

func sessionPIDFile(id string) string { return filepath.Join(sessionDir(id), "pid") }
func jobPIDFile(jobID string) string {
	return filepath.Join(cfg.DataDir, "jobs", jobID, "pid")
}

// sessionIsRunning checks whether the session bridge process is alive via its PID file.
func sessionIsRunning(id string) bool {
	return pidFileAlive(sessionPIDFile(id))
}

// sessionSpawn starts the session bridge as a detached background process.
func sessionSpawn(id, binPath, hostArg string) error {
	c := exec.Command(binPath, "ssh", "session", "run", id, hostArg)
	c.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000200 | 0x00000008, // CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS
	}
	if err := c.Start(); err != nil {
		return fmt.Errorf("start session process: %w", err)
	}
	return os.WriteFile(sessionPIDFile(id), []byte(strconv.Itoa(c.Process.Pid)), 0600)
}

// sessionKill terminates the session bridge process.
func sessionKill(id string) error {
	return killPIDFile(sessionPIDFile(id))
}

// sessionListIDs returns all active session IDs by scanning the sessions directory.
func sessionListIDs() ([]string, error) {
	sessionsDir := filepath.Join(cfg.DataDir, "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil, nil
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		if pidFileAlive(sessionPIDFile(id)) {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// jobSpawn starts a background job as a detached background process.
func jobSpawn(jobID, binPath string, timeoutSec int) (string, error) {
	dir := filepath.Dir(jobPIDFile(jobID))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create job dir: %w", err)
	}
	c := exec.Command(binPath, "ssh", "session", "job-run", jobID,
		fmt.Sprintf("--timeout=%d", timeoutSec))
	c.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000200 | 0x00000008, // CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS
	}
	if err := c.Start(); err != nil {
		return "", fmt.Errorf("start job process: %w", err)
	}
	_ = os.WriteFile(jobPIDFile(jobID), []byte(strconv.Itoa(c.Process.Pid)), 0600)
	return "", nil
}

// jobKill terminates the job process.
func jobKill(jobID string) {
	_ = killPIDFile(jobPIDFile(jobID))
}

// pidFileAlive returns true if the PID in the file refers to a running process.
func pidFileAlive(pidFile string) bool {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false
	}
	// tasklist is available on all Windows versions.
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), strconv.Itoa(pid))
}

// killPIDFile reads a PID file and terminates the process.
func killPIDFile(pidFile string) error {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return nil // already gone
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return nil
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}
	_ = p.Kill()
	_ = os.Remove(pidFile)
	return nil
}
