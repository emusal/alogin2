# Important Caveats

1. **Never store passwords in the DB** — `password` column must always be `_HIDDEN_` in production; use vault backends.
2. **`modernc.org/sqlite` vs `mattn/go-sqlite3`** — we use the pure-Go port to avoid CGO. Do not switch to `mattn/go-sqlite3` without updating all cross-compilation targets.
3. **Web UI is local-only** — do not add network-accessible auth without careful security review; the WebSocket terminal gives full shell access.
4. **`go build ./...` must compile on both macOS and Linux** — every platform-specific file needs a corresponding stub/alternative for the other platform.
