# Overseer MCP Templates

Drop these configs into each project so Overseer creates a per-project SQLite DB
(via `--cwd`) instead of using the default DB from the source install.

## Usage

Run from your project directory:

```bash
dot overseer mcp
```

This creates:
- `.mcp.json` -- Claude Code config with `--cwd` pointing to `$PWD`
- `.opencode/opencode.json` -- OpenCode config with `--cwd` pointing to `$PWD`

Existing files are skipped (not overwritten).

## Manual setup

1. **Claude Code** -- copy `mcp.json.example` to `<project>/.mcp.json`
   - Replace `{PROJECT_DIR}` with the absolute path to your project

2. **OpenCode** -- copy `opencode.json.example` to `<project>/.opencode/opencode.json`
   - Update the `--cwd` path to point to your project

Without `--cwd`, Overseer falls back to the DB in `~/dev/overseer/`, mixing
task data across projects.
