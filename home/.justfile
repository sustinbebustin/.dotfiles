# Machine-wide recipes. Call with `just -g <recipe>` anywhere; a project
# justfile with `set fallback := true` also falls through to these.

set positional-arguments := true

# Serve over HTTP from where you run it: no path serves this directory, a directory serves that directory, a file serves its tree and prints the file's URL
[no-cd]
serve path='.' port='8765':
    #!/usr/bin/env bash
    set -euo pipefail
    target=$1
    ip=$(hostname -I | awk '{print $1}')
    if [ -d "$target" ]; then
        root=$target
        page=''
    elif [ -f "$target" ]; then
        # Serve from here when the file is under this directory, so its
        # relative links (../assets, sibling pages) keep resolving. -s keeps
        # symlinked dirs (a clone's .workspace) inside the tree.
        rel=$(realpath -s --relative-to="$PWD" "$target")
        if [[ $rel == ../* ]]; then
            root=$(dirname "$target")
            page=$(basename "$target")
        else
            root=.
            page=$rel
        fi
    else
        echo "serve: $target is not a file or directory (relative paths resolve from $PWD)" >&2
        exit 1
    fi
    echo "open: http://$ip:$2/$page"
    exec python3 -m http.server "$2" -d "$root"

# Kill whatever is listening on the given TCP port(s), e.g. `just kill 3000` or `just kill 3000 3001`
kill +ports:
    #!/usr/bin/env bash
    set -uo pipefail
    for port in {{ports}}; do
        pids=$(ss -ltnpH "sport = :$port" 2>/dev/null | grep -oP 'pid=\K[0-9]+' | sort -u)
        if [ -z "$pids" ]; then
            echo "port $port: nothing listening"
            continue
        fi
        kill $pids 2>/dev/null
        sleep 0.5
        # Escalate if any survived.
        alive=$(for p in $pids; do kill -0 "$p" 2>/dev/null && echo "$p"; done)
        [ -n "$alive" ] && kill -9 $alive 2>/dev/null
        echo "port $port: killed $(echo $pids | tr '\n' ' ')"
    done

# Start the Plannotator Inbox (or reuse the running one) and publish it on the tailnet over HTTPS
inbox:
    #!/usr/bin/env bash
    set -euo pipefail
    # The Inbox binds 127.0.0.1 only and has no --tailscale flag, so front it
    # with Tailscale Serve on the same port it picked. It also refuses Host
    # headers it doesn't know, and reads PLANNOTATOR_ALLOWED_HOSTS only at
    # startup, so an Inbox started without our hostname has to be restarted.
    host=$(tailscale status --json | jq -r '.Self.DNSName // "" | rtrimstr(".")')
    if [ -z "$host" ]; then
        echo "inbox: no tailnet DNS name from 'tailscale status'; is Tailscale up and logged in?" >&2
        exit 1
    fi
    export PLANNOTATOR_ALLOWED_HOSTS="${PLANNOTATOR_ALLOWED_HOSTS:+$PLANNOTATOR_ALLOWED_HOSTS,}$host"
    start() {
        url=$(plannotator inbox --background)
        port=$(sed -E 's#^https?://[^:/]+:([0-9]+).*#\1#' <<<"$url")
        if ! [[ $port =~ ^[0-9]+$ ]]; then
            echo "inbox: could not read a port from plannotator's output: $url" >&2
            exit 1
        fi
    }
    # Retries cover an Inbox that is still starting when --background returns.
    accepts_host() {
        [ "$(curl -s --retry 5 --retry-connrefused --retry-delay 1 -o /dev/null -w '%{http_code}' -H "Host: $host:$port" "http://127.0.0.1:$port/")" = 200 ]
    }
    start
    if ! accepts_host; then
        echo "inbox: running Inbox doesn't allow $host; restarting it (messages are kept on disk)" >&2
        {{ just_executable() }} -f {{ justfile() }} inbox-stop >/dev/null
        start
        if ! accepts_host; then
            echo "inbox: restarted Inbox on port $port still refuses $host; check PLANNOTATOR_ALLOWED_HOSTS" >&2
            exit 1
        fi
    fi
    tailscale serve --bg --https="$port" "http://127.0.0.1:$port" >/dev/null
    echo "open: https://$host:$port/"

# Stop the background Plannotator Inbox and remove its Tailscale Serve mapping; messages stay on disk
inbox-stop:
    #!/usr/bin/env bash
    set -euo pipefail
    # Plannotator has no stop command. The running Inbox records its port in
    # <data dir>/inbox/inbox.json; the data dir resolves the way Plannotator
    # resolves it. The recorded pid and port both go stale (and can be reused)
    # if the Inbox dies without cleaning up, so kill only when the recorded pid
    # is still the listener on the recorded port.
    if [ -n "${PLANNOTATOR_DATA_DIR:-}" ]; then
        data=${PLANNOTATOR_DATA_DIR/#\~/$HOME}
    elif [ -d "$HOME/.plannotator" ]; then
        data=$HOME/.plannotator
    elif [[ ${XDG_DATA_HOME:-} == /* ]]; then
        data=$XDG_DATA_HOME/plannotator
    else
        data=$HOME/.plannotator
    fi
    state=$data/inbox/inbox.json
    if [ ! -f "$state" ]; then
        echo "inbox-stop: no Inbox state at $state; nothing to stop"
        exit 0
    fi
    port=$(jq -r '.port' "$state")
    pid=$(jq -r '.pid' "$state")
    if ss -ltnpH "sport = :$port" 2>/dev/null | grep -q "pid=$pid,"; then
        {{ just_executable() }} -f {{ justfile() }} kill "$port"
    else
        echo "inbox-stop: Inbox pid $pid is not listening on port $port; state is stale, nothing killed"
    fi
    if tailscale serve status --json | jq -e --arg p "$port" '(.TCP // {}) | has($p)' >/dev/null; then
        tailscale serve --https="$port" off >/dev/null
        echo "removed: Tailscale Serve mapping on port $port"
    fi
