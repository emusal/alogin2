#!/usr/bin/env sh
# alogin installer
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/emusal/alogin2/main/install.sh | sh
#
# Environment variables:
#   ALOGIN_VERSION      — specific version to install (default: latest)
#   ALOGIN_NO_WEB=1     — install CLI-only binary (without Web UI)
#   ALOGIN_INSTALL_DIR  — install directory (default: ~/.local/bin)

set -e

REPO="emusal/alogin2"
INSTALL_DIR="${ALOGIN_INSTALL_DIR:-$HOME/.local/bin}"

# ── Detect OS ─────────────────────────────────────────────────────────────────
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux)  OS=linux  ;;
  darwin) OS=darwin ;;
  *)
    echo "Error: unsupported OS: $OS" >&2
    exit 1
    ;;
esac

# ── Detect architecture ────────────────────────────────────────────────────────
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)          ARCH=amd64 ;;
  aarch64 | arm64) ARCH=arm64 ;;
  *)
    echo "Error: unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

# ── Determine binary name ──────────────────────────────────────────────────────
if [ "${ALOGIN_NO_WEB:-0}" = "1" ]; then
  BINARY="alogin-${OS}-${ARCH}"
else
  BINARY="alogin-web-${OS}-${ARCH}"
fi

# ── Determine download URL ─────────────────────────────────────────────────────
if [ -n "${ALOGIN_VERSION:-}" ]; then
  # Strip leading 'v' if present
  VERSION="${ALOGIN_VERSION#v}"
  URL="https://github.com/${REPO}/releases/download/v${VERSION}/${BINARY}"
else
  URL="https://github.com/${REPO}/releases/latest/download/${BINARY}"
fi

# ── Create install directory ───────────────────────────────────────────────────
mkdir -p "$INSTALL_DIR"
DEST="$INSTALL_DIR/alogin"

# ── Download ───────────────────────────────────────────────────────────────────
echo "Downloading ${BINARY} ..."
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$URL" -o "$DEST"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "$DEST" "$URL"
else
  echo "Error: curl or wget is required" >&2
  exit 1
fi

chmod +x "$DEST"

# ── Verify ─────────────────────────────────────────────────────────────────────
if ! "$DEST" version >/dev/null 2>&1; then
  echo "Error: downloaded binary failed to run (wrong OS/arch?)" >&2
  rm -f "$DEST"
  exit 1
fi

INSTALLED_VERSION=$("$DEST" version 2>/dev/null | head -1)
echo "Installed: ${INSTALLED_VERSION}"
echo "Location : ${DEST}"

# ── PATH check ─────────────────────────────────────────────────────────────────
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    echo ""
    echo "NOTE: ${INSTALL_DIR} is not in your PATH."
    echo "Add the following to your ~/.bashrc or ~/.zshrc:"
    echo ""
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
    echo ""
    ;;
esac

# ── Shell completion hint ──────────────────────────────────────────────────────
echo "To set up shell completions:"
echo "  alogin completion install              # zsh (default)"
echo "  alogin completion install --shell bash # bash"
