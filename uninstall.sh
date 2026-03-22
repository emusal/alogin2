#!/usr/bin/env sh
# alogin uninstaller
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/emusal/alogin2/main/uninstall.sh | sh
#
# Environment variables:
#   ALOGIN_PURGE=1      — also remove database and vault (irreversible)
#   ALOGIN_INSTALL_DIR  — directory where alogin was installed (default: ~/.local/bin)

set -e

INSTALL_DIR="${ALOGIN_INSTALL_DIR:-$HOME/.local/bin}"
BIN="$INSTALL_DIR/alogin"

DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/alogin"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/alogin"
COMPLETIONS_DIR="$DATA_DIR/completions"

# ── Try the built-in CLI uninstall first ──────────────────────────────────────
if [ -x "$BIN" ]; then
  if [ "${ALOGIN_PURGE:-0}" = "1" ]; then
    "$BIN" uninstall --purge --yes
  else
    "$BIN" uninstall --yes
  fi
  exit 0
fi

# ── Fallback: manual removal ──────────────────────────────────────────────────
echo "alogin binary not found at $BIN — performing manual removal."

remove() {
  if [ -e "$1" ]; then
    rm -rf "$1"
    echo "Removed: $1"
  fi
}

remove "$BIN"
remove "$COMPLETIONS_DIR"
remove "$CONFIG_DIR"

if [ "${ALOGIN_PURGE:-0}" = "1" ]; then
  remove "$DATA_DIR"
  echo "All alogin data removed."
else
  echo "Data directory kept: $DATA_DIR"
  echo "Use ALOGIN_PURGE=1 to also remove the database and vault."
fi

echo "alogin uninstalled."
