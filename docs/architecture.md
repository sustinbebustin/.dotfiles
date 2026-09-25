# Architecture

## Overview

GNU Stow-based dotfiles system. `dot` bootstraps everything. A single stow package, `home/`, mirrors `$HOME`. No templating; `dot` adds custom linking only where stow cannot express it (skills, extra Claude accounts).

## Components

```
.dotfiles/
├── dot                          # bootstrap entry point
├── packages/
│   └── Brewfile                 # Homebrew deps
├── home/                        # stow package -> $HOME
│   ├── .zshenv                  # sets ZDOTDIR, delegates to it
│   ├── .npmrc                   # registry settings (auth in ~/.npmrc.local)
│   ├── .claude/                 # Claude Code framework
│   └── .config/
│       ├── zsh/                 # shell config (ZDOTDIR)
│       ├── starship.toml        # prompt
│       ├── git/                 # git config + aliases
│       ├── ghostty/             # terminal emulator
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

Skills are the exception. `home/.claude/skills/` is stow-ignored, since stow can express neither the category flattening nor the folder-level symlink each skill is published as. `dot stow` publishes it to `~/.claude/skills` instead. An optional one-level category dir is flattened away when linking, since Claude Code only reads `~/.claude/skills/<name>/SKILL.md`.

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

Key tools loaded: pnpm, bun, starship, eza, zsh-autosuggestions, zsh-syntax-highlighting.

## Node Toolchain

pnpm is the single source of truth, installed by `dot init` from the standalone `get.pnpm.io` installer -- deliberately not via corepack, which pnpm upstream advises against. `pnpm runtime set node <version> -g` then provides it, pinned by `NODE_VERSION` in `dot`.

Global packages are declared in `PNPM_GLOBALS` in `dot` and installed with `pnpm add -g`. `dot doctor` warns if node resolves outside `$PNPM_HOME`, which means a second manager (nvm, brew, corepack) is competing for PATH.

Registry config is split: `~/.npmrc` is tracked and holds settings, while auth tokens go to the untracked `~/.npmrc.local` via `NPM_CONFIG_USERCONFIG`. Non-auth pnpm settings live in `~/.config/pnpm/config.yaml`, since pnpm v11 reads only auth and registry config from `.npmrc`.

## Packages

`packages/Brewfile` is the source of truth for system deps, grouped by category; manage it with `dot package add|remove|list`.

Go is a build dependency, not a runtime one: `dot stow` compiles the hooks in `home/.claude/hooks` before linking them, and `build_hooks` warns and skips when go is absent -- so an undeclared go means a machine silently runs without the hook safety gates. `make` is not declared, since it ships with the Xcode command line tools that Homebrew already requires on macOS and with the base install on Linux.

Everything else (pnpm, node, bun, cargo) managed outside Homebrew.

## Claude Code Framework

`home/.claude/` is the largest component -- a full AI workflow layer.

### Structure

```
.claude/
├── CLAUDE.md                    # global instructions
├── settings.json                # model, permissions, hooks, statusline
├── keybindings.json             # keyboard shortcuts
├── statusline.sh                # statusLine command
├── hooks/                       # Go safety gates
├── scripts/                     # helper scripts
├── commands/                    # slash commands
├── skills/                      # skill definitions (SKILL.md)
├── agents/                      # subagent definitions
└── rules/                       # always-loaded rules
```

### Hooks (Go)

One Go module builds one binary, `claude-hooks-bin`, registered as a single
PreToolUse hook. Each guard is a pure function in `internal/rules/<name>/`; the
registry in `internal/rules/rules.go` decides which tools each one inspects and
reduces their verdicts worst-wins (any deny blocks, else any ask prompts, else
allow). A rule that panics is contained and counted as a deny, so one broken
guard cannot silence the rest.

`internal/rules/testdata/decisions.json` records the decision the full rule set
reaches on a corpus of payloads -- the cross-rule behaviour unit tests cannot
see. Regenerate with `make golden` and review the diff: a change there is a
change to what the guards allow.

Machine-specific settings live in `~/.claude/hooks/config.json`, read by
`internal/config`. This repository is public, so that file is gitignored and
`config.json.example` records the shape; unlike the source tree it *is* stowed,
since the binary reads it from `~/.claude/hooks/`. Today it holds
`dangerousRm.allowedRoots`, absolute (or `~/`-prefixed) directories under which
a recursive `rm` runs without prompting -- strictly under, so the root itself
still asks. Roots must be at least two directories deep, and one bad entry
rejects the whole file: the config only ever widens what runs unprompted, so a
half-applied allowlist would be worse than none. A broken file is reported on
stderr and the guards run with no exemptions rather than blocking the session.
`$CLAUDE_HOOKS_CONFIG` overrides the location.

The same module builds a second binary, `claude-quality-bin`, registered as a
PostToolUse hook on `Write|Edit`. It formats and lints the edited file with the
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

`hooks/Makefile` builds both. The binaries are gitignored and built per-machine;
only the built binaries are stowed into `~/.claude/hooks/`, never the source tree.
`make list` prints the registered rules and the `matcher` settings.json needs.
`make check` is the gate before committing -- formatting, linters, tests -- and
needs `golangci-lint` (in the Brewfile); `.golangci.yml` configures both it and
the `gofumpt`/`gci` formatters `make fmt` applies.

| Rule | Applies to | Purpose |
|------|-----------|---------|
| `block-credential-files` | Read, Edit, Write, Bash, Grep | Deny access to credential files |
| `block-aws-cli` | Bash | Ask before any `aws` CLI invocation |
| `block-supabase-remote` | Bash | Deny Supabase CLI commands that wipe a remote database or delete a project; ask before other remote writes |
| `block-dangerous-git` | Bash | Ask before history- or worktree-destroying git commands; deny outward-facing gh operations (`pr close`, `repo delete`, writing `gh api`) |
| `block-dangerous-rm` | Bash | Ask before a recursive `rm` |
| `enforce-root` | Bash | Deny a top-level `cd`, which silently desyncs later commands |

`statusline.sh` is separate, registered as the `statusLine` command.

### Skills, agents, commands

Workflows live in skills, not slash commands. Skills are grouped by category under `skills/` (`engineering`, `git`, `meta`, `productivity`, `tools`, `workflows`); the category layer is flattened on publish (see Stow Strategy), so names are unique across categories.

Subagents are grouped by domain under `agents/`: `design/` (the Impeccable build pipeline), `explore/` (scoutmaster and the scouts it dispatches), and `review/` (code-review axes and stack-specific reviewers).

`commands/` holds the few remaining standalone slash commands.

## Key Patterns

1. **Stow over everything** -- custom linking only where stow cannot express it, no templating
2. **XDG-compliant** -- all config under `~/.config/`, `.zshenv` bootstraps `ZDOTDIR`
3. **Single bootstrap** -- `dot` handles Homebrew, packages, stow, shell in one pass
4. **Hook-enforced safety** -- the Go guards deny or ask before destructive or outward-facing tool calls
5. **Secrets excluded** -- `mcp.json`, `.env*`, hook `config.json`, `~/.npmrc.local` all untracked
