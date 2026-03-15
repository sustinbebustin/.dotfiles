# Create Handoff Workflow

Create a session handoff to preserve context for the next session. Handoffs are stored directly in the database.

## When to Use

- Ending a work session
- Before `/clear` (when you want a full snapshot)
- Handing off to another person/agent
- Checkpointing complex work
- Before a break

## Required Fields

Every handoff **must** include:

| Field | Description | Example |
|-------|-------------|---------|
| `goal` | Overall objective | "Implement user authentication with OAuth2" |
| `now` | Immediate next action (specific!) | "Fix token refresh in auth.py line 45" |

## Optional Fields

| Field | Description |
|-------|-------------|
| `summary` | Summary of work done so far |
| `test` | Verification command to run |

## CLI Command

Use the `continuity handoff create` command to store handoffs directly in the database:

```bash
continuity handoff create --goal "..." --now "..." [--summary "..."] [--test "..."]
```

### Options

| Option | Short | Description | Required |
|--------|-------|-------------|----------|
| `--goal` | `-g` | Session goal (what you're working toward) | Yes |
| `--now` | `-n` | Current focus (what you're doing right now) | Yes |
| `--summary` | `-s` | Summary of work done so far | No |
| `--test` | `-t` | Test command to verify progress | No |

## Examples

### Minimal Handoff

```bash
continuity handoff create \
  --goal "Implement user authentication with OAuth2" \
  --now "Fix the token refresh logic in auth.py line 45"
```

### Standard Handoff

```bash
continuity handoff create \
  --goal "Implement user authentication with OAuth2" \
  --now "Fix the token refresh logic in auth.py line 45" \
  --summary "Implemented OAuth2 flow, added JWT validation, created login endpoints" \
  --test "pytest tests/test_auth.py - all 12 tests should pass"
```

### Short Form

```bash
continuity handoff create -g "Fix bug" -n "Debugging auth issue" -s "Found root cause" -t "pytest"
```

## Best Practices

1. **Make `now` actionable** - Include file and line number
   - Good: "Fix token expiry check in auth.py line 45 - condition is inverted"
   - Bad: "Continue working on auth"

2. **Make `test` verifiable** - Exact command to run
   - Good: "pytest tests/test_auth.py - all 12 tests should pass"
   - Bad: "Make sure it works"

3. **Be specific in summary** - List key accomplishments
   - Good: "Added OAuth2 flow, JWT validation, login/callback endpoints"
   - Bad: "Did some auth work"

4. **Include context in goal** - Future sessions need orientation
   - Good: "Implement OAuth2 with PKCE for mobile auth"
   - Bad: "Auth stuff"

## After Creating

The handoff is stored in the database and can be retrieved with:

```bash
# List recent handoffs
continuity handoff list

# Resume from a handoff (loads for context injection)
continuity handoff resume

# Show details of a specific handoff
continuity handoff show <id>
```

Respond to the user with a summary:

```
Handoff created and stored in database.

Goal: {goal}
Next action: {now}
Verify with: {test}

Ready for next session to resume with: continuity handoff resume
```
