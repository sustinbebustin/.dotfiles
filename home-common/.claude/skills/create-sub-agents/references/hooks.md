# Hooks In Subagents

Two ways to wire hooks for a subagent: in the subagent's frontmatter, or in `settings.json` for the main session.

## Frontmatter Hooks (Subagent-Scoped)

Defined in the subagent's markdown file. Run only while THAT specific subagent is active. Cleaned up when it finishes.

All [hook events](https://code.claude.com/docs/en/hooks#hook-events) are supported. Common ones for subagents:

| Event | Matcher input | When it fires |
| --- | --- | --- |
| `PreToolUse` | Tool name | Before the subagent uses a tool |
| `PostToolUse` | Tool name | After the subagent uses a tool |
| `Stop` | (none) | When the subagent finishes (auto-converted to `SubagentStop`) |

```yaml
---
name: code-reviewer
description: Review code changes with automatic linting
hooks:
  PreToolUse:
    - matcher: "Bash"
      hooks:
        - type: command
          command: "./scripts/validate-command.sh $TOOL_INPUT"
  PostToolUse:
    - matcher: "Edit|Write"
      hooks:
        - type: command
          command: "./scripts/run-linter.sh"
---
```

When the agent runs as the MAIN SESSION (via `--agent` or the `agent` setting), frontmatter hooks fire ALONGSIDE hooks in `settings.json`.

When it runs as a SUBAGENT, `Stop` hooks in frontmatter are auto-converted to `SubagentStop` events.

Plugin subagents IGNORE the `hooks` field.

## Project-Level Hooks (Main Session)

Configure in `settings.json` to react to subagent lifecycle events from the parent's perspective.

| Event | Matcher input | When it fires |
| --- | --- | --- |
| `SubagentStart` | Agent type name | When a subagent begins execution |
| `SubagentStop` | Agent type name | When a subagent completes |

Both events support matchers to target specific agent types by name.

```json
{
  "hooks": {
    "SubagentStart": [
      {
        "matcher": "db-agent",
        "hooks": [
          { "type": "command", "command": "./scripts/setup-db-connection.sh" }
        ]
      }
    ],
    "SubagentStop": [
      {
        "hooks": [
          { "type": "command", "command": "./scripts/cleanup-db-connection.sh" }
        ]
      }
    ]
  }
}
```

## Conditional Validation Pattern

`PreToolUse` is the right hook for "allow some uses of a tool but not others". Common case: read-only DB access.

```yaml
---
name: db-reader
description: Execute read-only database queries
tools: Bash
hooks:
  PreToolUse:
    - matcher: "Bash"
      hooks:
        - type: command
          command: "./scripts/validate-readonly-query.sh"
---
```

The validation script reads JSON from stdin and exits with code 2 to block:

```bash
#!/bin/bash
INPUT=$(cat)
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

# Block SQL write operations (case-insensitive)
if echo "$COMMAND" | grep -iE '\b(INSERT|UPDATE|DELETE|DROP|CREATE|ALTER|TRUNCATE)\b' > /dev/null; then
  echo "Blocked: Only SELECT queries are allowed" >&2
  exit 2
fi

exit 0
```

Make the script executable on macOS/Linux:

```bash
chmod +x ./scripts/validate-readonly-query.sh
```

On Windows, write the validation script in PowerShell and add `shell: powershell` to the hook entry.

## Hook Output

- Exit 0 -> allow
- Exit 2 -> BLOCK and feed stderr back to Claude
- Other non-zero exits -> log error but allow

`PreToolUse` and `PostToolUse` both receive JSON via stdin. The full schema is at [hooks.md](https://code.claude.com/docs/en/hooks#pretooluse-input).

## When Hooks Beat System-Prompt Instructions

A line like "never delete files" in the subagent body is a request - the model can ignore it. A `PreToolUse` hook that exits 2 on `rm` is enforcement. Use hooks for:

- Hard guardrails (no writes to `.env`, no SQL writes, no network calls to certain hosts)
- Mandatory side effects (run linter after every Edit)
- Audit logging
