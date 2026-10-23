#!/bin/sh
set -e

PUID="${PUID:-1000}"
PGID="${PGID:-1000}"
CONFIG_DIR="${REPLACEARR_CONFIG_DIR:-/config}"
PLUGIN_DIR="${REPLACEARR_PLUGIN_DIR:-${CONFIG_DIR}/plugins}"
BUNDLED_PLUGINS="/usr/local/share/replacearr/plugins"

echo "----------------------------------------------------------------"
echo " Replacearr — Unified Sonarr, Radarr & Prowlarr Replacement"
echo "----------------------------------------------------------------"
echo " PUID:       ${PUID}"
echo " PGID:       ${PGID}"
echo " TZ:         ${TZ:-UTC}"
echo " Config Dir: ${CONFIG_DIR}"
echo " Plugin Dir: ${PLUGIN_DIR}"
echo "----------------------------------------------------------------"

# Ensure group exists with target PGID
if ! getent group replacearr >/dev/null 2>&1; then
    if ! getent group "${PGID}" >/dev/null 2>&1; then
        addgroup -g "${PGID}" replacearr
    else
        EXISTING_GROUP=$(getent group "${PGID}" | cut -d: -f1)
        addgroup replacearr 2>/dev/null || true
    fi
fi

# Ensure user exists with target PUID
if ! getent passwd replacearr >/dev/null 2>&1; then
    if ! getent passwd "${PUID}" >/dev/null 2>&1; then
        adduser -D -H -u "${PUID}" -G "${EXISTING_GROUP:-replacearr}" replacearr 2>/dev/null || \
        adduser -D -H -u "${PUID}" replacearr 2>/dev/null || true
    fi
fi

# Ensure runtime directories exist
mkdir -p "${CONFIG_DIR}" "${PLUGIN_DIR}" "${CONFIG_DIR}/definitions" /data/tv /data/movies /data/downloads 2>/dev/null || true

# Seed bundled plugins into /config/plugins if not already present
if [ -d "${BUNDLED_PLUGINS}" ]; then
    for plugin_bin in "${BUNDLED_PLUGINS}"/*; do
        if [ -f "${plugin_bin}" ]; then
            plugin_name=$(basename "${plugin_bin}")
            if [ ! -f "${PLUGIN_DIR}/${plugin_name}" ]; then
                cp "${plugin_bin}" "${PLUGIN_DIR}/${plugin_name}"
                chmod +x "${PLUGIN_DIR}/${plugin_name}"
            fi
        fi
    done
fi

# Ensure proper ownership of configuration directory
chown -R "${PUID}:${PGID}" "${CONFIG_DIR}" 2>/dev/null || true

exec su-exec "${PUID}:${PGID}" /usr/local/bin/replacearr "$@"
