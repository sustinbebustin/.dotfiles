#!/bin/bash
# Block dangerous git/gh commands that modify remote state or destroy local work

input=$(cat)

tool_name=$(echo "$input" | jq -r '.tool_name')

# Only check Bash commands
if [ "$tool_name" != "Bash" ]; then
    echo '{"decision": "approve"}'
    exit 0
fi

command=$(echo "$input" | jq -r '.tool_input.command // ""')

# Normalize: collapse whitespace, strip leading whitespace
normalized=$(echo "$command" | tr '\n' ' ' | sed 's/  */ /g; s/^ //')

# --- git commands ---

# Push (any variant)
if echo "$normalized" | grep -qE '\bgit\s+push\b'; then
    echo '{"decision": "block", "reason": "[BLOCKED] git push - remote modifications not allowed"}'
    exit 0
fi

# Merge
if echo "$normalized" | grep -qE '\bgit\s+merge\b'; then
    echo '{"decision": "block", "reason": "[BLOCKED] git merge - branch merges not allowed"}'
    exit 0
fi

# Rebase
if echo "$normalized" | grep -qE '\bgit\s+rebase\b'; then
    echo '{"decision": "block", "reason": "[BLOCKED] git rebase - rebasing not allowed"}'
    exit 0
fi

# Reset --hard
if echo "$normalized" | grep -qE '\bgit\s+reset\s+--hard\b'; then
    echo '{"decision": "block", "reason": "[BLOCKED] git reset --hard - destructive reset not allowed"}'
    exit 0
fi

# Clean (-f, -fd, -fx, etc)
if echo "$normalized" | grep -qE '\bgit\s+clean\b'; then
    echo '{"decision": "block", "reason": "[BLOCKED] git clean - file deletion not allowed"}'
    exit 0
fi

# Force delete branch
if echo "$normalized" | grep -qE '\bgit\s+branch\s+-[dD]\b'; then
    echo '{"decision": "block", "reason": "[BLOCKED] git branch delete not allowed"}'
    exit 0
fi

# Checkout -- (discard changes)
if echo "$normalized" | grep -qE '\bgit\s+checkout\s+--\b'; then
    echo '{"decision": "block", "reason": "[BLOCKED] git checkout -- discards changes, not allowed"}'
    exit 0
fi

# Restore (discard changes)
if echo "$normalized" | grep -qE '\bgit\s+restore\b'; then
    echo '{"decision": "block", "reason": "[BLOCKED] git restore - discarding changes not allowed"}'
    exit 0
fi

# Stash drop/clear
if echo "$normalized" | grep -qE '\bgit\s+stash\s+(drop|clear)\b'; then
    echo '{"decision": "block", "reason": "[BLOCKED] git stash drop/clear - stash destruction not allowed"}'
    exit 0
fi

# Tag delete
if echo "$normalized" | grep -qE '\bgit\s+tag\s+-d\b'; then
    echo '{"decision": "block", "reason": "[BLOCKED] git tag delete not allowed"}'
    exit 0
fi

# --- gh CLI commands ---

# gh pr merge
if echo "$normalized" | grep -qE '\bgh\s+pr\s+merge\b'; then
    echo '{"decision": "block", "reason": "[BLOCKED] gh pr merge - PR merging not allowed"}'
    exit 0
fi

# gh pr close
if echo "$normalized" | grep -qE '\bgh\s+pr\s+close\b'; then
    echo '{"decision": "block", "reason": "[BLOCKED] gh pr close - PR closing not allowed"}'
    exit 0
fi

# gh issue close/delete
if echo "$normalized" | grep -qE '\bgh\s+issue\s+(close|delete)\b'; then
    echo '{"decision": "block", "reason": "[BLOCKED] gh issue close/delete not allowed"}'
    exit 0
fi

# gh release create/delete
if echo "$normalized" | grep -qE '\bgh\s+release\s+(create|delete)\b'; then
    echo '{"decision": "block", "reason": "[BLOCKED] gh release create/delete not allowed"}'
    exit 0
fi

# gh repo delete/rename
if echo "$normalized" | grep -qE '\bgh\s+repo\s+(delete|rename)\b'; then
    echo '{"decision": "block", "reason": "[BLOCKED] gh repo delete/rename not allowed"}'
    exit 0
fi

# gh api with destructive methods
if echo "$normalized" | grep -qE '\bgh\s+api\b.*(-X\s*(PUT|POST|PATCH|DELETE)|--method\s*(PUT|POST|PATCH|DELETE))'; then
    echo '{"decision": "block", "reason": "[BLOCKED] gh api with destructive HTTP method not allowed"}'
    exit 0
fi

echo '{"decision": "approve"}'
