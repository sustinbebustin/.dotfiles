# Architecture

## Overview

GNU Stow-based dotfiles system. `dot` bootstraps everything. A single stow package, `home/`, mirrors `$HOME`. No templating; `dot` adds custom linking only where stow cannot express it (skills, shared agent dirs, generated droids, extra Claude accounts).

## Components

```
.dotfiles/
├── dot                          # bootstrap entry point
├── packages/
│   └── Brewfile                 # Homebrew deps
├── home/                        # stow package -> $HOME
│   ├── .zshenv                  # sets ZDOTDIR, delegates to it
│   ├── .agents/                 # agent config shared by Claude Code and Droid
│   ├── .claude/                 # Claude Code only
│   ├── .factory/                # Factory Droid only: settings (BYOK models, hooks)
│   └── .config/
│       ├── zsh/                 # shell config (ZDOTDIR)
│       ├── starship.toml        # prompt
│       ├── git/                 # git config + aliases
│       ├── tmux/                # tmux + TPM
│       ├── pnpm/                # pnpm settings
│       ├── ripgrep/             # rg defaults
│       └── karabiner/           # key remapping (macOS)
└── docs/                        # documentation
```

## Bootstrap: `dot`

Single script: Homebrew -> brew bundle -> Node toolchain -> CLI tools -> back up conflicting files -> stow `home` -> set zsh default.

The Node toolchain and CLI tool steps are non-fatal: a network failure there warns and continues, so `dot init` always reaches the stow step.

## Stow Strategy

The package tree maps 1:1 to `$HOME`:

| Source | Target |
|--------|--------|
| `home/.zshenv` | `~/.zshenv` |
| `home/.config/zsh/` | `~/.config/zsh/` |
| `home/.config/karabiner/` | `~/.config/karabiner/` |

Stow is run with `--no-folding` to create per-file symlinks rather than directory symlinks, so a directory holding both stowed and unstowed files stays intact.

Skills are the exception. `home/.agents/skills/` is stow-ignored, since stow can express neither the category flattening nor the folder-level symlink each skill is published as. `dot stow` publishes it to `~/.claude/skills` (and each account's `skills/`) and to `~/.agents/skills`, which Droid reads as personal skills. An optional one-level category dir is flattened away when linking, since Claude Code only reads `<skills dir>/<name>/SKILL.md`.

`home/.agents/commands/` and `home/.agents/agents/` are stow-ignored too and published whole by `link_shared_dirs`: one dir symlink each into `~/.claude` and every account dir, and `commands` alone into `~/.factory`. `link_tree` replaces whatever was at the target only when it is ours -- a symlink, or a dir holding nothing but links into the repo -- so a fresh machine's per-file stow leftovers are converted and hand-made content is refused.

### Extra Claude accounts

`~/.claude` is the default account and always linked. Extra accounts are opt-in per machine: `dot claude add <name>` records the name in `~/.config/dot/claude-accounts` (outside the repo) and links the Claude config into `~/.claude-<name>`; `dot stow` relinks every registered account, and `dot claude remove <name>` unlinks one while keeping its login. `.zshrc` reads the same file and defines a function `<name>` that runs the claude wrappers with `CLAUDE_CONFIG_DIR` set to that dir.

`home/.claude` is stowed into each account dir as a package in its own right, which is why it carries its own `.stow-local-ignore` -- keep it in step with the `.claude` entries of `home/.stow-local-ignore`. Session state (`projects/`, `plans/`, `file-history/`, `history.jsonl`, listed in `CLAUDE_SHARED_STATE` in `dot`) is shared by symlinking `~/.claude-<name>/<path>` to `~/.claude/<path>`; existing account data is merged into `~/.claude` first.

## Shell Config

XDG-compliant. `~/.zshenv` sets `ZDOTDIR=$HOME/.config/zsh` and sources `$ZDOTDIR/.zshenv`, redirecting all zsh config there.

| File (in `$ZDOTDIR`) | Scope | Purpose |
|------|-------|---------|
| `.zshenv` | All shells | `LANG`, `EDITOR`, XDG dirs, Homebrew, Node/pnpm/bun env |
| `.zprofile` | Login | libpq |
| `.zshrc` | Interactive | Completions, PATH extensions, aliases, plugins, starship prompt |

Anything that decides which binary a command resolves to belongs in `.zshenv`, not `.zshrc`. `.zshrc` is skipped by non-interactive shells, so config placed there makes scripts, git hooks, ssh commands and agent tooling resolve a different node -- or no node -- than the terminal does.

Key tools loaded: pnpm, bun, starship, eza, fzf, atuin, zoxide, direnv, zsh-autosuggestions, zsh-syntax-highlighting.

compinit and the two zsh plugins load from a one-shot `zle-line-init` handler, after the first prompt is drawn. Anything a plugin would normally do in its first `precmd` has to be run by hand there. Tool init scripts (starship, fzf, atuin, zoxide) are sourced from compiled copies in `~/.cache/zsh/init`, rebuilt when a tool's binary or arguments change; delete that directory after changing a tool's own config if its init output depends on it. The prompt hooks avoid forking in the common case: direnv runs only when an environment is loaded or an `.envrc` exists up the tree, and the ssh-agent check only when the socket link is dead. Autosuggestions come from atuin (current directory first), with zsh history as the fallback; that strategy list is set after `atuin init`, which otherwise replaces it.

## Node Toolchain

pnpm is the single source of truth, installed by `dot init` from the standalone `get.pnpm.io` installer -- deliberately not via corepack, which pnpm upstream advises against. `pnpm runtime set node <version> -g` then provides it, pinned by `NODE_VERSION` in `dot`. pnpm v12's `globalShims` makes that global `node` project-aware: inside a project that pins Node (`devEngines.runtime`, `.nvmrc`, `.node-version`) it runs the pinned version, elsewhere `NODE_VERSION`. `PNPM_SHIM_BYPASS=1` skips the dispatch for one command.

Global packages are declared in `PNPM_GLOBALS` in `dot` and installed with `pnpm add -g`. `dot doctor` warns if node resolves outside `$PNPM_HOME`, which means a second manager (nvm, brew, corepack) is competing for PATH.

pnpm settings live in the tracked `~/.config/pnpm/config.yaml`. Registry tokens live beside it in the untracked `~/.config/pnpm/auth.ini`, which `pnpm login` writes. There is no tracked `.npmrc`: pnpm reads only auth and certificate settings from INI files.

## Packages

`packages/Brewfile` is the source of truth for system deps, grouped by category; manage it with `dot package add|remove|list`.

Go is a build dependency, not a runtime one: `dot stow` compiles the hooks in `home/.agents/hooks` before linking them, and `build_hooks` warns and skips when go is absent -- so an undeclared go means a machine silently runs without the hook safety gates. `make` is not declared, since it ships with the Xcode command line tools that Homebrew already requires on macOS and with the base install on Linux.

Everything else (pnpm, node, bun, cargo) managed outside Homebrew.

## Agent Framework

The largest component -- a full AI workflow layer for Claude Code and Factory Droid. `home/.agents/` is the one source of truth for what both harnesses read; each harness dir keeps only what is specific to it.

### Structure

```
.agents/                         # shared
├── AGENTS.md                    # global instructions
├── skills/                      # skill definitions (SKILL.md)
├── commands/                    # slash commands
├── agents/                      # subagent definitions (Claude Code format)
└── hooks/                       # Go safety gates + droid generator
.claude/                         # Claude Code only
├── CLAUDE.md                    # symlink to ../.agents/AGENTS.md
├── settings.json                # gitignored; model, permissions, hooks, statusline
├── settings.base.json           # tracked part of settings.json, merged in by dot
├── keybindings.json             # keyboard shortcuts
├── statusline.sh                # statusLine command
├── scripts/                     # helper scripts
├── mods/                        # Claude Code mods
└── rules/                       # always-loaded rules
.factory/                        # Droid only
├── settings.json                # gitignored
└── settings.base.json           # BYOK models, hook registration
```

Droid reads `~/.agents/AGENTS.md` and `~/.agents/skills` natively. `.claude/CLAUDE.md` is a symlink to `AGENTS.md`, so both harnesses read the same text, Claude Code tool names included. It, and skill and agent bodies, still mention Claude Code tools; hide one from Droid with `disabledSkills` in its settings.

### Droids

Droid cannot read Claude Code agent files, so `cmd/agent-droids` in the hooks module translates each agent in `.agents/agents/` into `~/.factory/droids/<name>.md`; `dot stow` runs it right after building the hooks, and warns and skips when it is not built. Tools map to Droid's IDs (`Bash` -> `Execute`, `Write` -> `Create`, `WebFetch` -> `FetchUrl`; `Skill`, `NotebookEdit` and MCP tools dropped), `disallowedTools` with no allowlist becomes `tools: read-only` when it removes Edit and Write, models map to the BYOK `custom:claude-*` IDs, and `effort` becomes `reasoningEffort` clamped to `high`. `color`, `maxTurns` and `skills` are dropped. An agent that lists `Agent` is skipped, since droids cannot spawn subagents (today: scoutmaster). The frontmatter parser reads only flat `key: value` lines, and an unknown key, tool or model fails the whole run with the file named, so a new field gets a deliberate mapping. Every generated file carries a `# generated by dot from <path>` line, and the generator only overwrites or prunes files that do, so Droid's own droids are never touched.

### Settings

`home/.claude/settings.json` is gitignored but stowed like any other file, so every account links the same one. It stays out of git because Claude Code writes to it and much of it is personal (model, UI toggles, `autoMode` rules). The tracked `settings.base.json` holds what this setup depends on, and `dot stow` merges it into `settings.json`, creating the file on a fresh clone. The base wins on the keys it defines and never touches the rest; objects merge per key, arrays of plain values (permission rules) are unioned, and other arrays (hook entries) are replaced. A key deleted from the base stays in `settings.json`. A merge that changes the file backs up the previous copy under `backups/` and prints the diff; `dot doctor` flags a file that is behind the base.

`home/.factory/settings.json` (Factory Droid) works the same way, with its own `settings.base.json`. That base owns `customModels`, so a model added through Droid's UI is replaced on the next `dot stow`; add it to the base instead. Its `modelPolicy.allowedModelIds` hides every Factory-hosted model (custom models are not gated by it), and `sessionDefaultSettings.model` uses the `custom:<Display-Name>-<n>` form Droid rewrites that key to. The Anthropic models read their key with `apiKeyHelper` from the untracked `~/.factory/anthropic-api-key`, not from `ANTHROPIC_API_KEY`, which Claude Code would pick up in place of a subscription login.

### Hooks (Go)

One Go module (`agent-hooks`) builds one guard binary, `agent-guard-bin`,
registered as a single PreToolUse hook in both `.claude/settings.base.json` and
`.factory/settings.base.json`. Droid sends the same payload under its own tool
names, so `internal/hook` maps them back on decode (`Execute` -> `Bash`,
`Create` -> `Write`; Read, Edit and Grep match already) and the rules see one
vocabulary. Droid's `ApplyPatch` carries patch text with no `file_path`, so it is
left out of the matcher: edits made through it bypass the guard and the quality
hook. Each guard is a pure function in `internal/rules/<name>/`; the
registry in `internal/rules/rules.go` decides which tools each one inspects and
reduces their verdicts worst-wins (any deny blocks, else any ask prompts, else
allow). A rule that panics is contained and counted as a deny, so one broken
guard cannot silence the rest.

`internal/rules/testdata/decisions.json` records the decision the full rule set
reaches on a corpus of payloads -- the cross-rule behaviour unit tests cannot
see. Regenerate with `make golden` and review the diff: a change there is a
change to what the guards allow.

Machine-specific settings live in `~/.agents/hooks/config.json`, read by
`internal/config`. This repository is public, so that file is gitignored and
`config.json.example` records the shape; unlike the source tree it *is* stowed,
since the binary reads it from `~/.agents/hooks/`. Today it holds
`dangerousRm.allowedRoots`, absolute (or `~/`-prefixed) directories under which
a recursive `rm` runs without prompting -- strictly under, so the root itself
still asks. Roots must be at least two directories deep, and one bad entry
rejects the whole file: a half-applied allowlist would be worse than none.
`awsCli.profiles` maps an AWS CLI profile to `allow`, `readOnly`, `ask` or
`deny`; a profile it does not name, or one `block-aws-cli` cannot determine,
asks. A broken file is reported on stderr and the guards run with no exemptions
rather than blocking the session -- which also drops any `deny`, so those calls
fall back to asking.
`$AGENT_HOOKS_CONFIG` overrides the location.

A session grant (`internal/grant`) waives some of `block-dangerous-git`'s
prompts for one session, because the user approved a skill up front. Claude
Code registers `agent-guard-bin grant` under UserPromptExpansion and under
PostToolUse for `AskUserQuestion`, and there are two ways to get a grant:

- Typing `/commit-push-pr` with `--merge`, `--bypass` or `--merge-bypass`
  waives `gh pr merge` at once; typing it without one revokes the grant.
- A skill listed under `grants.skills` in `config.json` waives the operations
  its entry allows (`prMerge`, `workflowRun`, `releaseCreate`, `pushMain`) in
  two steps. Typing it in a session at or under one of its `roots` leaves a
  pending grant that waives nothing. It goes live when the user answers a
  question dialog with an option beginning with the entry's `confirm` text --
  for a deploy skill, the "Proceed" on its plan.

Both signals are the user's own: the expansion event fires only for a typed
slash command, so the model invoking the skill, or a prompt that merely quotes
the command, mints nothing; and the dialog's payload carries the option the
user picked, whatever answer the model's call supplied. A grant is a file named
after the session in `~/.local/state/agent-guard-grants/`, good for an hour. It
is not single-use, since one invocation can merge several targets or dispatch
several workflows, and it names operations, not targets: within the hour the
session can run them against any repo. Only the plain form is waived -- a
forced push, a deleting refspec, or a command in a nested script that lost its
expansions still asks. The agent runs as the user and could write that file
itself, so `protect-grants` asks before any tool call that names the directory
or runs the grant mode. Droid has no registration and always asks.

The same module builds a second binary, `agent-quality-bin`, registered as a
PostToolUse hook on `Write|Edit` (Droid: `Create|Edit`). It formats and lints the edited file with the
tools its project uses, detected rather than configured
(`internal/quality`): the extension picks an ecosystem (node, go), and each
tool runs only when its config file (`oxlint.config.ts`, `.oxfmtrc.json`,
`.golangci.yml`, ...) sits above the file within the file's git repo, from the
directory holding that config. What the tools cannot fix is reported to Claude
as additional context; it never blocks. A project overrides detection in
`.claude/hooks.json` -- the nearest one above the file, looked for up to the
session's project dir -- as `{"quality": {"<ecosystem>": {"<lint|format>":
false | [argv...]}}}`, with `{file}` and `{pkg}` filled in. An invalid file is
reported and ignored whole. Adding a language is adding an ecosystem to
`internal/quality/catalog.go`.

`hooks/Makefile` builds all three binaries. They are gitignored and built
per-machine; only the two hook binaries are stowed into `~/.agents/hooks/`,
never the source tree or the droid generator. `make list` prints the registered
rules and the matcher each settings.base.json needs; a test reads both files and
fails when either matcher drifts from the registry.
`make check` is the gate before committing -- formatting, linters, tests -- and
needs `golangci-lint` (in the Brewfile); `.golangci.yml` configures both it and
the `gofumpt`/`gci` formatters `make fmt` applies.

| Rule | Applies to | Purpose |
|------|-----------|---------|
| `block-credential-files` | Read, Edit, Write, Bash, Grep | Deny access to credential files |
| `block-aws-cli` | Bash | Ask before an `aws` call that could reach real AWS, unless the config's per-profile policy allows it (all calls, or reads only) or denies it |
| `block-supabase-remote` | Bash | Deny Supabase CLI commands that wipe a remote database or delete a project; ask before other remote writes |
| `block-dangerous-git` | Bash | Ask before history- or worktree-destroying git commands and outward-facing gh operations (`pr merge`, `pr close`, writing `gh api`); deny `gh repo delete` and `gh api` DELETE. A session grant waives `pr merge`, `workflow run`, `release create` or a push naming main |
| `protect-grants` | Edit, Write, Bash | Ask before a call that names the session grant store or runs `agent-guard-bin grant` |
| `block-dangerous-rm` | Bash | Ask before a recursive `rm` |
| `enforce-root` | Bash | Deny a top-level `cd`, which silently desyncs later commands |

`statusline.sh` is separate, registered as the `statusLine` command.

### Skills, agents, commands

Workflows live in skills, not slash commands. Skills are grouped by category under `.agents/skills/` (`engineering`, `git`, `meta`, `productivity`, `review`, `tools`, `workflows`); the category layer is flattened on publish (see Stow Strategy), so names are unique across categories.

Subagents are grouped by domain under `.agents/agents/` (Claude Code reads the subdirs through the dir link): `design/` (the Impeccable build pipeline), `explore/` (scoutmaster and the scouts it dispatches), and `review/` (the code-review skill's spec, standards, and doc-comment axes; `/vet`'s `vet-reviewer` and `vet-verifier`; and `design-consultant`).

`/vet` (`skills/review/vet/`) is the one review pipeline: `scope.sh` resolves the scope, `vet.workflow.js` fans out reviewers over lens files and verifies their findings, and a repo teaches it project facts through an overlay at `.claude/vet.md` (this repo's sits at the root, outside `home/`).

`commands/` holds the few remaining standalone slash commands.

## Key Patterns

1. **Stow over everything** -- custom linking only where stow cannot express it, no templating
2. **XDG-compliant** -- all config under `~/.config/`, `.zshenv` bootstraps `ZDOTDIR`
3. **Single bootstrap** -- `dot` handles Homebrew, packages, stow, shell in one pass
4. **Hook-enforced safety** -- the Go guards deny or ask before destructive or outward-facing tool calls
5. **Secrets excluded** -- `mcp.json`, `settings.json`, `.env*`, hook `config.json`, `~/.config/pnpm/auth.ini`, `~/.factory/anthropic-api-key` all untracked
