#!/usr/bin/env bash
# Fetches all Turborepo documentation referenced in turborepo.dev/llms.txt,
# stores each page as a markdown file under docs/ (preserving the path
# structure from llms.txt), and regenerates INDEX.md listing every cached
# doc with its title and section.

set -euo pipefail

LLMS_TXT_URL="https://turborepo.dev/llms.txt"
DOCS_BASE_URL="https://turborepo.dev/docs"
SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
# scripts -> turborepo
DOCS_DIR="${SCRIPT_DIR}/../docs"
INDEX_FILE="${DOCS_DIR}/INDEX.md"

mkdir -p "$DOCS_DIR"

echo "Fetching llms.txt..."
LLMS_CONTENT=$(curl -sS --fail "$LLMS_TXT_URL")

# Extract every markdown path from llms.txt entries of the form
#   - [Title](path.md): description
# Paths may or may not start with a leading slash; normalize so each entry
# is exactly one path with no leading slash (e.g. "guides/tools/nextjs.md").
PATHS=$(echo "$LLMS_CONTENT" \
    | grep -oP '\[[^\]]+\]\(\K/?[^)]+\.md(?=\))' \
    | sed 's|^/||' \
    | sort -u)

if [[ -z "$PATHS" ]]; then
    echo "No markdown paths found in llms.txt" >&2
    exit 1
fi

TOTAL=$(echo "$PATHS" | wc -l)
COUNT=0

echo "Found $TOTAL paths to fetch"
echo "---"

while IFS= read -r path; do
    COUNT=$((COUNT + 1))
    url="${DOCS_BASE_URL}/${path}"
    out="${DOCS_DIR}/${path}"

    echo "[$COUNT/$TOTAL] ${path}"
    mkdir -p "$(dirname "$out")"
    if curl -sS --fail -o "$out" "$url"; then
        echo "  -> saved"
    else
        echo "  -> FAILED (HTTP error)"
    fi
done <<< "$PATHS"

echo "---"
echo "Rebuilding INDEX.md..."

# Build a map of path -> "Title|Description" from llms.txt so the index
# preserves the human-friendly metadata from the source. Use perl for the
# tricky line parsing -- bash's =~ chokes on parentheses inside the pattern.
declare -A META_OF
while IFS=$'\t' read -r norm_path title desc; do
    [[ -z "$norm_path" ]] && continue
    META_OF["$norm_path"]="${title}|${desc}"
done < <(echo "$LLMS_CONTENT" | perl -ne '
    next unless /^\s*-\s*\[([^\]]+)\]\((\/?[^)]+\.md)\)(?::\s*(.*))?\s*$/;
    my ($title, $path, $desc) = ($1, $2, $3 // "");
    $path =~ s|^/||;
    print "$path\t$title\t$desc\n";
')

{
    echo "# Turborepo Docs Index"
    echo
    echo "Cached docs under \`docs/\`. Source: ${LLMS_TXT_URL}"
    echo "Last refreshed: $(date -Iseconds)"
    echo
    # List every cached file in path-sorted order, preserving directory layout.
    while IFS= read -r f; do
        rel="${f#${DOCS_DIR}/}"
        [[ "$rel" == "INDEX.md" ]] && continue
        meta="${META_OF[$rel]:-}"
        title="${meta%%|*}"
        desc="${meta#*|}"
        if [[ -n "$title" && -n "$desc" && "$desc" != "$meta" ]]; then
            printf -- "- \`%s\` -- **%s** -- %s\n" "$rel" "$title" "$desc"
        elif [[ -n "$title" ]]; then
            printf -- "- \`%s\` -- **%s**\n" "$rel" "$title"
        else
            # Fall back to the file's first H1 if llms.txt didn't have an entry.
            h1="$(awk '/^# / { sub(/^# /, ""); print; exit }' "$f")"
            if [[ -n "$h1" ]]; then
                printf -- "- \`%s\` -- **%s**\n" "$rel" "$h1"
            else
                printf -- "- \`%s\`\n" "$rel"
            fi
        fi
    done < <(find "$DOCS_DIR" -type f -name '*.md' | sort)
} > "$INDEX_FILE"

echo "Done. Files saved to $DOCS_DIR"
echo "Index written to $INDEX_FILE"
