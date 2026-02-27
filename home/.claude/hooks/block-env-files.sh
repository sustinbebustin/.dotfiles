#!/bin/bash
# Block Claude Code from reading .env files (but allow .env.example, .env.sample, .env.template)

# Read the hook input from stdin
input=$(cat)

# Extract tool name and relevant path/command
tool_name=$(echo "$input" | jq -r '.tool_name')
file_path=$(echo "$input" | jq -r '.tool_input.file_path // .tool_input.path // ""')
command=$(echo "$input" | jq -r '.tool_input.command // ""')
pattern=$(echo "$input" | jq -r '.tool_input.pattern // ""')

# Function to check if a path contains a blocked .env file
is_blocked_env() {
    local path="$1"
    # Match .env, .env.local, .env.production, .env.development, .dev.env, etc.
    # But NOT .env.example, .env.sample, or .env.template
    if echo "$path" | grep -qE '(^|/)\.env($|\.[^/]*$)|(^|/)[^/]*\.env($|[^/]*$)'; then
        if echo "$path" | grep -qE '\.(example|sample|template)$'; then
            return 1  # allowed
        fi
        return 0  # blocked
    fi
    return 1  # not an env file, allowed
}

# Check based on tool type
case "$tool_name" in
    "Read")
        if is_blocked_env "$file_path"; then
            echo '{"decision": "block", "reason": "🔒 Access to .env files is blocked for security. Use .env.example as a reference."}'
            exit 0
        fi
        ;;
    "Edit")
        if is_blocked_env "$file_path"; then
            echo '{"decision": "block", "reason": "🔒 Editing .env files is blocked for security."}'
            exit 0
        fi
        ;;
    "Write")
        if is_blocked_env "$file_path"; then
            echo '{"decision": "block", "reason": "🔒 Writing to .env files is blocked for security."}'
            exit 0
        fi
        ;;
    "Grep")
        if is_blocked_env "$file_path"; then
            echo '{"decision": "block", "reason": "🔒 Searching .env files is blocked for security."}'
            exit 0
        fi
        ;;
    "Bash")
        # Check if command might read/access env files
        # Readers
        if echo "$command" | grep -qE '(cat|less|more|head|tail|bat|nano|vim|vi|code|subl|open)\s+[^\|;&]*\.env'; then
            if ! echo "$command" | grep -qE '\.(example|sample|template)'; then
                echo '{"decision": "block", "reason": "🔒 Reading .env files via shell is blocked for security."}'
                exit 0
            fi
        fi
        # Source/export
        if echo "$command" | grep -qE '(source|\.|export)[^\|;&]*\.env'; then
            if ! echo "$command" | grep -qE '\.(example|sample|template)'; then
                echo '{"decision": "block", "reason": "🔒 Sourcing .env files via shell is blocked for security."}'
                exit 0
            fi
        fi
        # Search tools that might expose contents
        if echo "$command" | grep -qE '(grep|awk|sed|xargs|find)[^\|;&]*\.env'; then
            if ! echo "$command" | grep -qE '\.(example|sample|template)'; then
                echo '{"decision": "block", "reason": "🔒 Searching .env files via shell is blocked for security."}'
                exit 0
            fi
        fi
        ;;
esac

# Allow everything else
echo '{"decision": "approve"}'