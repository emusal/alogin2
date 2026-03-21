// Package vault provides a multi-tier secret storage abstraction.
// Priority chain: macOS Keychain / Linux Secret Service → age encrypted file → plaintext DB column.
package vault

import "fmt"

const ServiceName = "alogin"

// Vault is the abstraction over all secret storage backends.
type Vault interface {
	// Get retrieves the password for the given account (usually "user@host").
	Get(account string) (string, error)

	// Set stores a password.
	Set(account, password string) error

	// Delete removes a stored password.
	Delete(account string) error

	// Name returns a human-readable backend identifier.
	Name() string
}

// AccountKey returns the canonical vault account key for a server entry.
func AccountKey(user, host string) string {
	return user + "@" + host
}

// ChainVault tries each backend in order and returns on first success.
type ChainVault struct {
	backends []Vault
}

// NewChain creates a ChainVault from one or more backends.
func NewChain(backends ...Vault) *ChainVault {
	return &ChainVault{backends: backends}
}

func (c *ChainVault) Name() string { return "chain" }

func (c *ChainVault) Get(account string) (string, error) {
	var errs []error
	for _, b := range c.backends {
		pwd, err := b.Get(account)
		if err == nil && pwd != "" {
			return pwd, nil
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", b.Name(), err))
		}
	}
	if len(errs) > 0 {
		return "", errs[0]
	}
	return "", ErrNotFound
}

func (c *ChainVault) Set(account, password string) error {
	// Write to the first backend only.
	if len(c.backends) == 0 {
		return fmt.Errorf("no vault backend configured")
	}
	return c.backends[0].Set(account, password)
}

func (c *ChainVault) Delete(account string) error {
	var last error
	for _, b := range c.backends {
		if err := b.Delete(account); err == nil {
			return nil
		} else {
			last = err
		}
	}
	return last
}

// ErrNotFound is returned when a secret is not found in any backend.
var ErrNotFound = fmt.Errorf("secret not found in vault")
