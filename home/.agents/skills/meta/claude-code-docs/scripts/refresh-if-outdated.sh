#!/usr/bin/env bash
# Refreshes the Claude Code docs cache when it is missing or was fetched under
# a different Claude Code version than the one running now. Prints a one-line
# status suitable for skill context injection.

set -euo pipefail

SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
# Absolute rather than walked up from SCRIPT_DIR: this skill is symlinked into
# a dotfiles tree that nests it one level deeper, so a relative walk resolves
# to a sibling directory the skill never reads. Must match fetch-docs.sh, or
# the check reports on a different cache than the one it refreshes.
CONTEXT_DIR="${HOME}/.claude/context"
INDEX_FILE="${CONTEXT_DIR}/INDEX.md"
VERSION_FILE="${CONTEXT_DIR}/.claude-version"
FETCH_SCRIPT="${SCRIPT_DIR}/fetch-docs.sh"

# "2.1.234 (Claude Code)" -> "2.1.234"; empty if the CLI is unavailable.
current_version="$(claude --version 2>/dev/null | awk '{print $1}')" || true

cached_version=""
[[ -f "$VERSION_FILE" ]] && cached_version="$(cat "$VERSION_FILE")"

needs_refresh=0
reason=""

if [[ "${CLAUDE_DOCS_FORCE_REFRESH:-0}" == "1" ]]; then
    needs_refresh=1
    reason="forced refresh"
elif [[ ! -f "$INDEX_FILE" ]]; then
    needs_refresh=1
    reason="no cache found"
elif [[ -z "$current_version" ]]; then
    # Cache exists but the version is unknowable; stale docs beat no docs.
    needs_refresh=0
elif [[ -z "$cached_version" ]]; then
    needs_refresh=1
    reason="cache has no version stamp"
elif [[ "$cached_version" != "$current_version" ]]; then
    needs_refresh=1
    reason="Claude Code ${cached_version} -> ${current_version}"
fi

if [[ "$needs_refresh" -eq 1 ]]; then
    echo "Refreshing docs cache (${reason})..."
    # Send fetch progress to stderr so the skill's injected block stays clean
    CLAUDE_DOCS_VERSION="$current_version" bash "$FETCH_SCRIPT" >&2
    echo "Cache refreshed for Claude Code ${current_version:-unknown}."
elif [[ -z "$current_version" ]]; then
    echo "Cache is for Claude Code ${cached_version:-unknown}; current version could not be determined, using cache as-is."
else
    echo "Cache is current for Claude Code ${current_version}."
fi
