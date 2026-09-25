# Develop, Validate, Evaluate, Debug

## Load Locally

| Mode | How | Identity | Notes |
| --- | --- | --- | --- |
| Session flag | `claude --plugin-dir ./my-plugin` | `<name>@inline` | Repeat the flag for several plugins. Also accepts a `.zip`, or a folder of plugins (v2.1.265+; only subfolders with a manifest load). Overrides a same-named installed plugin unless managed settings force it. |
| Hosted zip | `claude --plugin-url <url>` | session only | Fetch failures show in the `/plugin` Errors tab |
| Skills-dir plugin | `claude plugin init <name>` scaffolds `~/.claude/skills/<name>/` | `<name>@skills-dir` | Auto-loads every session with no install. Remove with `claude plugin disable <name>@skills-dir` or by deleting the folder. |
| Local marketplace | `/plugin marketplace add ./marketplace`, then `/plugin install x@m` | `x@m` | Relative-path plugins load in place, so edits apply without a version bump |

A project-scope skills-dir plugin (`<cwd>/.claude/skills/<name>/`) has extra restrictions:

- It loads only after the user accepts workspace trust, and only from the primary working directory. It doesn't walk up to the repo root.
- Its MCP servers need per-server approval.
- Its monitors don't load.

## Reload Rules

- `/reload-plugins` picks up skills, agents, hooks, plugin MCP servers, and LSP servers. When a reload would invalidate the prompt cache, it asks you to rerun with `--force`.
- In a skills-dir plugin, `SKILL.md` edits apply live.
- **Monitors need a session restart.**

## Exercise Every Component

- Skills: run `/<plugin>:<skill>`, and also try a natural prompt that should trigger it.
- Agents: `@`-mention `<plugin>:<agent>`, or check `/context` under Custom Agents.
- Hooks: trigger the matched event, then read the hook debug log for matches, exit codes, and output.
- MCP: confirm the `mcp__plugin_<plugin>_<server>__*` tools are listed.
- `claude plugin details <name>` shows the component inventory, the **always-on** token cost (descriptions in every session), and per-component on-invoke cost. Keep always-on cost small.

## Validate

```bash
claude plugin validate ./my-plugin --strict   # CI: add --json
```

- Exit codes: `0` pass, `1` fail, `2` the run itself failed (e.g. an unreadable path).
- A plugin run checks `plugin.json`, `hooks/hooks.json`, and the frontmatter in the default `skills/`, `agents/`, and `commands/` dirs.
- It does **not** check a root `SKILL.md` or follow symlinks.
- Validating a dir without a manifest works on v2.1.233+. For example, `validate ./my-plugin/agents` checks agent frontmatter.
- A marketplace run (`validate .` at the marketplace root) checks the schema, duplicate names, path traversal, `renames` chains, and entry vs `plugin.json` version mismatches.
- Unknown fields produce warnings with a suggested fix. `--strict` makes them fail.
- Inside a session, `/plugin validate <path>` runs the same checks.

## Evaluate (`claude plugin eval`, v2.1.269+)

Evals measure whether Claude reaches for the plugin and gets the right result. Each case runs N times (default 3) **with** and **without** the plugin, and the report shows WITH, W/OUT, and Δ.

```bash
claude plugin eval init              # interview: reads the plugin, proposes + pilots cases
claude plugin eval init --bare case1 # blank template instead
claude plugin eval .                 # run the suite from the plugin root
```

Layout, in `evals/` or the dir set by `experimental.evals` / `--eval-dir`:

```text
evals/<case>/prompt.md        # frontmatter: max_turns, allowed_tools, runs, model, tags, env (EVAL_*)
evals/<case>/case.yaml        # optional: context.scaffold_script / history_file / add_dirs
evals/<case>/graders/<n>.md   # one grader per file
evals/mocks/<server>/<tool>.md
evals/results/                # add to .gitignore
```

Grader types:

- Free: `regex` (with a `target`), `tool_used`, `tool_order`, `file_exists`.
- Judge-model: `llm` (the rubric is the file body), `baseline`.
- There are no custom-code graders.

To check that a skill fired:

```markdown
---
type: tool_used
tool: Skill
input_match: '"skill"\s*:\s*"(?:[\w-]+:)?my-skill"'
---
```

In a two-arm run, a `tool_used: Skill` grader is reported but not scored unless it has `arm: both`.

Isolation:

- Every run starts in an empty sandbox with no user config.
- Only read-only tools are available. `--allow-tools Bash Write "mcp__plugin_<p>_<s>__*"` grants more.
- MCP servers are mocked through `mocks/<server>/<tool>.md`. `--allow-real-servers` opts out.
- Write each prompt the way a user would type it, without naming the skill. Put everything the task needs in the prompt or a scaffold script.

CI command:

```bash
claude plugin eval . --trust-plugin --threshold 0.8 --json results.json \
  --model <model> --judge-model <model> --no-publish --max-cost-usd 5
```

Exit codes: `0` every case met the threshold, `1` a case failed or a load error, `2` partial run (e.g. the cost cap was hit), `130` interrupted, `143` terminated.

## Debug

- `claude --debug` shows plugin loading, manifest errors, component registration, and MCP/LSP init failures.
- The `/plugin` **Errors** tab shows load errors, a missing LSP binary, and changed `command` sources.

| Symptom | Likely cause |
| --- | --- |
| Plugin loads, components missing | Components are inside `.claude-plugin/`, or a replacing path field hides the default dir |
| `path escapes plugin directory` | `../`, an absolute path, a backslash, or a symlink out of the root |
| Hook doesn't fire | Script not executable or missing its shebang, wrong event case (`PostToolUse`), or a bare MCP name in the matcher |
| MCP server fails | Path without `${CLAUDE_PLUGIN_ROOT}`, or a binary missing on the user's machine |
| `conflicting manifests` | `strict: false` entry plus a `plugin.json` that declares components |
| Update never arrives | `version` pinned and not bumped |
| Relative source fails | Marketplace added by direct URL to `marketplace.json` |

## CLI Quick Reference

`claude plugin`:

- Scaffolding: `init|new`.
- Install lifecycle: `install [--scope user|project|local]`, `uninstall|remove|rm [--prune] [--keep-data]`, `prune|autoremove`, `enable`, `disable`, `update`.
- Inspection: `list`, `details <name>`.
- Checks and releases: `validate`, `eval`, `eval init`, `tag [--push] [--dry-run]`.
- Marketplaces: `marketplace add|list|remove|update`.

Machine-readable and non-interactive flags:

- `install`, `uninstall`, `update`, `enable`, and `disable` take `--json` (v2.1.268+).
- `install` and `update` take `--yes` or `--accept-command <sha256>` to accept a `command` source or `headersHelper` non-interactively.
