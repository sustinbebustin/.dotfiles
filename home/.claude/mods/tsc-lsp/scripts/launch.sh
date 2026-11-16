#!/usr/bin/env bash
#
# Start a TypeScript language server on stdio, backed by the project's own
# compiler so editor diagnostics match what the project's `tsc` reports.
#
# TypeScript 7+ ships a native server (`tsc --lsp --stdio`). Older releases
# have none; typescript-language-server drives their JS tsserver instead.
#
# The tsc used is the first found of:
#   1. node_modules/.bin/tsc in the project root or an ancestor
#   2. node_modules/.bin/tsc one directory down, for roots whose JS lives in a
#      child repo (e.g. an mws working copy's frontend/)
#   3. tsc on PATH
#
# stdout carries LSP protocol only, so every message goes to stderr.

set -euo pipefail

root="${CLAUDE_PROJECT_DIR:-$PWD}"

find_project_tsc() {
  local dir="$1" candidate
  while :; do
    if [ -x "$dir/node_modules/.bin/tsc" ]; then
      printf '%s\n' "$dir/node_modules/.bin/tsc"
      return 0
    fi
    [ "$dir" = / ] && break
    dir=$(dirname "$dir")
  done
  for candidate in "$1"/*/node_modules/.bin/tsc; do
    if [ -x "$candidate" ]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  return 1
}

# Major version of tsc $1 ("Version 7.0.2" -> 7); empty when unparseable.
major_version() {
  "$1" --version 2>/dev/null | sed -n 's/^Version \([0-9][0-9]*\)\..*/\1/p'
}

if ! tsc=$(find_project_tsc "$root") && ! tsc=$(command -v tsc); then
  echo "tsc-lsp: no TypeScript found: no node_modules/.bin/tsc in $root, its ancestors, or its child directories, and no tsc on PATH. Install TypeScript in the project (or globally: bun add -g typescript), then restart Claude Code." >&2
  exit 1
fi

major=$(major_version "$tsc")
if [ "${major:-0}" -ge 7 ]; then
  exec "$tsc" --lsp --stdio
fi

if command -v typescript-language-server >/dev/null; then
  exec typescript-language-server --stdio
fi

echo "tsc-lsp: $tsc is TypeScript ${major:-of unknown version}, which has no built-in language server (that needs 7+), and typescript-language-server is not on PATH. Install it (bun add -g typescript-language-server) or move the project to TypeScript 7, then restart Claude Code." >&2
exit 1
