#!/usr/bin/env bash
# Fetches all Claude Code documentation from code.claude.com/docs/llms.txt
# and stores each page as a markdown file in .claude/context/

set -euo pipefail

LLMS_TXT_URL="https://code.claude.com/docs/llms.txt"
SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
CONTEXT_DIR="${SCRIPT_DIR}/../context"

mkdir -p "$CONTEXT_DIR"

echo "Fetching llms.txt..."
LLMS_CONTENT=$(curl -sS "$LLMS_TXT_URL")

# Extract all URLs from the llms.txt content
URLS=$(echo "$LLMS_CONTENT" | grep -oP 'https://code\.claude\.com/docs/en/[^\s\)]+')

TOTAL=$(echo "$URLS" | wc -l)
COUNT=0

echo "Found $TOTAL URLs to fetch"
echo "---"

while IFS= read -r url; do
    COUNT=$((COUNT + 1))

    # Derive filename from URL path: docs/en/foo/bar.md -> foo--bar.md, docs/en/baz.md -> baz.md
    path="${url#https://code.claude.com/docs/en/}"
    filename="${path//\//__}"

    echo "[$COUNT/$TOTAL] $filename"
    if curl -sS --fail -o "${CONTEXT_DIR}/${filename}" "$url"; then
        echo "  -> saved"
    else
        echo "  -> FAILED (HTTP error)"
    fi
done <<< "$URLS"

echo "---"
echo "Done. Files saved to $CONTEXT_DIR"
