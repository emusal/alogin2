#!/bin/bash
set -e

PASS="${POSTGRES_PASSWORD:-testuser}"
DB="${POSTGRES_DB:-testdb}"

# Start PostgreSQL as postgres user in background.
su -c "POSTGRES_PASSWORD='${PASS}' POSTGRES_DB='${DB}' /usr/local/bin/docker-entrypoint.sh postgres &" postgres

# Wait for PostgreSQL to be ready.
for i in $(seq 1 30); do
    if su -c "psql -U postgres -c 'SELECT 1'" postgres &>/dev/null; then
        break
    fi
    sleep 1
done

# Start SSH daemon in foreground.
exec /usr/sbin/sshd -D
