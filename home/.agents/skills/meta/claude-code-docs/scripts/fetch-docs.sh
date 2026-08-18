#!/usr/bin/env bash
# Fetches all Claude Code documentation from code.claude.com/docs/llms.txt,
# stores each page as a markdown file in .claude/context/, and regenerates
# INDEX.md listing every cached doc with its title and description.

set -euo pipefail

LLMS_TXT_URL="https://code.claude.com/docs/llms.txt"
# Absolute rather than walked up from $0: this skill is symlinked into a
# dotfiles tree that nests it one level deeper, so a relative walk resolves to
# a sibling directory the skill never reads.
CONTEXT_DIR="${HOME}/.claude/context"
INDEX_FILE="${CONTEXT_DIR}/INDEX.md"
VERSION_FILE="${CONTEXT_DIR}/.claude-version"

# Stamped onto the cache so refresh-if-outdated.sh can tell which Claude Code
# version these docs were fetched for. Passed in by that script; recomputed
# here when fetch-docs.sh is run directly.
CLAUDE_VERSION="${CLAUDE_DOCS_VERSION:-$(claude --version 2>/dev/null | awk '{print $1}')}"

mkdir -p "$CONTEXT_DIR"

echo "Fetching llms.txt..."
LLMS_CONTENT=$(curl -sS "$LLMS_TXT_URL")

# Extract all URLs from the llms.txt content, skipping unwanted docs.
# Use ERE + POSIX classes so this works with both GNU and BSD grep.
SKIP_PATTERN='(agent-sdk/|amazon-bedrock|chrome|github-enterprise-server|gitlab-ci-cd|google-vertex-ai|jetbrains|microsoft-foundry|vs-code)'
URLS=$(echo "$LLMS_CONTENT" \
    | grep -oE 'https://code\.claude\.com/docs/en/[^[:space:])]+' \
    | grep -vE "https://code\.claude\.com/docs/en/${SKIP_PATTERN}")

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
        # Strip the 4-line "Documentation Index" preamble injected at the
        # top of every page. Done with head/tail for portability across
        # GNU and BSD coreutils (sed -i and {;} block syntax differ).
        # Copied back through the original path rather than moved onto it:
        # cached files may be symlinks into a dotfiles store, and mv would
        # replace each link with a regular file, detaching it from the store.
        out="${CONTEXT_DIR}/${filename}"
        if head -1 "$out" | grep -q '^> ## Documentation Index'; then
            tail -n +5 "$out" > "$out.tmp" && cat "$out.tmp" > "$out" && rm -f "$out.tmp"
        fi
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
    [[ -n "$CLAUDE_VERSION" ]] && echo "Claude Code version: $CLAUDE_VERSION"
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

if [[ -n "$CLAUDE_VERSION" ]]; then
    echo "$CLAUDE_VERSION" > "$VERSION_FILE"
else
    # No stamp is better than a wrong one: the next run then treats the cache
    # as unversioned and refetches rather than trusting docs of unknown vintage.
    rm -f "$VERSION_FILE"
fi

echo "Done. Files saved to $CONTEXT_DIR"
echo "Index written to $INDEX_FILE"
