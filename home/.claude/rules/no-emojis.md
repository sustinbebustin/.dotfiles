# No Emojis

Never use emojis in code, documentation, comments, commit messages, or any project files.
Use text-based alternatives that render consistently across all environments.

## Professional Replacements

### Status Indicators

| Instead of | Use |
|------------|-----|
| Checkmark emoji | `[DONE]`, `[PASS]`, `[x]`, `(done)` |
| X/cross emoji | `[FAIL]`, `[ERROR]`, `[BLOCKED]`, `[ ]` |
| Warning emoji | `[WARN]`, `WARNING:`, `[!]` |
| Info emoji | `[INFO]`, `NOTE:`, `[i]` |

### Section Markers

| Instead of | Use |
|------------|-----|
| Rocket/sparkle | `NEW:`, `FEATURE:`, or omit entirely |
| Lightbulb | `TIP:`, `HINT:` |
| Target | `GOAL:`, `TARGET:` |
| Fire/hot | `IMPORTANT:`, `CRITICAL:` |

### Visual Separators

| Instead of | Use |
|------------|-----|
| Decorative emojis | `---`, `===`, `***` |
| Bullet emojis | `-`, `*`, `+` |
| Arrow emojis | `->`, `-->`, `=>` |

## DO

```markdown
## Features

- Session persistence with SQLite
- Semantic code search via embeddings
- [DONE] CLI installation script

NOTE: Requires Python 3.11+
```

```python
# WARNING: This function modifies global state
# TODO: Refactor to use dependency injection
```

## DON'T

```markdown
## Features

- Session persistence with SQLite
- Semantic code search via embeddings
- Done CLI installation script

Note: Requires Python 3.11+
```

## Why

- Emojis render inconsistently across terminals, editors, and fonts
- Plain text is more accessible and grep-friendly
- Professional documentation standards favor text-based formatting
- Version control diffs are cleaner without Unicode symbols
