## Git Conventions

Combine **atomic commits** (one logical change per commit) with **conventional commits** (a standard message format), on branches named for the change they carry.

### Atomic Commits

Each commit should:
- Contain **exactly one logical change**
- Be **independently revertable** without breaking other functionality
- Leave the codebase in a **working state**
- Be as **small as possible** while remaining complete

#### When to Split Commits

| Situation | Action |
|-----------|--------|
| New utility + feature using it | Two commits: utility first, then feature |
| Bug fix discovered while working on feature | Separate commit for the fix |
| Refactor + behavior change | Refactor first, behavior change second |
| Multiple unrelated file changes | One commit per logical change |
| Formatting/linting + code changes | Formatting commit first |

#### When to Keep Together

- Changes that only make sense together (e.g., function + its tests)
- Rename/move that touches many files but is one logical operation
- Config changes required by the code change

### Conventional Commit Format

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

#### Types

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

#### Scope

Optional, indicates the area affected: `feat(auth)`, `fix(api)`, `docs(readme)`

#### Description

- Imperative mood: "add" not "added" or "adds"
- Lowercase, no period
- Under 50 characters
- Reads as a user-facing sentence: release tooling (Release Please, GitHub auto-notes) quotes subjects verbatim

#### Body

- Wrap at 72 characters
- Explain **what** and **why**, not how
- Separate from subject with blank line

#### Footer

- `BREAKING CHANGE: <description>` for breaking changes, plus `!` after the type/scope
- `Closes #123` or `Fixes #456` for issue references

#### Examples

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
```
feat(auth)!: require API key for all endpoints

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

### Branch Naming

```
<type>/<short-description>
```

- Type: one of the commit types above, matching the dominant change on the branch (the same type the PR title uses)
- Description: kebab-case, imperative, 2-4 words naming the change
- Optional issue reference as a suffix: `fix/null-panel-calc-892`

Examples: `feat/signwell-webhook`, `fix/auth-redirect-loop`, `chore/bump-eslint`, `docs/api-auth-guide`

A name like `patch-1`, `wip`, `my-branch`, or `update` says nothing about the change; name the change instead.

### Authorship

Write commit messages as if the user authored them: no AI attribution, no `Co-Authored-By` or "Generated with" trailers.
