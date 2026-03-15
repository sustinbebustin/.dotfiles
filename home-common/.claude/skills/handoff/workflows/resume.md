# Resume Handoff Workflow

Resume work from a previous session handoff to maintain continuity.

## When to Use

- Starting a new session
- Picking up where you left off
- After a break, restart, or `/clear`
- Taking over from another person/agent
- Recovering context after memory pressure

## Step 1: Discover Handoff

Find available handoffs from the database:

```bash
# List recent handoffs
continuity handoff list

# List more handoffs
continuity handoff list --limit 10

# Search handoffs by content
continuity search "feature name" --type handoff
```

## Step 2: Load Handoff Content

Use the resume command to load handoff content:

```bash
# Resume from most recent handoff
continuity handoff resume

# Resume from specific handoff by ID (full or partial)
continuity handoff resume abc123

# Get JSON output for programmatic use
continuity handoff resume --format json
```

Alternatively, view details without resuming:

```bash
# Show handoff details
continuity handoff show abc123
```

### Key Fields to Understand

| Field | Action |
|-------|--------|
| `goal` | Understand overall objective |
| `now` | Your immediate first action |
| `test` | How to verify correctness |
| `summary` | Context from previous session |

## Step 3: Review Relevant Files

If the handoff mentions specific files, review them before starting work.

## Step 4: Execute the `now` Action

The `now` field is your immediate first action. Execute it before anything else.

### What Makes a Good `now` Field

| Good | Bad |
|------|-----|
| "Fix token expiry check in auth.py line 45 - condition is inverted" | "Continue working on auth" |
| "Add validation for email field in user_form.py:UserForm.clean()" | "Finish the form" |
| "Run pytest tests/test_auth.py to reproduce the failing refresh test" | "Debug the tests" |

### If `now` Is Unclear

1. Check the summary for additional context
2. Look for file references in the handoff content
3. Use the `test` command to understand expected behavior
4. Check the ledger for current state: `continuity ledger show`

## Step 5: Run Test Verification

After completing `now`, run the `test` command:

| Result | Action |
|--------|--------|
| All pass | Proceed to next items |
| Some fail | Focus on failures before moving on |
| All fail | Review `now` action, check approach |
| Test error | Check environment, dependencies |

## Ledger Integration

For live session state, check the active ledger:

```bash
# Show current ledger
continuity ledger show

# List recent ledgers
continuity ledger list
```

The ledger contains:
- Current goal
- What you should be doing now
- Test command to verify progress
- Current branch

After resuming, update the ledger if needed:

```bash
continuity ledger update --now "Next action to take"
```

## Example Resume Workflows

### Feature Development

```bash
$ continuity handoff resume
ID: abc12345
Type: session
Goal: Add export to CSV functionality
Now: Implement date range filter in export.py line 42 - skeleton exists
Test: Run pytest tests/test_export.py - all 8 tests should pass
Summary: Created export module structure, implemented base CSV writer
```

**Resume:**
1. Review `goal` - CSV export
2. Review `summary` - Module exists, writer works
3. Execute `now` - Go to export.py line 42, implement filter
4. Run `test` - Execute pytest
5. If tests pass, update ledger with next action

### Bug Fix

```bash
$ continuity handoff resume
ID: def67890
Type: session
Goal: Fix memory leak in WebSocket handler
Now: Apply connection cleanup fix to ws_handler.py line 89
Test: Run python scripts/ws_load_test.py - memory under 500MB
Summary: Leak only occurs during handshake disconnect. Initial fix broke reconnection logic - avoid that approach.
```

**Resume:**
1. Review `goal` - Fix memory leak
2. Note the summary - Don't break reconnection logic
3. Read ws_handler.py, focus on line 89
4. Execute `now` - Apply cleanup fix carefully
5. Run `test` - Check memory stays under 500MB

## Clear vs Compact

| Situation | Action |
|-----------|--------|
| Quick task, < 50% context | Let it compact |
| Long task, > 70% context | Update ledger, then /clear |
| Handing off to another session | Create handoff |
| Multi-day implementation | Use both ledger + periodic handoffs |

**Why clear instead of compact?** Each compaction is lossy. Clearing + loading the ledger gives fresh context with full signal.

## Resume Response Template

After loading a handoff, respond:

```
Resumed from handoff: {id}

Goal: {goal}
First action: {now}
Verify with: {test}

Summary: {summary}

Reviewing files and executing first action...
```

## Quick Reference

### Resume Commands

```bash
# Resume most recent handoff
continuity handoff resume

# Resume specific handoff
continuity handoff resume <id>

# List available handoffs
continuity handoff list

# Show handoff details
continuity handoff show <id>

# Check current ledger
continuity ledger show
```

### Resume Checklist

- [ ] List available handoffs: `continuity handoff list`
- [ ] Resume the handoff: `continuity handoff resume`
- [ ] Note the `goal`
- [ ] Review `summary` for context
- [ ] Execute the `now` action
- [ ] Run `test` to verify
- [ ] Update ledger with next action
