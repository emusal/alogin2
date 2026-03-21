# Migration Notes

Reference files (v1):
- `../server_list` — TSV: `proto host user passwd port gateway locale`
- `../gateway_list` — `name  hop1 hop2 ...`
- `../alias_hosts` — `alias  user@host`
- `../clusters` — `cluster_name  host1 host2 ...`
- `../term_themes` — `locale  theme_name`

Special handling:
- `<space>` in password field → literal space
- `<tab>` in password field → literal tab
- `-` in port field → `0` (use default)
- `-` in gateway field → `NULL` (direct connection)
- `-` in locale field → `""` (system default)
- `_HIDDEN_` in password → look up in macOS Keychain (legacy v1 keychain mode)
