# Search Tools Guide

Choose the right search tool for each task. Each tool has a distinct purpose.

## Tool Selection

| Tool | Use When | Query Type |
|------|----------|------------|
| `continuity search` | Exploring unfamiliar code by concept | Natural language |
| `ast-grep` | Finding exact syntax patterns | Code structure |
| `rg` / `grep` | Exact text matches, non-code files | Literal strings |

## 1. Semantic Search (Discovery)

Use first when exploring code you haven't examined yet.

```bash
continuity search "authentication flow" --type code
continuity search "error handling" --type code --language python
continuity search "database migrations" --type code -x  # exclude tests
continuity search "session persistence"                  # FTS for sessions/decisions
```

Best for: "How does X work?", finding related implementations, conceptual exploration

## 2. ast-grep (Structural Precision)

Use when you know the exact syntax pattern you need.

```bash
# Pattern syntax: $VAR (single), $$$VAR (variadic), $_ (anonymous)
ast-grep --pattern 'def $FUNC($$$ARGS)' --lang python src/
ast-grep --pattern 'async def $FUNC($$$ARGS)' --lang python src/
ast-grep --pattern '@$DECORATOR' --lang python src/
ast-grep --pattern 'class $NAME($$$BASES):' --lang python src/
ast-grep --pattern 'raise $EXC($$$MSG)' --lang python src/
ast-grep --pattern '$OBJ.$METHOD($$$ARGS)' --lang python src/
```

Best for: Finding all usages of an API, refactoring, precise structural matches

## 3. ripgrep (Text Search)

Use for exact strings, config files, or when semantic/AST search is unavailable.

```bash
rg -i 'search term' --type md    # Documentation
rg 'CONFIG_KEY' --type yaml      # Config files
rg -l 'pattern' src/             # List matching files
```

Best for: Exact strings, non-code files, fallback when other tools unavailable

## Priority Order

1. Semantic search for conceptual exploration
2. ast-grep for known structural patterns
3. ripgrep for exact text matches

## DON'T

```bash
# Don't use text search for code exploration
rg "authentication" src/           # Use semantic search instead
grep -R "parse config" src/        # Use semantic search instead

# Don't use regex for structural patterns
rg "def.*authenticate" src/        # Use ast-grep instead
```
