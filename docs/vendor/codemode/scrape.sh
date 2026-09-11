#!/usr/bin/env bash
# Scrape the Code Mode and MCP docs from https://developers.cloudflare.com/agents/llms.txt
# into ./docs, mirroring URL paths.
set -euo pipefail

INDEX_URL="https://developers.cloudflare.com/agents/llms.txt"
BASE_URL="https://developers.cloudflare.com/agents/"
OUT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/docs"
CONCURRENCY="${CONCURRENCY:-8}"

# Pages kept, as paths relative to BASE_URL: the Code Mode and MCP sections, plus
# pages elsewhere that devote a substantial section to either.
KEEP_REGEX='^(tools/codemode/|tools/mcp/|model-context-protocol/'
KEEP_REGEX+='|concepts/tools/|concepts/agentic-patterns/human-in-the-loop/'
KEEP_REGEX+='|harnesses/think/tools/|tools/payments/x402/charge-for-mcp-tools/'
KEEP_REGEX+='|tools/payments/mpp/accept-payments/|tools/payments/mpp/pay-from-agents-sdk/)'

# Start clean so pages dropped from the filter or the index don't linger.
rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

# llms.txt links already point at markdown variants (<path>/index.md).
mapfile -t urls < <(
  curl -fsSL "$INDEX_URL" \
    | grep -oE "${BASE_URL//./\\.}[^) ]*index\.md" \
    | sort -u \
    | while read -r url; do
        if grep -qE "$KEEP_REGEX" <<<"${url#"$BASE_URL"}"; then echo "$url"; fi
      done
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
