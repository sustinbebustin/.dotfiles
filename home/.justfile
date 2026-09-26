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
