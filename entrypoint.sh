#!/usr/bin/env bash
# Local development container entrypoint (root Dockerfile / docker-compose.yml).
# Production uses docker/entrypoint.sh.
set -euo pipefail

WEB_DIR="${WEB_DIR:-web}"
CONFIG_DIR="${REPLACEARR_CONFIG_DIR:-/config}"
PLUGIN_DIR="${CONFIG_DIR}/plugins"
DEFINITIONS_DIR="${CONFIG_DIR}/definitions"
# Bundled Replacearr-owned catalog (repo ./definitions, bind-mounted at /workspace).
BUNDLED_DEFINITIONS="/workspace/definitions"

mkdir -p /data/TV /data/Movies /data/Music /data/Books /data/Audiobooks /data/downloads \
	"${CONFIG_DIR}" \
	"${PLUGIN_DIR}" \
	"${DEFINITIONS_DIR}" \
	tmp

# Seed missing Cardigann YAML into the runtime definitions dir without overwriting edits.
if [ -d "${BUNDLED_DEFINITIONS}" ]; then
	seeded=0
	for def in "${BUNDLED_DEFINITIONS}"/*.yml "${BUNDLED_DEFINITIONS}"/*.yaml; do
		[ -f "${def}" ] || continue
		name=$(basename "${def}")
		if [ ! -f "${DEFINITIONS_DIR}/${name}" ]; then
			cp "${def}" "${DEFINITIONS_DIR}/${name}"
			seeded=$((seeded + 1))
		fi
	done
	if [ -f "${BUNDLED_DEFINITIONS}/schema.json" ] && [ ! -f "${DEFINITIONS_DIR}/schema.json" ]; then
		cp "${BUNDLED_DEFINITIONS}/schema.json" "${DEFINITIONS_DIR}/schema.json"
	fi
	if [ "${seeded}" -gt 0 ]; then
		echo "==> Seeded ${seeded} indexer YAML definition(s) into ${DEFINITIONS_DIR}"
	fi
fi

if [ ! -x "${WEB_DIR}/node_modules/.bin/vite" ]; then
	echo "==> Installing frontend dependencies..."
	(cd "${WEB_DIR}" && npm install)
fi

echo "==> Building plugins..."
make --no-print-directory build-plugins

if ! command -v air >/dev/null 2>&1; then
	echo "error: air not found in PATH; rebuild the development image (make start)" >&2
	exit 1
fi

echo "==> Starting Replacearr with Air live reload (:8989) and Vite dev server (:5173)..."
echo "==> Edit a .go file under the bind-mounted workspace to rebuild/restart the API process."

# Clear any leftover API listener from a previous Air child before we start.
# Without this, a deleted-inode process can hold :8989 across Air rebuild failures.
PORT="${REPLACEARR_PORT:-8989}"
if command -v fuser >/dev/null 2>&1; then
	fuser -k "${PORT}/tcp" >/dev/null 2>&1 || true
fi
# Kill by /proc exe identity (safer than pkill -f, which can match wrappers).
for proc in /proc/[0-9]*; do
	pid="${proc#/proc/}"
	exe="$(readlink "${proc}/exe" 2>/dev/null || true)"
	case "${exe}" in
		*'/tmp/replacearr'|*'/tmp/replacearr (deleted)')
			kill -9 "${pid}" >/dev/null 2>&1 || true
			;;
	esac
done

trap 'kill 0' INT TERM
air -c .air.toml &
(cd "${WEB_DIR}" && npm run dev) &
wait
