# Platform-Specific Code

Build tags are used to separate platform code:

```
internal/vault/keychain_darwin.go     //go:build darwin
internal/vault/libsecret_linux.go     //go:build linux
internal/vault/keychain_linux.go      //go:build linux  (stub)
internal/cluster/terminal_macos.go    //go:build darwin
internal/cluster/terminal_linux.go    //go:build linux
internal/cluster/iterm.go             //go:build darwin
internal/cluster/tmux.go              (all platforms)
```

When adding platform-specific code, always provide a stub or fallback for the other platform so `go build ./...` succeeds on both macOS and Linux.
