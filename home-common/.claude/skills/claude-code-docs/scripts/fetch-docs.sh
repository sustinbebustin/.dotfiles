#!/usr/bin/env bash
# Fetches all Claude Code documentation from code.claude.com/docs/llms.txt,
# stores each page as a markdown file in .claude/context/, and regenerates
# INDEX.md listing every cached doc with its title and description.

set -euo pipefail

LLMS_TXT_URL="https://code.claude.com/docs/llms.txt"
SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
# scripts -> claude-code-docs -> skills -> .claude
CONTEXT_DIR="${SCRIPT_DIR}/../../../context"
INDEX_FILE="${CONTEXT_DIR}/INDEX.md"

mkdir -p "$CONTEXT_DIR"

echo "Fetching llms.txt..."
LLMS_CONTENT=$(curl -sS "$LLMS_TXT_URL")

# Extract all URLs from the llms.txt content, skipping unwanted docs
SKIP_PATTERN='(agent-sdk/|amazon-bedrock|chrome|github-enterprise-server|gitlab-ci-cd|google-vertex-ai|jetbrains|microsoft-foundry|vs-code)'
URLS=$(echo "$LLMS_CONTENT" \
    | grep -oP 'https://code\.claude\.com/docs/en/[^\s\)]+' \
    | grep -vP "https://code\.claude\.com/docs/en/${SKIP_PATTERN}")

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
        # Strip the 4-line "Documentation Index" preamble injected at the top of every page
        sed -i '1{/^> ## Documentation Index/{N;N;N;d;}}' "${CONTEXT_DIR}/${filename}"
        echo "  -> saved"
    else
        echo "  -> FAILED (HTTP error)"
    fi
done <<< "$URLS"

echo "---"
echo "Rebuilding INDEX.md..."

{
    echo "# Claude Code Docs Index"
    echo
    echo "Cached docs under \`~/.claude/context/\`. Read the file matching your question."
    echo "Last refreshed: $(date -Iseconds)"
    echo
    for f in "$CONTEXT_DIR"/*.md; do
        name="$(basename "$f")"
        [[ "$name" == "INDEX.md" ]] && continue
        title="$(awk '/^# / { sub(/^# /, ""); print; exit }' "$f")"
        desc="$(awk '/^> / { sub(/^> /, ""); print; exit }' "$f")"
        if [[ -n "$title" && -n "$desc" ]]; then
            printf -- "- \`%s\` — **%s** — %s\n" "$name" "$title" "$desc"
        elif [[ -n "$title" ]]; then
            printf -- "- \`%s\` — **%s**\n" "$name" "$title"
        else
            printf -- "- \`%s\`\n" "$name"
        fi
    done
} > "$INDEX_FILE"

echo "Done. Files saved to $CONTEXT_DIR"
echo "Index written to $INDEX_FILE"
