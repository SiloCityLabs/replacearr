#!/usr/bin/env bash
# Local development container entrypoint (root Dockerfile / docker-compose.yml).
# Production uses docker/entrypoint.sh.
set -euo pipefail

WEB_DIR="${WEB_DIR:-web}"
CONFIG_DIR="${REPLACEARR_CONFIG_DIR:-/config}"
PLUGIN_DIR="${REPLACEARR_PLUGIN_DIR:-bin/plugins}"
DEFINITIONS_DIR="${REPLACEARR_DEFINITIONS_DIR:-${CONFIG_DIR}/definitions}"
# Bundled Replacearr-owned catalog (repo ./definitions, bind-mounted at /workspace).
BUNDLED_DEFINITIONS="${REPLACEARR_BUNDLED_DEFINITIONS:-/workspace/definitions}"

mkdir -p /data/TV /data/Movies /data/downloads \
	"${CONFIG_DIR}" \
	"${PLUGIN_DIR}" \
	"${DEFINITIONS_DIR}"

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

echo "==> Starting Replacearr backend (:8989) and Vite dev server (:5173)..."
trap 'kill 0' INT TERM
go run ./cmd/replacearr &
(cd "${WEB_DIR}" && npm run dev) &
wait
