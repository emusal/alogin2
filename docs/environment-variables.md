# Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ALOGIN_ROOT` | — | Legacy: parent dir for DB + config |
| `ALOGIN_DB` | `~/.local/share/alogin/alogin.db` | SQLite database path |
| `ALOGIN_CONFIG` | `~/.config/alogin/config.toml` | Config file path |
| `ALOGIN_LOG_LEVEL` | `0` | 0=errors, 1=info, 2=debug |
| `ALOGIN_LANG` | system | Default locale for new servers |
| `ALOGIN_SSHOPT` | — | Extra SSH options string |
| `ALOGIN_SSHCMD` | `ssh` | SSH binary path |
| `ALOGIN_KEYCHAIN_USE` | — | Force Keychain backend |
