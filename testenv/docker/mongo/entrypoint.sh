#!/bin/bash
set -e

PASS="${MONGO_INITDB_ROOT_PASSWORD:-testuser}"
USER="${MONGO_INITDB_ROOT_USERNAME:-admin}"

# Start MongoDB in background.
mongod --bind_ip_all --fork --logpath /var/log/mongod.log

# Wait for MongoDB to be ready.
for i in $(seq 1 30); do
    if mongosh --quiet --eval "db.runCommand({ping:1})" &>/dev/null; then
        break
    fi
    sleep 1
done

# Create admin user.
mongosh --quiet admin --eval "
  db.createUser({
    user: '${USER}',
    pwd: '${PASS}',
    roles: [{role: 'root', db: 'admin'}]
  })
" &>/dev/null || true

# Restart with auth enabled.
mongod --shutdown
mongod --bind_ip_all --auth --fork --logpath /var/log/mongod.log

# Start SSH daemon in foreground.
exec /usr/sbin/sshd -D
