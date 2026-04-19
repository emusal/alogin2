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

# ── Install skills ────────────────────────────────────────────────────────────
SKILLS_DIR="${ALOGIN_SKILLS_DIR:-$HOME/.agent/skills}"

if [ "${ALOGIN_NO_SKILLS:-0}" != "1" ]; then
  if [ -n "${ALOGIN_VERSION:-}" ]; then
    SKILLS_URL="https://github.com/${REPO}/releases/download/v${VERSION}/skills.tar.gz"
  else
    SKILLS_URL="https://github.com/${REPO}/releases/latest/download/skills.tar.gz"
  fi

  TMP_SKILLS=$(mktemp -d)
  trap 'rm -rf "$TMP_SKILLS"' EXIT

  echo "Downloading skills ..."
  SKILLS_OK=0
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$SKILLS_URL" -o "$TMP_SKILLS/skills.tar.gz" && SKILLS_OK=1
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$TMP_SKILLS/skills.tar.gz" "$SKILLS_URL" && SKILLS_OK=1
  fi

  if [ "$SKILLS_OK" = "1" ]; then
    mkdir -p "$TMP_SKILLS/skills"
    tar -xzf "$TMP_SKILLS/skills.tar.gz" -C "$TMP_SKILLS/skills"
    for skill_dir in "$TMP_SKILLS/skills/"/*/; do
      [ -d "$skill_dir" ] || continue
      name=$(basename "$skill_dir")
      mkdir -p "$SKILLS_DIR/$name"
      cp "$skill_dir/SKILL.md" "$SKILLS_DIR/$name/SKILL.md"
    done
    echo "Skills    : ${SKILLS_DIR}"
  else
    echo "Warning: could not download skills (skipping)" >&2
  fi
fi

# ── Install sample plugins ─────────────────────────────────────────────────────
PLUGIN_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/alogin/plugins"

if [ "${ALOGIN_NO_PLUGINS:-0}" != "1" ]; then
  if [ -n "${ALOGIN_VERSION:-}" ]; then
    PLUGINS_URL="https://github.com/${REPO}/releases/download/v${VERSION}/plugins.tar.gz"
  else
    PLUGINS_URL="https://github.com/${REPO}/releases/latest/download/plugins.tar.gz"
  fi

  TMP_PLUGINS=$(mktemp -d)
  trap 'rm -rf "$TMP_PLUGINS"' EXIT

  echo "Downloading sample plugins ..."
  PLUGINS_OK=0
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$PLUGINS_URL" -o "$TMP_PLUGINS/plugins.tar.gz" && PLUGINS_OK=1
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$TMP_PLUGINS/plugins.tar.gz" "$PLUGINS_URL" && PLUGINS_OK=1
  fi

  if [ "$PLUGINS_OK" = "1" ]; then
    mkdir -p "$PLUGIN_DIR"
    tar -xzf "$TMP_PLUGINS/plugins.tar.gz" -C "$TMP_PLUGINS"
    # Copy only files that don't already exist (don't overwrite user-modified plugins)
    for f in "$TMP_PLUGINS/plugins/"*.yaml; do
      [ -f "$f" ] || continue
      name=$(basename "$f")
      if [ ! -f "$PLUGIN_DIR/$name" ]; then
        cp "$f" "$PLUGIN_DIR/$name"
      fi
    done
    echo "Plugins   : ${PLUGIN_DIR}"
  else
    echo "Warning: could not download sample plugins (skipping)" >&2
  fi
fi

# ── Post-install setup hint ───────────────────────────────────────────────────
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo " Next: add the following to your ~/.bashrc or ~/.zshrc"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "  # 1. Make sure alogin is on your PATH"
echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
echo ""
echo "  # 2. Install tab-completion (run once)"
echo "  alogin completion install --shell bash"
echo ""
echo "  # 3. Load completion + shell shims on every new shell"
echo "  source <(alogin completion bash)"
echo "  source <(alogin shell-init)"
echo ""
echo "Then reload your shell:  source ~/.bashrc  (or open a new terminal)"
echo ""
