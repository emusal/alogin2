# Vault System

Priority chain (highest first):

1. **macOS Keychain** (`vault/keychain_darwin.go`, `//go:build darwin`) — `go-keychain`
2. **Linux Secret Service** (`vault/libsecret_linux.go`, `//go:build linux`) — D-Bus `org.freedesktop.secrets`
3. **age file vault** (`vault/age.go`) — `~/.local/share/alogin/vault.age`, master passphrase
4. **Plaintext** (`vault/plaintext.go`) — reads `password` column directly (legacy/migration only)

Service name format in Keychain/libsecret: `alogin:<host>`, account: `<user>`.
