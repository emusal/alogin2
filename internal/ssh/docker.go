package ssh

import (
	"fmt"
	"os"
	"os/exec"
)

// DockerExec runs an interactive shell in a Docker container.
// containerID may be a container name or ID.
func DockerExec(containerID, user string) error {
	args := []string{"exec", "-it"}
	if user != "" {
		args = append(args, "-u", user)
	}
	args = append(args, containerID, "/bin/sh", "-c",
		"if command -v bash >/dev/null 2>&1; then exec bash; else exec sh; fi")

	cmd := exec.Command("docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker exec %s: %w", containerID, err)
	}
	return nil
}

// VagrantSSH runs an interactive ssh session into a Vagrant box.
// It changes to the directory containing the Vagrantfile.
func VagrantSSH(vagrantDir string) error {
	cmd := exec.Command("vagrant", "ssh")
	if vagrantDir != "" {
		cmd.Dir = vagrantDir
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("vagrant ssh: %w", err)
	}
	return nil
}
