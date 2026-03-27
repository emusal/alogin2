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

# Register the server itself
alogin compute add \
  --host bastion_host \
  --port 2222 \
  --user testuser \
  --password testuser \
  --proto ssh

echo "Registering Bastion gateway path..."
# Define a gateway path named 'bastion_gw' that jumps through 'bastion_host'
alogin auth gateway add bastion_gw bastion_host

echo "[2] Registering Target Servers..."
SERVERS=(
  "target-ubuntu"
  "target-alpine"
  "target-centos7"
  "target-centos6"
  "target-legacy-rsa"
)

for target in "${SERVERS[@]}"; do
  echo "-> Adding ${target}..."
  alogin compute add \
    --host "${target}" \
    --user testuser \
    --password testuser \
    --proto ssh \
    --gateway bastion_gw || echo "Failed to add ${target}"
done

echo "[3] Creating a Cluster (test-cluster)..."
# Adding all target servers into a single cluster using the new command
alogin access cluster add test-cluster target-ubuntu target-alpine target-centos7 target-centos6 target-legacy-rsa

echo "=========================================================="
echo "Setup Complete!"
echo "Commands to try:"
echo "  1) Multi-hop SSH: alogin access ssh target-centos6 --auto-gw"
echo "  2) Cluster connect: alogin access cluster test-cluster --auto-gw --mode tmux"
echo "  3) Server list: alogin compute list"
echo "=========================================================="
