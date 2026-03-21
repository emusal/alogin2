//go:build darwin

package vault

import (
	"fmt"
	"os/exec"
	"strings"
)

// KeychainVault stores secrets in macOS Keychain using the `security` CLI.
// This avoids CGO dependencies while providing full Keychain integration.
type KeychainVault struct{}

// NewKeychain creates a KeychainVault.
func NewKeychain() *KeychainVault { return &KeychainVault{} }

func (v *KeychainVault) Name() string { return "keychain" }

func (v *KeychainVault) Get(account string) (string, error) {
	out, err := exec.Command("security",
		"find-generic-password",
		"-s", ServiceName,
		"-a", account,
		"-w", // print password only
	).Output()
	if err != nil {
		return "", ErrNotFound
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func (v *KeychainVault) Set(account, password string) error {
	// Delete first (update-or-create)
	_ = v.Delete(account)

	err := exec.Command("security",
		"add-generic-password",
		"-s", ServiceName,
		"-a", account,
		"-w", password,
	).Run()
	if err != nil {
		return fmt.Errorf("keychain set: %w", err)
	}
	return nil
}

func (v *KeychainVault) Delete(account string) error {
	err := exec.Command("security",
		"delete-generic-password",
		"-s", ServiceName,
		"-a", account,
	).Run()
	// Ignore "not found" exit codes
	if err != nil {
		if strings.Contains(err.Error(), "exit status") {
			return nil
		}
		return err
	}
	return nil
}
