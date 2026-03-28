#!/bin/bash
set -e

PASS="${REDIS_PASSWORD:-testuser}"

# Start Redis with requirepass in background.
redis-server --requirepass "${PASS}" --daemonize yes

# Start SSH daemon in foreground.
exec /usr/sbin/sshd -D
