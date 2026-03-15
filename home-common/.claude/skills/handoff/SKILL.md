---
name: handoff
description: Session continuity management - create handoffs, resume from handoffs, and maintain ledger state. Use when ending sessions, starting new sessions, before /clear, or managing context across sessions.
user_invocable: true
---

# Handoff

Unified session continuity skill for managing context across Claude Code sessions.

## Quick Start

**Determine which workflow applies:**

| User Intent | Workflow | File |
|-------------|----------|------|
| Ending session, wrapping up, done for today | **Create** | [create.md](workflows/create.md) |
| Starting session, resuming work, picking up | **Resume** | [resume.md](workflows/resume.md) |
| Before /clear, context high, saving state | **Ledger** | [ledger.md](workflows/ledger.md) |

## Workflow Selection

Choose the appropriate workflow based on the user's intent:

### Create Workflow

Use when the user wants to:
- End a work session
- Hand off work to another session/person
- Create a checkpoint before stopping
- Wrap up for the day
- Create a snapshot of current state

**Trigger phrases:** "create handoff", "done for today", "end session", "wrap up", "hand off", "pick this up later"

**Action:** Follow [create.md](workflows/create.md)

### Resume Workflow

Use when the user wants to:
- Start a new session with prior context
- Continue where they left off
- Resume after a break or restart
- Take over work from another person/agent
- Recover context after /clear

**Trigger phrases:** "resume handoff", "continue work", "pick up where", "new session", "where did I leave off"

**Action:** Follow [resume.md](workflows/resume.md)

### Ledger Workflow

Use when the user wants to:
- Update live session state
- Prepare for /clear
- Save progress without ending session
- Context usage is high (70%+)
- Multi-day implementation tracking

**Trigger phrases:** "update ledger", "save state", "before clear", "high context", "context usage", "70%"

**Action:** Follow [ledger.md](workflows/ledger.md)

## Storage

All handoffs and ledgers are stored in the **project database** (`.continuity/database/project.db`).

- **Ledgers**: Stored in the `ledgers` table with fields for goal, now, test, branch, and timestamps
- **Handoffs**: Stored in the `handoffs` table with full JSON content in the `content` column
- **Search**: Full-text search (FTS5) is automatically indexed for both ledgers and handoffs

No files are created in the filesystem. All data persists across sessions via SQLite.

## CLI Commands

### Ledger Commands

```bash
# Create a new ledger (or update existing)
continuity ledger create --goal "Implement feature X" --now "Working on component Y"

# Update the active ledger
continuity ledger update --now "Finished component Y, starting Z"
continuity ledger update --goal "New goal" --test "Run pytest tests/"

# Show the active ledger
continuity ledger show

# Show a specific ledger by ID
continuity ledger show --id abc123

# List recent ledgers
continuity ledger list
continuity ledger list --limit 5

# Get ledger as JSON (for hooks/scripts)
continuity ledger find --format json
```

### Handoff Commands

```bash
# Create a handoff
continuity handoff create --goal "Feature X" --now "Completed step 1"

# Resume from most recent handoff
continuity handoff resume

# Resume from a specific handoff
continuity handoff resume abc123

# Show handoff details
continuity handoff show <id>

# List recent handoffs
continuity handoff list
continuity handoff list --limit 10

# Search handoffs
continuity search "query"
```

## Workflow Comparison

| Aspect | Create | Resume | Ledger |
|--------|--------|--------|--------|
| **When** | End of session | Start of session | During session |
| **Purpose** | Snapshot for transfer | Load prior context | Live state tracking |
| **Frequency** | Once per transfer | Once per session start | Many times per session |
| **Storage** | Database (handoffs table) | Reads from database | Database (ledgers table) |

## Integration

These workflows work together:

```
Session A                      Session B
──────────────────────────────────────────────────
Work on task
    │
    ├── /handoff (ledger)     # Update state during work
    │       │
    │       └── continuity ledger update --now "..."
    │
    ├── /handoff (ledger)     # Before /clear
    │
    └── /handoff (create)     # End of session
            │
            │   stores in database
            ▼
    .continuity/database/project.db
            │
            │   reads from database
            ▼
                               /handoff (resume)
                                     │
                                     └── continuity handoff resume
                                               │
                                               ▼
                                         Continue work
                                               │
                                               └── ...repeat
```

## Best Practices

1. **Use ledger during long sessions** - Update frequently as state changes
2. **Create handoff at session end** - Don't lose context between sessions
3. **Resume at session start** - Load context before starting work
4. **Clear > compact** - Fresh context with ledger beats degraded compacted context
5. **One "now" item** - Forces focus, prevents sprawl
6. **Be specific** - Include file paths, line numbers, exact commands

## Related

- `continuity context` - Synthesize context from all sources
- `continuity search` - Search sessions, decisions, handoffs
- `continuity record` - Record decisions and events
