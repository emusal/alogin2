package plugin

import (
	"fmt"
	"os"
	"strings"

	"github.com/emusal/alogin2/internal/vault"
)

// Secrets holds resolved secret values keyed by var name (e.g. "DB_PASS").
// Values are kept in memory only and must never be logged.
type Secrets map[string]string

// ResolveSecrets fetches all secrets declared in the plugin's auth.mapping.
func ResolveSecrets(p *Plugin, vlt vault.Vault) (Secrets, error) {
	secrets := make(Secrets, len(p.Auth.Mapping))
	for _, m := range p.Auth.Mapping {
		val, err := resolveOne(m, p.Auth.Provider, vlt)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", m.Var, err)
		}
		secrets[m.Var] = val
	}
	return secrets, nil
}

func resolveOne(m VarMapping, provider AuthProvider, vlt vault.Vault) (string, error) {
	switch provider {
	case AuthProviderVault:
		val, err := vlt.Get(m.Path)
		if err != nil {
			return "", fmt.Errorf("vault get %q: %w", m.Path, err)
		}
		return val, nil
	case AuthProviderEnv:
		val := os.Getenv(m.Path)
		if val == "" {
			return "", fmt.Errorf("env var %q not set", m.Path)
		}
		return val, nil
	case AuthProviderStatic:
		return m.Path, nil // path IS the literal value
	default:
		return "", fmt.Errorf("unknown auth provider %q", provider)
	}
}

// ApplyTemplate substitutes {{VAR}} placeholders with values from secrets.
// Unrecognized placeholders are left unchanged.
func ApplyTemplate(tmpl string, secrets Secrets) string {
	result := tmpl
	for k, v := range secrets {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	return result
}
