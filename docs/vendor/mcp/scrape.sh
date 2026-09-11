#!/usr/bin/env bash
# Scrape the Model Context Protocol docs from https://modelcontextprotocol.io/llms.txt
# into ./docs, mirroring URL paths.
set -euo pipefail

INDEX_URL="https://modelcontextprotocol.io/llms.txt"
BASE_URL="https://modelcontextprotocol.io/"
OUT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/docs"
CONCURRENCY="${CONCURRENCY:-8}"

# Start clean so pages dropped from the index don't linger.
rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

# llms.txt links already point at markdown variants (<path>.md).
mapfile -t urls < <(
  curl -fsSL "$INDEX_URL" \
    | grep -oE "${BASE_URL//./\\.}[^) ]*\.md" \
    | sort -u
)

echo "Found ${#urls[@]} pages. Writing to $OUT_DIR (concurrency=$CONCURRENCY)"

fetch_one() {
  local url="$1"
  local rel="${url#"$BASE_URL"}"
  local out="$OUT_DIR/${rel}"
  mkdir -p "$(dirname "$out")"
  if curl -fsSL --retry 3 --retry-delay 1 "$url" -o "$out"; then
    echo "  ok   $rel"
  else
    echo "  FAIL $rel" >&2
    rm -f "$out"
  fi
}
export -f fetch_one
export OUT_DIR BASE_URL

printf '%s\n' "${urls[@]}" | xargs -P "$CONCURRENCY" -I{} bash -c 'fetch_one "$@"' _ {}

echo "Done. $(find "$OUT_DIR" -name '*.md' | wc -l) markdown files in $OUT_DIR"
