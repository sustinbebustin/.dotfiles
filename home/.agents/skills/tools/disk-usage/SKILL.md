---
name: disk-usage
description: Disk usage report -- filesystems, largest directories, caches, dependency dirs, Docker, and the cleanup commands that free the most space.
argument-hint: "[root]"
disable-model-invocation: true
allowed-tools: Bash(uv run ${CLAUDE_SKILL_DIR}/scripts/disk_usage.py), Bash(uv run ${CLAUDE_SKILL_DIR}/scripts/disk_usage.py *)
metadata:
  author: sustinbebustin
---

# Disk usage

Run the report with the Bash tool and `run_in_background: true`. A scan of `/` can outlast the foreground timeout:

```bash
uv run ${CLAUDE_SKILL_DIR}/scripts/disk_usage.py $ARGUMENTS
```

Wait for the completion notification, then read its output file. Reply with the markdown it printed, exactly as printed. Add no intro and no commentary. If it printed an error, relay the error and stop.

The report is read-only. Run a command from its Reclaimable table only when the user names that command.
