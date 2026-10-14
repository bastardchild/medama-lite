#!/bin/sh
set -e

# Ensure data directory exists and has correct ownership for nobody user
if [ -d "/app/data" ]; then
    chown -R nobody:nogroup /app/data
    chmod 750 /app/data
fi

# Ensure HOME is always /app/data for DuckDB extension cache
export HOME=/app/data

# If command starts with an option, prepend medamalite
if [ "${1#-}" != "$1" ]; then
    set -- /app/medamalite "$@"
fi

# If running medamalite as root, drop privileges to nobody
if [ "$1" = "/app/medamalite" ] || [ "$1" = "medamalite" ]; then
    exec gosu nobody:nogroup env HOME=/app/data "$@"
fi

# Otherwise, execute command as-is
exec "$@"
