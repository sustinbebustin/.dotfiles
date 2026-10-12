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
