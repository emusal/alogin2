//go:build windows

package vault

import (
	"fmt"
	"os/exec"
	"strings"
)

// CredentialManagerVault stores secrets in Windows Credential Locker
// using PowerShell's Windows.Security.Credentials.PasswordVault API.
// This avoids CGO while providing OS-integrated credential storage.
type CredentialManagerVault struct{}

// NewKeychain returns a CredentialManagerVault for Windows.
// Mirrors the NewKeychain() convention used on macOS and Linux.
func NewKeychain() Vault {
	return &CredentialManagerVault{}
}

func (v *CredentialManagerVault) Name() string { return "windows-credential-manager" }

func (v *CredentialManagerVault) Get(account string) (string, error) {
	script := fmt.Sprintf(
		`$t=[Windows.Security.Credentials.PasswordVault,Windows.Security.Credentials,ContentType=WindowsRuntime]`+
			`;$v=New-Object $t;`+
			`try{$c=$v.Retrieve('%s','%s');$c.RetrievePassword();Write-Output $c.Password}`+
			`catch{exit 1}`,
		ServiceName, psEscape(account),
	)
	out, err := powershell(script)
	if err != nil {
		return "", ErrNotFound
	}
	return strings.TrimRight(out, "\r\n"), nil
}

func (v *CredentialManagerVault) Set(account, password string) error {
	_ = v.Delete(account)
	script := fmt.Sprintf(
		`$t=[Windows.Security.Credentials.PasswordVault,Windows.Security.Credentials,ContentType=WindowsRuntime]`+
			`;$v=New-Object $t;`+
			`$c=New-Object Windows.Security.Credentials.PasswordCredential('%s','%s','%s');`+
			`$v.Add($c)`,
		ServiceName, psEscape(account), psEscape(password),
	)
	_, err := powershell(script)
	if err != nil {
		return fmt.Errorf("credential manager set: %w", err)
	}
	return nil
}

func (v *CredentialManagerVault) Delete(account string) error {
	script := fmt.Sprintf(
		`$t=[Windows.Security.Credentials.PasswordVault,Windows.Security.Credentials,ContentType=WindowsRuntime]`+
			`;$v=New-Object $t;`+
			`try{$c=$v.Retrieve('%s','%s');$v.Remove($c)}catch{}`,
		ServiceName, psEscape(account),
	)
	_, _ = powershell(script)
	return nil
}

func powershell(script string) (string, error) {
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	return string(out), err
}

// psEscape escapes single quotes for PowerShell string literals.
func psEscape(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
