# NUB - Rust Token Killer

**Usage**: Token-optimized CLI proxy (60-90% savings on dev operations)

## Meta Commands (always use nub directly)

```bash
nub gain              # Show token savings analytics
nub gain --history    # Show command usage history with savings
nub discover          # Analyze Claude Code history for missed opportunities
nub proxy <cmd>       # Execute raw command without filtering (for debugging)
```

## Installation Verification

```bash
nub --version         # Should show: nub X.Y.Z
nub gain              # Should work (not "command not found")
which nub             # Verify correct binary
```

## Hook-Based Usage

All other commands are automatically rewritten by the Claude Code hook.
Example: `git status` → `nub git status` (transparent, 0 tokens overhead)

Refer to CLAUDE.md for full command reference.
