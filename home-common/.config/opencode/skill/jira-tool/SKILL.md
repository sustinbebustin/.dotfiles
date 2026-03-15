---
name: jira-tool
description: Create, update, transition, and manage Jira tickets. Use when working with Jira issues, tracking work, or automating ticket workflows.
---

# Jira Tool

CLI for Jira Cloud operations via API key auth.

## Quick Reference

| Task | Command |
|------|---------|
| Create ticket | `scripts/jira-tool.sh create -p PROJECT -s "Summary" [-t Type] [-d "Desc"] [-l labels]` |
| Get ticket | `scripts/jira-tool.sh get ISSUE-KEY` |
| Update ticket | `scripts/jira-tool.sh update ISSUE-KEY [-s "Summary"] [-d "Desc"] [-a accountId]` |
| Add comment | `scripts/jira-tool.sh comment ISSUE-KEY "Comment body"` |
| Transition | `scripts/jira-tool.sh transition ISSUE-KEY [STATUS]` |
| Close ticket | `scripts/jira-tool.sh close ISSUE-KEY` |
| Assign | `scripts/jira-tool.sh assign ISSUE-KEY [accountId]` |
| Search | `scripts/jira-tool.sh search "JQL query" [max]` |
| Check creds | `scripts/jira-tool.sh status` |

## Authentication

Requires env vars (see `scripts/.env.example`):

```bash
export JIRA_EMAIL="you@lgcy.network"
export JIRA_API_TOKEN="your-api-token"
# optional, defaults to https://lgcy.atlassian.net
export JIRA_URL="https://lgcy.atlassian.net"
```

Generate API token: https://id.atlassian.com/manage-profile/security/api-tokens

Check credentials: `scripts/jira-tool.sh status`

## Commands

### Create

```bash
scripts/jira-tool.sh create \
  -p PROJECT \
  -s "Fix authentication bug" \
  -t Bug \
  -d "Detailed description" \
  -l "urgent,backend" \
  --priority High \
  -a accountId
```

Required: `-p` (project), `-s` (summary)
Optional: `-t` (type, default: Task), `-d` (description), `-l` (labels), `--priority`, `-a` (assignee accountId), `--parent` (for Sub-task), `--epic` (link to epic)

#### Sub-tasks

```bash
scripts/jira-tool.sh create -p PROJ -s "Subtask summary" -t Sub-task --parent PROJ-123
```

#### Link to Epic

```bash
scripts/jira-tool.sh create -p PROJ -s "Task summary" --epic PROJ-100
```

NOTE: Assignee uses Jira Cloud accountId (not email). Find via `search` on existing tickets.

### Get

```bash
scripts/jira-tool.sh get PROJ-123
```

Returns JSON with summary, status, assignee, priority, description, labels.

### Update

```bash
scripts/jira-tool.sh update PROJ-123 -s "New summary" -d "New desc" -a accountId
```

### Comment

```bash
scripts/jira-tool.sh comment PROJ-123 "Fixed in commit abc123"
```

### Transition

List available transitions:
```bash
scripts/jira-tool.sh transition PROJ-123
```

Transition to status:
```bash
scripts/jira-tool.sh transition PROJ-123 "In Progress"
```

### Close

Tries common close statuses (Done, Closed, Resolved, Complete):
```bash
scripts/jira-tool.sh close PROJ-123
```

### Assign

```bash
scripts/jira-tool.sh assign PROJ-123 accountId   # assign
scripts/jira-tool.sh assign PROJ-123 -1           # unassign
```

### Search

```bash
scripts/jira-tool.sh search "project = PROJ AND status = Open" 50
```

### Delete

```bash
scripts/jira-tool.sh delete PROJ-123
```

## Common JQL Queries

| Query | JQL |
|-------|-----|
| My open tickets | `assignee = currentUser() AND status != Done` |
| Project backlog | `project = PROJ AND status = "To Do"` |
| Recently updated | `project = PROJ AND updated >= -7d` |
| High priority | `priority in (High, Highest) AND status != Done` |

## Output

Most commands return JSON. Parse with `jq`:

```bash
scripts/jira-tool.sh get PROJ-123 | jq '.fields.status.name'
scripts/jira-tool.sh search "assignee = currentUser()" | jq '.issues[].key'
```

## Typical Workflow

```bash
# Create and start work
scripts/jira-tool.sh create -p PROJ -s "Implement feature X" -t Task
scripts/jira-tool.sh transition PROJ-456 "In Progress"

# Update progress
scripts/jira-tool.sh comment PROJ-456 "Initial implementation done"

# Complete
scripts/jira-tool.sh close PROJ-456
```
