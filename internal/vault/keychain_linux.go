//go:build linux

package vault

// On Linux, use LibsecretVault instead of KeychainVault.
// NewKeychain is provided as an alias to NewLibsecret for cross-platform code.
func NewKeychain() Vault {
	v := NewLibsecret()
	if v == nil {
		return nil
	}
	return v
}
