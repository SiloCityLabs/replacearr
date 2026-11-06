#!/usr/bin/env bash
# Free a stale Replacearr API process (common Air failure mode) then exec the binary.
# Used by .air.toml full_bin and optionally by entrypoint.sh.
#
# IMPORTANT: do not `pkill -f` the binary path here — this script's own argv
# contains that path, so a pattern match would kill the wrapper before exec.
set -euo pipefail

PORT="${REPLACEARR_PORT:-8989}"
BIN="${1:-./tmp/replacearr}"
SELF_PID="$$"

if [ ! -x "${BIN}" ]; then
	echo "error: api binary not found or not executable: ${BIN}" >&2
	exit 1
fi

free_api_port() {
	# Kill whatever is still listening on the API port.
	if command -v fuser >/dev/null 2>&1; then
		fuser -k "${PORT}/tcp" >/dev/null 2>&1 || true
	fi

	# Kill leftover Air children by executable identity (not argv — argv would
	# match this wrapper because BIN is passed as $1).
	for proc in /proc/[0-9]*; do
		pid="${proc#/proc/}"
		if [ "${pid}" = "${SELF_PID}" ] || [ "${pid}" = "${PPID}" ]; then
			continue
		fi
		exe="$(readlink "${proc}/exe" 2>/dev/null || true)"
		case "${exe}" in
			*'/tmp/replacearr'|*'/tmp/replacearr (deleted)')
				kill -9 "${pid}" >/dev/null 2>&1 || true
				;;
		esac
	done

	# Brief pause so the kernel releases the listen socket.
	sleep 0.3
}

free_api_port
exec "${BIN}" "${@:2}"
