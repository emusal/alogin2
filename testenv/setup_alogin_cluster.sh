#!/usr/bin/env bash

# This script configures the local alogin registry using the testenv containers.
# It assumes alogin v2 is installed locally on the host machine.
#
# Gateway routing is handled by the 'testenv' profile rather than per-server
# --gateway flags. Activating the profile routes all connections through the
# bastion automatically.

echo "=========================================================="
echo "          alogin Test Environment Setup Script            "
echo "=========================================================="

if ! command -v alogin &> /dev/null; then
  echo "Error: 'alogin' command not found. Please install alogin first."
  exit 1
fi

echo "[1] Registering gateway (Bastion)..."
# Register bastion_host as a local hostname alias for localhost
alogin net hosts add bastion_host localhost 2>/dev/null || true

# Register the bastion server itself (direct connection, no profile needed)
alogin server add \
  --host bastion_host \
  --port 2222 \
  --user testuser \
  --password testuser \
  --proto ssh

echo "Registering gateway route 'bastion_gw' (jumps through bastion_host)..."
alogin net gateway add bastion_gw bastion_host

echo "[2] Creating 'testenv' profile with bastion_gw gateway..."
alogin net profile add testenv \
  --gateway bastion_gw \
  --desc "Test environment — routes all connections through bastion_host:2222"

echo "[3] Registering Target Servers (SSH)..."
SSH_SERVERS=(
  "target-ubuntu"
  "target-alpine"
  "target-centos7"
  "target-centos6"
  "target-legacy-rsa"
)

for target in "${SSH_SERVERS[@]}"; do
  echo "-> Adding ${target}..."
  alogin server add \
    --host "${target}" \
    --user testuser \
    --password testuser \
    --proto ssh || echo "Failed to add ${target}"
done

echo "[4] Registering Plugin Test Servers (DB/Cache)..."
DB_SERVERS=(
  "target-mariadb"
  "target-redis"
  "target-postgres"
  "target-mongo"
)

for target in "${DB_SERVERS[@]}"; do
  echo "-> Adding ${target}..."
  alogin server add \
    --host "${target}" \
    --user testuser \
    --password testuser \
    --proto ssh || echo "Failed to add ${target}"
done

echo "[4.5] Registering 3-hop chain (bastion → middle → deep-target)..."
# 'middle' is reached via the active profile (bastion), no extra flags needed.
alogin server add \
  --host middle \
  --user testuser \
  --password testuser \
  --proto ssh || echo "Failed to add middle"

echo "Registering gateway route 'middle_gw' (jumps through bastion_host)..."
alogin net gateway add middle_gw middle

# 'deep-target' uses 'middle' as a server-level gateway hop.
# With the testenv profile active the full chain becomes:
#   profile (bastion_gw) → middle (server gateway) → deep-target
alogin server add \
  --host deep-target \
  --user testuser \
  --password testuser \
  --proto ssh \
  --gateway middle_gw || echo "Failed to add deep-target"

echo "[5] Creating Clusters..."
# SSH target cluster
alogin ssh cluster add test-cluster \
  target-ubuntu target-alpine target-centos7 target-centos6 target-legacy-rsa

# DB/cache cluster (for batch inspection via MCP)
alogin ssh cluster add db-cluster \
  target-mariadb target-redis target-postgres target-mongo

echo "[6] Installing plugin definitions..."
PLUGIN_DIR="${HOME}/.config/alogin/plugins"
mkdir -p "${PLUGIN_DIR}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
for yaml in "${SCRIPT_DIR}/plugins/"*.yaml; do
  name="$(basename "${yaml}")"
  echo "-> Installing plugin: ${name}"
  cp "${yaml}" "${PLUGIN_DIR}/${name}"
done

echo "[7] Registering app bindings..."
alogin app add --name mariadb-test  --server target-mariadb  --app mariadb  --desc "MariaDB plugin test"
alogin app add --name redis-test    --server target-redis    --app redis    --desc "Redis plugin test"
alogin app add --name postgres-test --server target-postgres --app postgres --desc "PostgreSQL plugin test"
alogin app add --name mongo-test    --server target-mongo    --app mongo    --desc "MongoDB plugin test"

echo "[8] Activating 'testenv' profile..."
alogin net profile use testenv

echo "=========================================================="
echo "Setup Complete!"
echo ""
echo "Active profile: testenv (gateway: bastion_gw → bastion_host:2222)"
echo "All connections are now routed through the bastion automatically."
echo ""
echo "Commands to try:"
echo "  1) Multi-hop SSH:        alogin ssh connect target-centos6"
echo "  2) Cluster connect:      alogin ssh cluster test-cluster --mode tmux"
echo "  3) Server list:          alogin server list"
echo "  4) Plugin connect:       alogin app connect mariadb-test"
echo "  5) DB cluster inspect:   alogin ssh cluster db-cluster --mode tmux"
echo "  6) 3-hop SSH:            alogin ssh connect deep-target"
echo ""
echo "Profile management:"
echo "  alogin net profile list"
echo "  alogin net profile use none    # disable gateway routing"
echo "  alogin net profile use testenv # re-enable"
echo "=========================================================="
