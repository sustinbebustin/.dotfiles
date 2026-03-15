# Claude Code Hooks

Development home for Claude Code hooks. Hooks are developed here and synced to `~/.claude/hooks/` for use.

**Zero runtime dependencies** - hooks are pre-bundled, just sync and go.

---

## Available Hooks

| Hook | Event | Purpose |
|------|-------|---------|
| `session-start-ledger.sh` | SessionStart | Loads active ledger from database after `/clear` |
| `continuity-pre-compact.sh` | PreCompact | Writes marker file before context compaction |
| `continuity-post-compact.sh` | PostCompact | Runs after context compaction completes |
| `skill-activation-prompt.sh` | UserPromptSubmit | Suggests skills based on user prompt |
| `typescript-preflight.sh` | PreToolUse | TypeScript validation before file edits |
| `notification.sh` | various | System notification helper |
| `stop-tts.sh` | various | Stop text-to-speech playback |

---

## Marker File Flow

The session-start and pre-compact hooks work together using a marker file:

```
1. User runs /clear
   └── PreCompact event triggers continuity-pre-compact.sh
       └── Creates .continuity/.post_clear marker file

2. New session starts
   └── SessionStart event triggers session-start-ledger.sh
       ├── Checks for .post_clear marker
       ├── If marker exists:
       │   ├── Deletes marker file
       │   ├── Queries ledger from database via continuity ledger find
       │   └── Injects ledger context into session
       └── If no marker (fresh session):
           └── Skips ledger loading (no context injection)
```

This flow ensures:
- Fresh sessions start without legacy context
- Post-clear sessions resume with the active ledger
- No automatic handoff generation occurs

---

## Architecture

```
.claude/hooks/
├── *.sh                          # Shell scripts (entry points)
├── dist/
│   └── *.mjs                     # Pre-bundled JS (ready to run)
├── src/
│   └── *.ts                      # TypeScript source (for development)
└── package.json                  # Dev dependencies (esbuild only)
```

**Development:** Edit `src/*.ts`, run `npm run build` to rebuild.

**Deployment:** Sync to `~/.claude/hooks/` (handled by install script).

---

## Why Shell Wrapper + Bundled JS?

Claude Code hooks execute shell commands. We use two patterns:

**1. Pure Shell Scripts** (`.sh`) - For simple hooks
- Direct bash scripts for straightforward logic
- Example: `session-start-ledger.sh`, `continuity-pre-compact.sh`
- Uses `jq` for JSON parsing when needed

**2. Shell Wrapper + Bundled JS** - For complex logic
- Shell wrapper pipes stdin to bundled JS
- Example: `skill-activation-prompt.sh` calls `dist/skill-activation-prompt.mjs`

**Benefits of bundled JS:**
- **No npm install needed at runtime** - the `.mjs` is self-contained
- **JSON handling is native** - hooks receive JSON stdin, JS handles it naturally
- **Regex support** - full regex for pattern matching (bash regex is limited)
- **Maintainable** - TypeScript source with types, compiles to single file
- **Fast startup** - no module resolution, just `node file.mjs`

---

## How It Works

1. Claude calls the shell wrapper on `UserPromptSubmit`
2. Shell wrapper pipes stdin to the bundled JS
3. JS reads `skill-rules.json` from plugin and project directories
4. Matches user prompt against keyword/regex triggers
5. Outputs skill suggestions to stdout (injected into Claude's context)

---

## Installation

### 1. Run the install script

The install script syncs hooks to `~/.claude/hooks/`:

```bash
./install.sh
```

### 2. Register in settings.json

Add hooks to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "$HOME/.claude/hooks/skill-activation-prompt.sh"
          }
        ]
      }
    ]
  }
}
```

**Note:** Use `$HOME/.claude/hooks/*` paths in settings.json, not project paths.

---

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `HOME` | User home directory (hooks live at `$HOME/.claude/hooks/`) |
| `CLAUDE_PROJECT_DIR` | Current project directory (for project-level overrides) |

---

## skill-rules.json Format

```json
{
  "version": "1.0",
  "skills": {
    "skill-name": {
      "type": "domain",
      "priority": "high",
      "promptTriggers": {
        "keywords": ["keyword1", "keyword2"],
        "intentPatterns": ["regex.*pattern"]
      }
    }
  },
  "agents": {
    "agent-name": {
      "type": "domain",
      "priority": "medium",
      "promptTriggers": {
        "keywords": ["explore", "investigate"]
      }
    }
  }
}
```

**Priority levels:** `critical`, `high`, `medium`, `low`

**Merge behavior:** Project rules override plugin rules (same key = project wins).

---

## Development

### Modify a hook

```bash
# Edit TypeScript source
vim .claude/hooks/src/skill-activation-prompt.ts

# Install dev dependencies (first time only)
npm --prefix .claude/hooks install

# Rebuild bundled JS
npm --prefix .claude/hooks run build

# Test manually
echo '{"prompt": "help me debug this"}' | .claude/hooks/skill-activation-prompt.sh

# Sync to ~/.claude/hooks/
./install.sh
```

### Creating new hooks

Follow the same pattern:

**1. Create shell wrapper** (`my-hook.sh`):
```bash
#!/bin/bash
set -e
SCRIPT_DIR="$(dirname "$0")"
cat | node "$SCRIPT_DIR/dist/my-hook.mjs"
```

**2. Create TypeScript source** (`src/my-hook.ts`):
```typescript
import { readFileSync } from 'fs';

interface HookInput {
  prompt: string;
  // ... other fields from the hook event
}

async function main() {
  const input: HookInput = JSON.parse(readFileSync(0, 'utf-8'));

  // Your logic here

  // Output goes to Claude's context
  console.log('Your message to Claude');

  process.exit(0);
}

main().catch(err => {
  console.error('Error:', err);
  process.exit(1);
});
```

**3. Add to package.json build script** (or create separate):
```json
{
  "scripts": {
    "build": "esbuild src/my-hook.ts --bundle --platform=node --format=esm --outdir=dist --out-extension:.js=.mjs"
  }
}
```

**4. Build and test:**
```bash
npm run build
echo '{"prompt": "test"}' | ./my-hook.sh
```

---

## Hook Input Schema

The `UserPromptSubmit` event provides:

```typescript
interface HookInput {
  session_id: string;
  transcript_path: string;
  cwd: string;
  permission_mode: string;
  prompt: string;  // The user's message
}
```

Other hook events have different schemas. Check Claude Code documentation for specifics.
