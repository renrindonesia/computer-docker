#!/bin/sh
# Boots the virtual-display stack (Xvfb + window manager + VNC bridge) so the
# desktop and any headful browser are viewable through /vnc/, then hands off to
# the API server as PID 1's child.
#
# All display services are best-effort: if a binary is missing (e.g. the image
# was built without the VNC layer) the API still starts. The API's reverse proxy
# at /vnc/ simply returns 502 until websockify is up.
set -e

DISPLAY="${DISPLAY:-:99}"
export DISPLAY
SCREEN_GEOMETRY="${SCREEN_GEOMETRY:-1280x800x24}"
VNC_PORT="${VNC_PORT:-5900}"
NOVNC_PORT="${NOVNC_PORT:-6080}"
NOVNC_WEB="${NOVNC_WEB:-/usr/share/novnc}"

start() { echo "entrypoint: starting $*"; "$@" & }

if command -v Xvfb >/dev/null 2>&1; then
    start Xvfb "$DISPLAY" -screen 0 "$SCREEN_GEOMETRY" -nolisten tcp

    # Give Xvfb a moment to create the display socket before clients connect.
    i=0
    while [ ! -e "/tmp/.X11-unix/X${DISPLAY#:}" ] && [ "$i" -lt 50 ]; do
        i=$((i + 1)); sleep 0.1
    done

    command -v fluxbox >/dev/null 2>&1 && start fluxbox

    # x11vnc: password-protected if VNC_PASSWORD is set, otherwise open. An open
    # session on a public domain exposes the desktop to anyone — always set
    # VNC_PASSWORD in production.
    if [ -n "$VNC_PASSWORD" ]; then
        AUTH="-passwd $VNC_PASSWORD"
    else
        echo "entrypoint: WARNING VNC_PASSWORD unset — desktop is unauthenticated" >&2
        AUTH="-nopw"
    fi
    # shellcheck disable=SC2086
    command -v x11vnc >/dev/null 2>&1 && \
        start x11vnc -display "$DISPLAY" -forever -shared -rfbport "$VNC_PORT" -quiet $AUTH

    # websockify serves the noVNC web client and bridges browser websockets to
    # the raw VNC port. This is what the Go proxy forwards /vnc/ to.
    if command -v websockify >/dev/null 2>&1 && [ -d "$NOVNC_WEB" ]; then
        start websockify --web "$NOVNC_WEB" "$NOVNC_PORT" "localhost:${VNC_PORT}"
    fi
else
    echo "entrypoint: Xvfb not installed — skipping display stack" >&2
fi

exec api
