//go:build linux

package vault

import (
	"fmt"
	"os/exec"
	"strings"
)

// LibsecretVault stores secrets in the Linux Secret Service (GNOME Keyring / KWallet)
// using the `secret-tool` CLI (part of libsecret-tools).
type LibsecretVault struct{}

// NewLibsecret creates a LibsecretVault.
// Returns nil if secret-tool is not available.
func NewLibsecret() *LibsecretVault {
	if _, err := exec.LookPath("secret-tool"); err != nil {
		return nil
	}
	return &LibsecretVault{}
}

func (v *LibsecretVault) Name() string { return "libsecret" }

func (v *LibsecretVault) Get(account string) (string, error) {
	out, err := exec.Command("secret-tool", "lookup",
		"service", ServiceName,
		"account", account,
	).Output()
	if err != nil {
		return "", ErrNotFound
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func (v *LibsecretVault) Set(account, password string) error {
	cmd := exec.Command("secret-tool", "store",
		"--label", fmt.Sprintf("alogin: %s", account),
		"service", ServiceName,
		"account", account,
	)
	cmd.Stdin = strings.NewReader(password)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("secret-tool store: %w", err)
	}
	return nil
}

func (v *LibsecretVault) Delete(account string) error {
	err := exec.Command("secret-tool", "clear",
		"service", ServiceName,
		"account", account,
	).Run()
	if err != nil {
		return fmt.Errorf("secret-tool clear: %w", err)
	}
	return nil
}
