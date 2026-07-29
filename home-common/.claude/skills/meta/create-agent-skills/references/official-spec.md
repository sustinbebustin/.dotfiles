# Official Skill Specification (2026)

Source: [code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills)

Skills follow the [Agent Skills](https://agentskills.io) open standard; Claude Code extends it with invocation control, subagent execution, and dynamic context injection.

## Commands and Skills Are Merged

Custom slash commands have been merged into skills. A file at `.claude/commands/review.md` and a skill at `.claude/skills/review/SKILL.md` both create `/review` and work the same way. Existing `.claude/commands/` files keep working. Skills add optional features: a directory for supporting files, frontmatter to control invocation, and automatic context loading.

If a skill and a command share the same name, the skill takes precedence.

## SKILL.md File Structure

Every skill requires a `SKILL.md` file with YAML frontmatter followed by standard markdown instructions.

```markdown
---
name: your-skill-name
description: What it does and when to use it
---

# Your Skill Name

## Instructions
Clear, step-by-step guidance.

## Examples
Concrete examples of using this skill.
```

## Complete Frontmatter Reference

All fields are optional. Only `description` is recommended.

| Field | Required | Description |
|-------|----------|-------------|
| `name` | No | Display name. Lowercase letters, numbers, hyphens only (max 64 chars). Defaults to directory name if omitted. |
| `description` | Recommended | What it does and when to use it. Front-load the key use case: combined `description` + `when_to_use` is truncated at **1,536 characters** in the skill listing to reduce context usage. If omitted, uses the first paragraph of markdown content. |
| `when_to_use` | No | Additional context for when Claude should invoke the skill (trigger phrases, example requests). Appended to `description`; counts toward the 1,536-char cap. |
| `argument-hint` | No | Hint shown during autocomplete. Example: `[issue-number]` or `[filename] [format]` |
| `arguments` | No | Named positional arguments declared for `$name` substitution. Names map to argument positions in order — `arguments: [issue, branch]` makes `$issue` the first arg and `$branch` the second. Accepts a space-separated string or YAML list. |
| `disable-model-invocation` | No | Set `true` to prevent Claude from auto-loading. Use for manual workflows. Also blocks preloading into a subagent's `skills:` field, and (v2.1.196+) blocks a scheduled task firing with the skill as its prompt. Default: `false` |
| `user-invocable` | No | Set `false` to hide from `/` menu. Use for background knowledge. Default: `true` |
| `allowed-tools` | No | Tools Claude can use without per-use approval while the skill is active. Does *not* restrict tools — every tool remains callable and permission settings still govern unlisted tools. Accepts space-separated string or YAML list. |
| `disallowed-tools` | No | Tools removed from Claude's available pool while the skill is active. Use for autonomous skills that must never call a tool (e.g. `AskUserQuestion` in a background loop). Restriction clears on the next message. Accepts space/comma-separated string or YAML list. |
| `model` | No | Model to use. Accepts an alias (`haiku`, `sonnet`, `opus`), a full model ID (e.g. `claude-opus-4-7`, `claude-sonnet-4-6`), or `inherit`. Override applies for the rest of the current turn and is **not** saved to settings — the session model resumes on the next prompt. |
| `effort` | No | Effort level while skill is active. Options: `low`, `medium`, `high`, `xhigh`, `max`; available levels depend on the model. Overrides session effort. |
| `context` | No | Set `fork` to run in isolated subagent context |
| `agent` | No | Subagent type when `context: fork`. Options: `Explore`, `Plan`, `general-purpose`, or any custom subagent from `.claude/agents/`. Defaults to `general-purpose`. |
| `background` | No | Only applies with `context: fork`. Default `true` — the forked subagent runs in the background and returns its result later. `false` waits for it in the invoking turn and keeps the full tool set. Requires v2.1.218+. |
| `hooks` | No | Hooks scoped to this skill's lifecycle. See [Hooks in skills and agents](https://code.claude.com/docs/en/hooks#hooks-in-skills-and-agents). |
| `paths` | No | Glob patterns limiting auto-activation. When set, Claude loads the skill automatically only when working with matching files. Comma-separated string or YAML list. Same format as path-specific memory rules. |
| `shell` | No | Shell for inline and fenced shell-injection blocks. `bash` (default) or `powershell`. `powershell` requires the PowerShell tool: on by default on Windows without Git Bash, `CLAUDE_CODE_USE_POWERSHELL_TOOL=1` elsewhere. |

Boolean fields accept `yes`, `no`, `on`, `off`, `1`, and `0` in any case as well as `true`/`false` (v2.1.218+; earlier versions recognized only `true`/`false`).

## Invocation Control

| Frontmatter | User can invoke | Claude can invoke | When loaded into context |
|-------------|----------------|-------------------|--------------------------|
| (default) | Yes | Yes | Description always in context, full skill loads when invoked |
| `disable-model-invocation: true` | Yes | No | Description not in context, full skill loads when you invoke |
| `user-invocable: false` | No | Yes | Description always in context, full skill loads when invoked |

## Skill Locations & Priority

```
Enterprise (highest priority) → Personal → Project → Plugin (lowest priority)
```

| Type | Path | Applies to |
|------|------|-----------|
| Enterprise | See managed settings | All users in organization |
| Personal | `~/.claude/skills/<name>/SKILL.md` | You, across all projects |
| Project | `.claude/skills/<name>/SKILL.md` | Anyone working in repository |
| Plugin | `<plugin>/skills/<name>/SKILL.md` | Where plugin is enabled |

Plugin skills use a `plugin-name:skill-name` namespace, so they cannot conflict with other levels. If a skill and a command share the same name, the skill takes precedence.

### Live change detection

Claude Code watches skill directories for file changes. Adding, editing, or removing a skill under `~/.claude/skills/`, the project `.claude/skills/`, or a `.claude/skills/` inside an `--add-dir` directory takes effect within the current session without restart. Creating a top-level skills directory that did not exist when the session started requires restarting Claude Code so the new directory can be watched.

### Nested directory discovery

When working with files in subdirectories, Claude Code automatically discovers skills from nested `.claude/skills/` directories. If you're editing a file in `packages/frontend/`, Claude Code also looks for skills in `packages/frontend/.claude/skills/`. Supports monorepo setups where packages have their own skills.

### Skills from `--add-dir`

The `--add-dir` flag and `/add-dir` command grant file access, not configuration discovery — but skills are an exception: `.claude/skills/` within an added directory is loaded automatically (as is `.claude/agents/`). Commands and output styles are *not* loaded from additional directories. The exception covers only `--add-dir`/`/add-dir`; the `permissions.additionalDirectories` setting grants file access alone and loads no skills.

## Bundled Skills

Claude Code ships with prompt-based bundled skills available in every session: `/doctor`, `/code-review`, `/batch`, `/debug`, `/loop`, `/claude-api`, plus `/run`, `/verify`, and `/run-skill-generator` for launching and checking your app. Unlike built-in commands (fixed logic), bundled skills are detailed playbooks Claude orchestrates using its tools. Invoke them like any other skill.

Claude auto-invokes some of them, but since v2.1.215 `/verify` and `/code-review` run only when you invoke them, and neither can be preloaded into a subagent. `disableBundledSkills` turns off every bundled skill except `/doctor` (hide that with `DISABLE_DOCTOR_COMMAND` or a `skillOverrides` entry of `"doctor": "off"`).

`/code-review` (renamed from `/simplify` in v2.1.147) reports correctness bugs at a chosen effort level: e.g. `/code-review high`. Pass `--comment` to post findings as inline GitHub PR comments.

## How Skills Work

1. **Discovery**: Claude loads only name and description at startup (budget: 1% of the model's context window)
2. **Activation**: When your request matches a skill's description, Claude loads the full content
3. **Execution**: Claude follows the skill's instructions

## String Substitutions

| Variable | Description |
|----------|-------------|
| `$ARGUMENTS` | All arguments passed when invoking. If not present in content, arguments are appended as `ARGUMENTS: <value>`. |
| `$ARGUMENTS[N]` | Specific argument by 0-based index. Uses shell-style quoting — wrap multi-word values in quotes. |
| `$N` | Shorthand for `$ARGUMENTS[N]` (`$0`, `$1`, ...). |
| `$name` | Named argument declared in the `arguments` frontmatter list. With `arguments: [issue, branch]`, `$issue` expands to the first arg and `$branch` to the second. |
| `${CLAUDE_SESSION_ID}` | Current session ID. Useful for logging or session-specific files. |
| `${CLAUDE_EFFORT}` | Current effort level: `low`, `medium`, `high`, `xhigh`, or `max`. Use to adapt skill instructions to the active effort setting. |
| `${CLAUDE_SKILL_DIR}` | Directory containing the skill's `SKILL.md`. For plugin skills, the skill's subdirectory within the plugin, not the plugin root. Use inside shell-injection blocks to reference bundled scripts regardless of cwd. |
| `${CLAUDE_PROJECT_DIR}` | Project root — the same path hooks and MCP servers receive. Requires v2.1.196+. |

`${CLAUDE_SKILL_DIR}` and `${CLAUDE_PROJECT_DIR}` are substituted both in the markdown body and in `allowed-tools` Bash rules, so `allowed-tools: Bash(${CLAUDE_SKILL_DIR}/scripts/render.sh *)` matches the exact command the body tells Claude to run and the script executes without prompting. (`allowed-tools` substitution requires v2.1.129+.)

## Dynamic Context Injection

The `` !`command` `` syntax runs shell commands before content is sent to Claude:

```markdown
## Context
- Current branch: !`git branch --show-current`
- PR diff: !`gh pr diff`
```

Commands execute immediately and their output replaces the placeholder. Claude only sees the final result — this is preprocessing, not something Claude executes.

For multi-line commands, open a fenced markdown code block whose opening fence ends with an exclamation mark (no space between the triple-backtick and the `!`), list commands on subsequent lines, then close with a normal fence.

**Disabling:** Set `"disableSkillShellExecution": true` in settings to replace every command with `[shell command execution disabled by policy]`. Most useful in managed settings. Does not affect bundled/managed skills.

**Extended thinking:** Include the word `ultrathink` anywhere in skill content to enable thinking mode.

## Progressive Disclosure

```
my-skill/
├── SKILL.md           # Entry point (required)
├── reference.md       # Detailed docs (loaded when needed)
├── examples.md        # Usage examples (loaded when needed)
└── scripts/
    └── helper.py      # Utility script (executed, not loaded)
```

Keep SKILL.md under 500 lines. Link to supporting files:
```markdown
For API details, see [reference.md](reference.md).
```

## Running in a Subagent

Add `context: fork` to run in isolation:

```yaml
---
name: deep-research
description: Research a topic thoroughly
context: fork
agent: Explore
---

Research $ARGUMENTS thoroughly...
```

The skill content becomes the subagent's prompt. It won't have access to conversation history.

**Background by default (v2.1.218+).** The forked subagent runs in the background and its result arrives in the conversation when it finishes; `background: false` waits for it in the invoking turn. Claude Code waits regardless in `-p`/SDK mode, with `CLAUDE_CODE_DISABLE_BACKGROUND_TASKS=1`, when the same forked skill is already running, and when a scheduled task fires it. A backgrounded fork runs with the narrower background-subagent tool set (the fork-the-conversation exemption doesn't apply — it's a regular agent type), so set `background: false` if the skill needs a tool outside it. Its edits also fall outside `/rewind` checkpoints; revert with git.

`context: fork` skills also end slash-command stacking: `/other-skill /forked-skill args` stops expanding at the forked skill.

**Warning:** `context: fork` only makes sense for skills with explicit task instructions. If a skill contains only guidelines like "use these API conventions," the subagent receives the guidelines but no actionable prompt and returns without meaningful output.

**Avoid self-recursion.** The forked subagent inherits the same skill registry, so a body that restates the description's trigger phrases can re-match its own skill and re-invoke itself. The runtime now guards against tight loops (v2.1.145 fix), but write the body as a direct task ("Research $ARGUMENTS...") rather than "when the user asks for X, do Y".

## Skill Content Lifecycle

When invoked, rendered `SKILL.md` content enters the conversation as a single message and stays there for the rest of the session. Claude Code does *not* re-read the skill file on later turns — write guidance as standing instructions rather than one-time steps.

The persistence covers instructions, not permissions: an `allowed-tools` grant clears on your next message and re-applies each time you invoke the skill. Re-invoking with identical rendered content adds a short "already loaded" note instead of a second copy (v2.1.202+); content that differs, because arguments or injected command output changed, is appended in full.

**Auto-compaction:** invoked skills are carried forward within a token budget. When the conversation is summarized to free context, Claude Code re-attaches the most recent invocation of each skill after the summary, keeping the first **5,000 tokens** of each. Re-attached skills share a combined budget of **25,000 tokens**, filled starting from the most recently invoked. Older skills can be dropped entirely after compaction if many were invoked.

If a skill seems to stop influencing behavior, the content is usually still present and the model is choosing other tools. Strengthen `description` and instructions, or use hooks to enforce behavior deterministically. If the skill is large or you invoked several after it, re-invoke to restore the full content.

## Permissions & Access Control

Three ways to control which skills Claude can invoke:

**Disable all skills** — deny the `Skill` tool in `/permissions`.

**Allow or deny specific skills** via permission rules:

```
Skill(commit)          # exact match
Skill(review-pr *)     # prefix match with any arguments
Skill(deploy *)        # in deny rules, blocks the skill
```

**Hide individual skills** with `disable-model-invocation: true` — removes the skill from Claude's context entirely. Note: `user-invocable` only controls menu visibility, not Skill-tool access.

A few built-in commands *are* available through the Skill tool (e.g. `/init`, `/review`, `/security-review`). Others like `/compact` are not.

### `skillOverrides` setting

`skillOverrides` in [settings](https://code.claude.com/docs/en/settings) controls per-skill visibility *without* editing the skill's frontmatter — useful for shared-repo or MCP-provided skills you can't modify. The `/skills` menu writes overrides to `.claude/settings.local.json` (cycle with `Space`, save with `Enter`).

| Value | Listed to Claude | In `/` menu |
|-------|------------------|-------------|
| `"on"` | Name + description | Yes |
| `"name-only"` | Name only | Yes |
| `"user-invocable-only"` | Hidden | Yes |
| `"off"` | Hidden | Hidden |

```json
{
  "skillOverrides": {
    "legacy-context": "name-only",
    "deploy": "off"
  }
}
```

A skill absent from `skillOverrides` is treated as `"on"`. Plugin skills are not affected — manage those via `/plugin`.

## Configuration Knobs

| Setting / Env Var | Effect |
|-------------------|--------|
| `skillListingBudgetFraction` | Fraction of the context window the skill listing may use (default `0.01`). |
| `SLASH_COMMAND_TOOL_CHAR_BUDGET` | Set the skill-listing budget to a fixed character count instead. |
| `skillListingMaxDescChars` | Per-entry cap on `description` + `when_to_use` (default 1,536). |
| `disableBundledSkills: true` | Turn off every bundled skill except `/doctor`. |
| `disableSkillShellExecution: true` | Disable shell-injection preprocessing for user/project/plugin/add-dir skills. Bundled/managed skills unaffected. |
| `CLAUDE_CODE_USE_POWERSHELL_TOOL=1` | Required for skills with `shell: powershell`. |

## Troubleshooting

**Skill not triggering:**
1. Make sure `description` includes keywords users would naturally say
2. Verify it appears in `What skills are available?`
3. Rephrase the request to match the description more closely
4. Invoke directly with `/skill-name` if user-invocable

**Skill triggers too often:** Tighten the description, or set `disable-model-invocation: true`.

**Descriptions cut short:** All names are always included, but descriptions are shortened to fit the character budget when many skills are loaded, dropping the least-invoked skills' descriptions first. Run `/doctor` for an estimate of the listing's context cost and its biggest contributors. Front-load key use cases, trim `description`/`when_to_use`, set low-priority skills to `"name-only"` in `skillOverrides`, or raise `skillListingBudgetFraction` / `SLASH_COMMAND_TOOL_CHAR_BUDGET`. Each entry is capped at 1,536 chars regardless of budget.

## Distribution

- **Project skills**: Commit `.claude/skills/` to version control
- **Plugins**: Add `skills/` directory to plugin
- **Enterprise**: Deploy organization-wide through managed settings
