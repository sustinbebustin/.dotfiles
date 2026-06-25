---
name: scrape-docs
description: Generate and run a script that mirrors a documentation site (from an llms.txt or sitemap.xml) into a local directory as markdown, preserving URL path structure.
argument-hint: "<llms.txt-or-sitemap-url> <destination-dir>"
disable-model-invocation: true
allowed-tools: Bash, Read, Write, Edit, WebFetch
---

# Scrape Docs

Given a source index URL (llms.txt or sitemap.xml) and a destination directory, generate a self-contained bash script that downloads every page as markdown and mirrors the URL path structure on disk. Then run it.

## Inputs

Parse `$ARGUMENTS` as two whitespace-separated tokens:
- `$1` — source URL (llms.txt, sitemap.xml, or similar index)
- `$2` — destination directory (absolute path preferred)

If either is missing or ambiguous, ask the user before proceeding.

## Procedure

1. **Inspect the index.** Fetch the source URL. Determine the format (llms.txt = plain markdown links, sitemap.xml = `<loc>` tags). Note the URL pattern of the doc pages (e.g. all share a common path prefix like `/docs/14/`).

2. **Probe for a markdown variant.** Pick one sample doc URL and `curl -sI` three candidates in this order:
   - `<url>.md`
   - `<url>.mdx`
   - `<url>` (raw HTML, last resort)

   Check `content-type` on the 200 response. `text/markdown` is the win condition. Stop at the first match. If only HTML is available, tell the user and ask whether to proceed with HTML (no conversion) or abort so they can pick a tool like `pandoc` or `html-to-markdown`.

3. **Write `scrape.sh` into the destination directory.** Use the template below as the starting point. Adjust:
   - `INDEX_URL` — the source URL
   - The URL filter `grep -oE` — match the doc-page pattern found in step 1 and exclude the index file itself
   - The fetch URL inside `fetch_one` — append `.md` / `.mdx` / leave bare per step 2
   - The path-stripping prefix in `rel=...` — trim the shared URL prefix so on-disk paths mirror the URL structure cleanly

4. **Make it executable and run it.** Show the user a summary (page count, output path, sample file).

5. **Verify.** `find` the output dir, count `.md` files, `head` one to confirm content. Report any failures from the script's stderr.

## Script template

```bash
#!/usr/bin/env bash
# Scrape docs from <INDEX_URL> into ./docs, mirroring URL paths.
set -euo pipefail

INDEX_URL="<FILL>"
OUT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/docs"
CONCURRENCY="${CONCURRENCY:-8}"

mkdir -p "$OUT_DIR"

# llms.txt: extract https URLs directly.
# sitemap.xml: extract from <loc>...</loc>.
mapfile -t urls < <(
  curl -fsSL "$INDEX_URL" \
    | grep -oE '<URL-PATTERN-REGEX>' \
    | grep -v '<INDEX-EXCLUSION>' \
    | sort -u
)

echo "Found ${#urls[@]} pages. Writing to $OUT_DIR (concurrency=$CONCURRENCY)"

fetch_one() {
  local url="$1"
  local rel="${url#<COMMON-URL-PREFIX>}"
  local out="$OUT_DIR/${rel}.md"
  mkdir -p "$(dirname "$out")"
  if curl -fsSL --retry 3 --retry-delay 1 "${url}<.md-or-empty>" -o "$out"; then
    echo "  ok   $rel"
  else
    echo "  FAIL $rel" >&2
    rm -f "$out"
  fi
}
export -f fetch_one
export OUT_DIR

printf '%s\n' "${urls[@]}" | xargs -P "$CONCURRENCY" -I{} bash -c 'fetch_one "$@"' _ {}

echo "Done. $(find "$OUT_DIR" -name '*.md' | wc -l) markdown files in $OUT_DIR"
```

For sitemaps, replace the URL extraction with:

```bash
mapfile -t urls < <(
  curl -fsSL "$INDEX_URL" \
    | grep -oE '<loc>[^<]+</loc>' \
    | sed -E 's#</?loc>##g' \
    | grep -E '<URL-FILTER>' \
    | sort -u
)
```

## Notes

- Keep the script self-contained and re-runnable. The user re-runs it to refresh content.
- Place `scrape.sh` *inside* the destination dir alongside the `docs/` subfolder it produces. Don't litter the parent.
- Default concurrency 8 is safe for most sites. Mention `CONCURRENCY=N ./scrape.sh` as the knob.
