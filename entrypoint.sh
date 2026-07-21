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
export VNC_PORT

start() { echo "entrypoint: starting $*"; "$@" & }

if command -v Xvfb >/dev/null 2>&1; then
    # Xvfb needs this dir to create its display socket; it may not exist on a
    # fresh/volume-mounted /tmp. Without it Xvfb fails and x11vnc can't connect.
    mkdir -p /tmp/.X11-unix && chmod 1777 /tmp/.X11-unix

    # Railway keeps /tmp across restarts, so a previous instance's lock and
    # socket for our display survive and make Xvfb bail with "server already
    # active for display". Clear them for our display number before starting.
    DNUM="${DISPLAY#:}"
    rm -f "/tmp/.X${DNUM}-lock" "/tmp/.X11-unix/X${DNUM}" 2>/dev/null || true

    start Xvfb "$DISPLAY" -screen 0 "$SCREEN_GEOMETRY" -nolisten tcp

    # Window manager. Like x11vnc, fluxbox dies instantly if it opens the display
    # before Xvfb is ready — that race left the screen a bare (black) root with no
    # WM, toolbar or menu. Retry until X is up. Also paint a visible root colour
    # first (if xsetroot exists) so a connected-but-empty desktop reads as alive.
    if command -v fluxbox >/dev/null 2>&1; then
        start sh -c '
            until command -v xsetroot >/dev/null 2>&1 && xsetroot -solid "#2e3440" 2>/dev/null; do
                sleep 1; n=$((${n:-0}+1)); [ "${n:-0}" -ge 30 ] && break
            done
            n=0
            until fluxbox; do
                n=$((n + 1)); [ "$n" -ge 30 ] && { echo "entrypoint: fluxbox gave up waiting for X" >&2; break; }
                echo "entrypoint: fluxbox waiting for X on $DISPLAY ($n)"; sleep 1
            done'
    fi

    # x11vnc + websockify expose the desktop under /vnc/, which sits OUTSIDE the
    # API key (noVNC's relative asset/websocket URLs can't carry ?key=). The VNC
    # password is the only guard on full mouse/keyboard control of the machine.
    # To avoid a second secret it defaults to API_KEY; set VNC_PASSWORD only to
    # override. Fail closed: with neither set we do NOT start the VNC bridge. The
    # API keeps running; the /vnc/ proxy just returns 502 until a secret exists.
    #
    # NOTE: the RFB protocol truncates passwords to 8 chars, so only the first 8
    # of the key are actually checked — connect with the same value you use for
    # ?key= (or just its first 8 chars).
    VNC_PW="${VNC_PASSWORD:-$API_KEY}"
    if [ -z "$VNC_PW" ]; then
        echo "entrypoint: no API_KEY/VNC_PASSWORD — NOT exposing desktop (set one to enable /vnc/)" >&2
    elif ! command -v x11vnc >/dev/null 2>&1; then
        echo "entrypoint: x11vnc not installed — /vnc/ disabled" >&2
    elif ! command -v websockify >/dev/null 2>&1; then
        echo "entrypoint: websockify not installed — /vnc/ disabled" >&2
    else
        # Race-proof: x11vnc exits immediately if X isn't up yet, so retry until
        # Xvfb is ready rather than one-shotting it (the cause of the earlier
        # "Couldn't connect to XServer:99"). Password passed via env to dodge
        # shell-quoting issues and to keep it out of the process arg list.
        export X11VNC_PW="$VNC_PW"
        start sh -c '
            n=0
            until x11vnc -display "$DISPLAY" -forever -shared -rfbport "$VNC_PORT" \
                    -quiet -passwd "$X11VNC_PW"; do
                n=$((n + 1))
                [ "$n" -ge 60 ] && { echo "entrypoint: x11vnc gave up waiting for $DISPLAY" >&2; break; }
                echo "entrypoint: x11vnc waiting for X on $DISPLAY ($n)"
                sleep 1
            done'

        # Locate the noVNC web assets — the Debian package path varies by
        # release. websockify --web serves them; the Go proxy forwards /vnc/ here.
        WEB=""
        for d in "$NOVNC_WEB" /usr/share/novnc /usr/share/webapps/novnc /usr/lib/novnc; do
            if [ -n "$d" ] && [ -d "$d" ]; then WEB="$d"; break; fi
        done
        if [ -z "$WEB" ]; then
            echo "entrypoint: noVNC web dir not found (looked in /usr/share/novnc etc) — /vnc/ will 404 for assets" >&2
            start websockify "$NOVNC_PORT" "localhost:${VNC_PORT}"
        else
            echo "entrypoint: serving noVNC from $WEB"
            start websockify --web "$WEB" "$NOVNC_PORT" "localhost:${VNC_PORT}"
        fi
    fi
else
    echo "entrypoint: Xvfb not installed — skipping display stack" >&2
fi

exec api
