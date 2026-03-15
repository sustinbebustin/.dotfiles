# Ledger Workflow

Maintain a ledger that survives `/clear` for long-running sessions. The ledger is stored in the database and automatically loaded by the SessionStart hook after a `/clear`.

## When to Use

- Before running `/clear`
- Context usage approaching 70%+
- Multi-day implementations
- Complex refactors you pick up/put down
- Any session expected to hit 85%+ context

## When NOT to Use

- Quick tasks (< 30 min)
- Simple bug fixes
- Single-file changes

## Why Clear Instead of Compact?

Each compaction is lossy compression. After several compactions, you're working with degraded context. Clearing + loading the ledger gives you fresh context with full signal.

## Process

### 1. Check for Existing Ledger

Check if a ledger already exists for the current project:

```bash
continuity ledger show
```

- **If exists**: Update the ledger using `continuity ledger update`
- **If not**: Create a new ledger using `continuity ledger create`

### 2. Create a New Ledger

Create a ledger with your goal and current focus:

```bash
continuity ledger create --goal "Replace JWT with session auth" --now "Implementing logout endpoint"
```

Optional fields:
```bash
continuity ledger create \
  --goal "Replace JWT with session auth" \
  --now "Implementing logout endpoint" \
  --test "uv run pytest tests/ -v" \
  --branch "feature/session-auth"
```

### 3. Ledger Fields

| Field | Required | Description |
|-------|----------|-------------|
| `goal` | Yes | One-liner success criteria |
| `now` | Yes | Current focus (ONE thing only) |
| `test` | No | Test command to run |
| `branch` | No | Git branch for this work |

The database automatically tracks:
- `id`: Unique identifier for the ledger
- `created_at`: When the ledger was created
- `updated_at`: Last modification timestamp
- `is_active`: Whether this is the current active ledger

### 4. Update Guidelines

**When to update:**
- Session start: Read and refresh
- After major decisions
- Before `/clear`
- At natural breakpoints
- When context usage >70%

**What to update:**
- Update "now" with current focus (ONE item only)
- Update "goal" if scope has changed
- **Updates automatically refresh the timestamp**

### 5. After Clear Recovery

When resuming after `/clear`:

1. Ledger loads automatically (SessionStart hook extracts it from database)
2. Review the goal and current focus
3. Ask 1-3 targeted questions to validate assumptions
4. Update ledger with clarifications
5. Continue work with fresh context

## CLI Commands

```bash
# Create a new ledger
continuity ledger create --goal "Goal description" --now "Current focus"

# Create with all fields
continuity ledger create --goal "..." --now "..." --test "npm test" --branch "feature/x"

# Update ledger fields
continuity ledger update --now "New current focus"
continuity ledger update --goal "Updated goal" --now "New focus"

# Show current ledger state
continuity ledger show

# Show a specific ledger by ID
continuity ledger show --id abc123

# List recent ledgers
continuity ledger list
continuity ledger list --limit 5

# Get ledger as JSON (used by hooks)
continuity ledger find --format json
```

## Example

Creating a ledger for an auth refactor:

```bash
# Create the ledger
continuity ledger create \
  --goal "Replace JWT auth with session-based auth. Done when all tests pass and no JWT imports remain." \
  --now "Logout endpoint and session invalidation" \
  --test "npm test -- --grep session" \
  --branch "feature/session-auth"
```

Updating progress:

```bash
# Update current focus after completing login endpoint
continuity ledger update --now "Middleware swap and JWT removal"
```

Viewing ledger state:

```bash
# Show current ledger
continuity ledger show
```

Output:
```
ID: a1b2c3d4-5678-90ab-cdef-1234567890ab
Goal: Replace JWT auth with session-based auth
Now: Middleware swap and JWT removal
Test: npm test -- --grep session
Branch: feature/session-auth
Updated: 2026-01-15T14:30:00Z
Status: active
```

## Response Template

After creating/updating the ledger:

```
Ledger updated.

Current state:
- Goal: <one-liner success criteria>
- Now: <current focus>
- Test: <test command if set>
- Branch: <branch if set>

Ready for /clear - ledger will reload on resume.
```

## Comparison with Other Tools

| Tool | Scope | Storage |
|------|-------|---------|
| CLAUDE.md | Project | File (always fresh, stable patterns) |
| TodoWrite | Turn | Memory (survives compaction, degrades) |
| Ledger | Session | Database (survives /clear, never compressed) |

## Key Points

- **Keep it concise** - Brevity matters for context
- **One "now" item** - Forces focus, prevents sprawl
- **Update frequently** - Stale ledgers lose value quickly
- **Clear > compact** - Fresh context beats degraded context
- **Database storage** - Ledger persists in `.continuity/database/project.db`
