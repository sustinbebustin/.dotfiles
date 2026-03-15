# Always Use Full Paths

Never use `cd` to change directories in Bash commands. Directory changes don't persist between commands.

## DO

```bash
# Use --prefix for npm/pnpm
npm --prefix frontend install
pnpm --prefix backend run build

# Use full paths for commands
pytest backend/tests/
ls /Users/austin/project/src/

# Use subshells for multi-command sequences
(cd frontend && npm install && npm run build)
```

## DON'T

```bash
# Don't use standalone cd
cd frontend && npm install    # Directory won't persist after this command
cd src                        # Pointless - next command starts fresh
```

## Why

Each Bash command runs in a fresh shell. Using `cd` wastes tokens and creates confusion about the current directory.
