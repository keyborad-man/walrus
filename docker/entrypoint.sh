#!/usr/bin/env bash
set -euo pipefail

DATA_DIR="${WALRUS_DATA_DIR:-/data}"
mkdir -p "${DATA_DIR}/steamcmd" "${DATA_DIR}/dst-server" "${DATA_DIR}/clusters" "${DATA_DIR}/backups"

# Ensure steamcmd binary is available inside /data/steamcmd for persistent updates
if [ ! -f "${DATA_DIR}/steamcmd/steamcmd.sh" ] && [ -f "/opt/steamcmd/steamcmd.sh" ]; then
  cp -a /opt/steamcmd/. "${DATA_DIR}/steamcmd/"
fi

exec /usr/local/bin/walrus "$@"
