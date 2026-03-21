# SSH Implementation Notes

- **PTY + SIGWINCH** (`internal/ssh/session.go`): `signal.Notify(sigCh, syscall.SIGWINCH)` → `session.WindowChange(h, w)`. Replaces `trap WINCH` in `conn.exp`.
- **Locale** (`internal/ssh/session.go`): `os.Setenv("LC_ALL", locale)` before starting PTY — handles EUC-KR servers.
- **`<space>`/`<tab>` literals** (`internal/migrate/parse_server_list.go`): Converted to real characters during migration.
- **Port 0** in DB → `internal/ssh/client.go` `defaultPort(proto)` resolves to 22 (ssh/sftp/sshfs), 21 (ftp), 23 (telnet), etc.
- **Docker** (`internal/ssh/docker.go`): Uses `docker exec` rather than SSH; handled separately from the main SSH chain.
- **ShellChain fallback** (`internal/ssh/shell_chain.go`): When `DialChain` fails with `ErrDialViaEOF` (proxy refuses direct-tcpip), the connect flow automatically retries using `ShellChain`. This replicates v1's `conn.exp` behavior: connects to hop[0] via SSH, then runs `ssh -tt user@hop[N]` inside the shell of each intermediate hop. Works on any server with shell access, no `AllowTcpForwarding` required on proxies. Uses regex-based prompt/auth detection identical to v1's expect patterns.
