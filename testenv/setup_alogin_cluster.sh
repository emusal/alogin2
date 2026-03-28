#!/usr/bin/env bash

# This script configures the local alogin registry using the testenv containers.
# It assumes alogin v2 is installed locally on the host machine.

echo "=========================================================="
echo "          alogin Test Environment Setup Script            "
echo "=========================================================="

if ! command -v alogin &> /dev/null; then
  echo "Error: 'alogin' command not found. Please install alogin first."
  exit 1
fi

echo "[1] Registering gateway (Bastion)..."
# Use 'bastion_host' as a known hostname for localhost
alogin net hosts add bastion_host localhost 2>/dev/null || true

# Register the bastion server itself
alogin compute add \
  --host bastion_host \
  --port 2222 \
  --user testuser \
  --password testuser \
  --proto ssh

echo "Registering Bastion gateway path..."
# Define a gateway path named 'bastion_gw' that jumps through 'bastion_host'
alogin auth gateway add bastion_gw bastion_host

echo "[2] Registering Target Servers (SSH)..."
SSH_SERVERS=(
  "target-ubuntu"
  "target-alpine"
  "target-centos7"
  "target-centos6"
  "target-legacy-rsa"
)

for target in "${SSH_SERVERS[@]}"; do
  echo "-> Adding ${target}..."
  alogin compute add \
    --host "${target}" \
    --user testuser \
    --password testuser \
    --proto ssh \
    --gateway bastion_gw || echo "Failed to add ${target}"
done

echo "[3] Registering Plugin Test Servers (DB/Cache)..."
DB_SERVERS=(
  "target-mariadb"
  "target-redis"
  "target-postgres"
  "target-mongo"
)

for target in "${DB_SERVERS[@]}"; do
  echo "-> Adding ${target}..."
  alogin compute add \
    --host "${target}" \
    --user testuser \
    --password testuser \
    --proto ssh \
    --gateway bastion_gw || echo "Failed to add ${target}"
done

echo "[4] Creating Clusters..."
# SSH target cluster
alogin access cluster add test-cluster \
  target-ubuntu target-alpine target-centos7 target-centos6 target-legacy-rsa

# DB/cache cluster (for batch inspection via MCP)
alogin access cluster add db-cluster \
  target-mariadb target-redis target-postgres target-mongo

echo "[5] Installing plugin definitions..."
PLUGIN_DIR="${HOME}/.config/alogin/plugins"
mkdir -p "${PLUGIN_DIR}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
for yaml in "${SCRIPT_DIR}/plugins/"*.yaml; do
  name="$(basename "${yaml}")"
  echo "-> Installing plugin: ${name}"
  cp "${yaml}" "${PLUGIN_DIR}/${name}"
done

echo "[6] Registering app-server bindings..."
alogin app-server add --name mariadb-test --server target-mariadb --app mariadb --auto-gw --desc "MariaDB plugin test"
alogin app-server add --name redis-test   --server target-redis   --app redis   --auto-gw --desc "Redis plugin test"
alogin app-server add --name postgres-test --server target-postgres --app postgres --auto-gw --desc "PostgreSQL plugin test"
alogin app-server add --name mongo-test   --server target-mongo   --app mongo   --auto-gw --desc "MongoDB plugin test"

echo "=========================================================="
echo "Setup Complete!"
echo "Commands to try:"
echo "  1) Multi-hop SSH:        alogin access ssh target-centos6 --auto-gw"
echo "  2) Cluster connect:      alogin access cluster test-cluster --mode tmux"
echo "  3) Server list:          alogin compute list"
echo "  4) Plugin connect:       alogin app-server connect mariadb-test"
echo "  5) DB cluster inspect:   alogin access cluster db-cluster --mode tmux"
echo "=========================================================="
