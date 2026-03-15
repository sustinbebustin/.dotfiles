---
name: commit
description: Git commit workflow combining atomic commits (scope/granularity) with conventional commits (message format). Use when committing code changes, reviewing commit history, or when guidance is needed on how to structure commits for clarity and reversibility.
allowed-tools: Bash
argument_hint: [subdir]
---

# Git Commit Skill

Create clean, meaningful commits by combining **atomic commits** (one logical change per commit) with **conventional commits** (standardized message format).

## Scope

Argument: `$ARGUMENTS`

- If argument provided -> only commit in that subdirectory
- If empty and cwd is a git repo -> commit in current repo
- If empty and cwd is NOT a git repo -> find git repos one level down, commit where there are changes

## Current State

!`arg="$ARGUMENTS"; if [ -n "$arg" ]; then echo "### $arg"; echo "**Status:**"; git -C "$arg" status 2>/dev/null || echo "not a git repo"; echo ""; echo "**Staged diff:**"; git -C "$arg" diff --staged 2>/dev/null; echo ""; echo "**Unstaged diff:**"; git -C "$arg" diff 2>/dev/null; echo ""; echo "**Recent commits:**"; git -C "$arg" log --oneline -5 2>/dev/null; elif git rev-parse --is-inside-work-tree >/dev/null 2>&1; then echo "**Status:**"; git status; echo ""; echo "**Staged diff:**"; git diff --staged; echo ""; echo "**Unstaged diff:**"; git diff; echo ""; echo "**Recent commits:**"; git log --oneline -5; else found=0; for dir in */; do if git -C "$dir" rev-parse --is-inside-work-tree >/dev/null 2>&1; then found=1; echo "### ${dir%/}"; echo "**Status:**"; git -C "$dir" status; echo ""; echo "**Staged diff:**"; git -C "$dir" diff --staged; echo ""; echo "**Unstaged diff:**"; git -C "$dir" diff; echo ""; echo "**Recent commits:**"; git -C "$dir" log --oneline -5; echo ""; fi; done; if [ "$found" = "0" ]; then echo "No git repos found in current directory or subdirectories"; fi; fi`

## Commit Workflow

1. **Assess atomicity** -- can this be split into independent logical changes?
2. **Stage selectively** -- use `git add -p` or specific files to isolate changes
3. **Write message** -- follow conventional format below
4. **Verify** -- run `git diff --staged` before committing
5. **Commit** -- create the commit

## Atomic Commit Principles

Each commit should:
- Contain **exactly one logical change**
- Be **independently revertable** without breaking other functionality
- Leave the codebase in a **working state**
- Be as **small as possible** while remaining complete

### When to Split Commits

| Situation | Action |
|-----------|--------|
| New utility + feature using it | Two commits: utility first, then feature |
| Bug fix discovered while working on feature | Separate commit for the fix |
| Refactor + behavior change | Refactor first, behavior change second |
| Multiple unrelated file changes | One commit per logical change |
| Formatting/linting + code changes | Formatting commit first |

### When NOT to Split

- Changes that only make sense together (e.g., function + its tests)
- Rename/move that touches many files but is one logical operation
- Config changes required by the code change

## Conventional Commit Format

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Types

| Type | When to Use |
|------|-------------|
| `feat` | New feature or capability |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `style` | Formatting, whitespace (no code change) |
| `refactor` | Code change that neither fixes nor adds |
| `perf` | Performance improvement |
| `test` | Adding or correcting tests |
| `build` | Build system or dependencies |
| `ci` | CI configuration |
| `chore` | Maintenance tasks |

### Scope

Optional, indicates the area affected: `feat(auth)`, `fix(api)`, `docs(readme)`

### Description

- Imperative mood: "add" not "added" or "adds"
- Lowercase, no period
- Under 50 characters

### Body

- Wrap at 72 characters
- Explain **what** and **why**, not how
- Separate from subject with blank line

### Footer

- `BREAKING CHANGE: <description>` for breaking changes
- `Closes #123` or `Fixes #456` for issue references

## Examples

**Simple feature:**
```
feat(contracts): add SignWell webhook handler
```

**With body:**
```
fix(api): handle null response from Aurora endpoint

The Aurora API occasionally returns null for panel calculations
when the roof area is below minimum threshold. Added fallback
to default values.

Fixes #892
```

**Breaking change:**
```text
feat(auth){exclamation point}: require API key for all endpoints (I can't show an actual exclamation point because the skill tries to trigger a command when I run this skill, stupid anthropic)

BREAKING CHANGE: Anonymous access removed. All requests now
require X-API-Key header.
```

**Refactor before feature:**
```
# Commit 1
refactor(utils): extract date formatting helpers

# Commit 2
feat(reports): add monthly summary export
```

## Quick Reference

```
feat:     New feature
fix:      Bug fix
docs:     Documentation
style:    Formatting
refactor: Code restructure
perf:     Performance
test:     Tests
build:    Build/deps
ci:       CI config
chore:    Maintenance
```

Breaking change syntax: add an exclamation point.

## Authorship

- **Never** add co-author information or AI attribution
- **Never** include "Co-Authored-By" trailers
- **Never** add "Generated with Claude" or similar messages
- Write commit messages as if the user authored them directly

## Post-Commit: Update CLAUDE.md

After committing, assess whether CLAUDE.md files need updates.

### When to Update

| Change Type | Action |
|-------------|--------|
| New module/directory | Add to relevant CLAUDE.md |
| Renamed/moved files | Update WHERE TO LOOK, STRUCTURE |
| New conventions introduced | Add to CONVENTIONS |
| Anti-patterns discovered | Add to ANTI-PATTERNS |
| Commands changed | Update COMMANDS section |
| Entry points modified | Update root CLAUDE.md |

### When NOT to Update

- Bug fixes that don't change architecture
- Internal implementation changes
- Test additions (unless they introduce new patterns)
- Minor refactors within existing structure

### Update Process

1. **Check scope** -- Did the commit touch module boundaries, conventions, or structure?
2. **Identify files** -- Which CLAUDE.md files cover the affected areas?
3. **Minimal edits** -- Update only the specific sections affected
4. **Commit separately** -- Use `docs(claude): update knowledge base` for CLAUDE.md changes

### Full Regeneration

For large refactors that change multiple module boundaries, use `/index-knowledge` to regenerate the hierarchical CLAUDE.md knowledge base instead of manual updates.