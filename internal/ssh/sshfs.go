package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Mount mounts a remote path via SSHFS.
// Requires sshfs to be installed (macOS: brew install macfuse sshfs, Linux: apt install sshfs).
func Mount(hopCfg HopConfig, remotePath, localPath string) error {
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("mkdir mountpoint %s: %w", localPath, err)
	}

	sshfsCmd, err := findSSHFS()
	if err != nil {
		return err
	}

	remote := fmt.Sprintf("%s@%s:%s", hopCfg.User, hopCfg.Host, remotePath)
	args := []string{
		remote,
		localPath,
		"-p", fmt.Sprintf("%d", hopCfg.Port),
		"-o", "reconnect",
		"-o", "ServerAliveInterval=15",
	}

	if hopCfg.Password != "" {
		args = append(args, "-o", "password_stdin")
	}
	if hopCfg.KeyPath != "" {
		args = append(args, "-o", "IdentityFile="+hopCfg.KeyPath)
	}

	cmd := exec.Command(sshfsCmd, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if hopCfg.Password != "" {
		cmd.Stdin = newPasswordReader(hopCfg.Password)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sshfs %s → %s: %w", remote, localPath, err)
	}

	absPath, _ := filepath.Abs(localPath)
	fmt.Printf("Mounted %s at %s\n", remote, absPath)
	return nil
}

// Unmount unmounts an SSHFS mount point.
func Unmount(localPath string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("diskutil", "unmount", localPath)
	default:
		cmd = exec.Command("fusermount", "-u", localPath)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findSSHFS() (string, error) {
	paths := []string{"sshfs"}
	if runtime.GOOS == "darwin" {
		paths = append(paths,
			"/usr/local/bin/sshfs",
			"/opt/homebrew/bin/sshfs",
		)
	}
	for _, p := range paths {
		if path, err := exec.LookPath(p); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("sshfs not found; install via: brew install macfuse sshfs (macOS) or apt install sshfs (Linux)")
}

type passwordReader struct {
	data []byte
	pos  int
}

func newPasswordReader(pwd string) *passwordReader {
	return &passwordReader{data: []byte(pwd + "\n")}
}

func (r *passwordReader) Read(p []byte) (int, error) {
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
