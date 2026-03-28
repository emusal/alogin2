#!/bin/bash
set -e

PASS="${MARIADB_ROOT_PASSWORD:-testuser}"
DB="${MARIADB_DATABASE:-testdb}"

# Ensure socket directory exists (may be missing in fresh container).
mkdir -p /run/mysqld
chown mysql:mysql /run/mysqld

# ── Phase 1: bootstrap (no networking, no grant tables) ───────────────────────
mariadbd --skip-networking --skip-grant-tables --user=mysql &
BOOT_PID=$!

# Wait for socket to become available.
for i in $(seq 1 60); do
    if mariadb -u root --protocol=socket -e "SELECT 1" &>/dev/null; then
        break
    fi
    sleep 0.5
done

# Set root password via native_password and create test DB.
mariadb -u root --protocol=socket <<SQL
FLUSH PRIVILEGES;
ALTER USER 'root'@'localhost' IDENTIFIED VIA mysql_native_password USING PASSWORD('${PASS}');
CREATE DATABASE IF NOT EXISTS \`${DB}\`;
FLUSH PRIVILEGES;
SQL

# Shut down the bootstrap instance gracefully.
kill "$BOOT_PID"
wait "$BOOT_PID" 2>/dev/null || true

# ── Phase 2: normal startup (background) ──────────────────────────────────────
mariadbd --user=mysql &

# Wait for the normal instance to accept connections.
for i in $(seq 1 60); do
    if mariadb -u root -p"${PASS}" --protocol=socket -e "SELECT 1" &>/dev/null; then
        break
    fi
    sleep 0.5
done

# ── Phase 3: SSH daemon (foreground, PID 1) ───────────────────────────────────
exec /usr/sbin/sshd -D
