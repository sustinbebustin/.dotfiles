# Architecture

## Overview

GNU Stow-based dotfiles system. `dot` bootstraps everything. A single stow package, `home/`, mirrors `$HOME`. No templating, no custom scripts.

## Components

```
.dotfiles/
├── dot                          # bootstrap entry point
├── packages/
│   └── Brewfile                 # Homebrew deps
├── home/                        # stow package -> $HOME
│   ├── .zshenv                  # sets ZDOTDIR, XDG dirs
│   ├── .agents/skills/          # shared agent skills (published by dot, not stow)
│   ├── .claude/                 # Claude Code framework
│   ├── .codex/                  # Codex config
│   └── .config/
│       ├── zsh/                 # shell config (ZDOTDIR)
│       ├── git/                 # git config + aliases
│       ├── ghostty/             # terminal emulator
│       ├── tmux/                # tmux + TPM
│       ├── ripgrep/             # rg defaults
│       └── karabiner/           # key remapping (macOS)
└── docs/                        # documentation
```

## Bootstrap: `dot`

Single script: Homebrew -> brew bundle -> Node toolchain -> CLI tools -> stow `home` -> set zsh default.

The Node toolchain and CLI tool steps are non-fatal: a network failure there warns and continues, so `dot init` always reaches the stow step.

## Stow Strategy

The package tree maps 1:1 to `$HOME`:

| Source | Target |
|--------|--------|
| `home/.zshenv` | `~/.zshenv` |
| `home/.config/zsh/` | `~/.config/zsh/` |
| `home/.config/karabiner/` | `~/.config/karabiner/` |

Stow is run with `--no-folding` to create per-file symlinks rather than directory symlinks, so a directory holding both stowed and unstowed files stays intact.

Agent skills are the exception. Claude Code reads `~/.claude/skills` and Codex reads `~/.agents/skills`, and neither reads the other's. The shared tree lives at Codex's root (`home/.agents/skills/`) but is stow-ignored, since stow can express none of what publishing it requires: the one-to-many fan-out, the category flattening, or the folder-level symlink Codex needs to follow a skill. `dot stow` publishes it to both roots instead. An optional one-level category dir is flattened away when linking, since both CLIs only read `<root>/<name>/SKILL.md`.

## Shell Config

XDG-compliant. `.zshenv` sets `ZDOTDIR=$HOME/.config/zsh`, redirecting all zsh config there.

| File | Scope | Purpose |
|------|-------|---------|
| `.zshenv` | All shells | `LANG`, `EDITOR`, XDG dirs, `ZDOTDIR`, Homebrew, Node/pnpm/bun env |
| `.zprofile` | Login | libpq |
| `.zshrc` | Interactive | Completions, PATH extensions, aliases, plugins, starship prompt |

Anything that decides which binary a command resolves to belongs in `.zshenv`, not `.zshrc`. `.zshrc` is skipped by non-interactive shells, so config placed there makes scripts, git hooks, ssh commands and agent tooling resolve a different node -- or no node -- than the terminal does.

Key tools loaded: pnpm, bun, starship, eza, zsh-autosuggestions, zsh-syntax-highlighting.

## Node Toolchain

pnpm is the single source of truth, installed by `dot init` from the standalone `get.pnpm.io` installer -- deliberately not via corepack, which pnpm upstream advises against. `pnpm env use --global` then provides node, pinned by `NODE_VERSION` in `dot`.

Global packages are declared in `PNPM_GLOBALS` in `dot` and installed with `pnpm add -g`. `dot doctor` warns if node resolves outside `$PNPM_HOME`, which means a second manager (nvm, brew, corepack) is competing for PATH.

Registry config is split: `~/.npmrc` is tracked and holds settings, while auth tokens go to the untracked `~/.npmrc.local` via `NPM_CONFIG_USERCONFIG`. Non-auth pnpm settings live in `~/.config/pnpm/config.yaml`, since pnpm v11 reads only auth and registry config from `.npmrc`.

## Packages

`packages/Brewfile` is the source of truth for system deps:

| Category | Packages |
|----------|----------|
| Core | git, stow, zsh |
| Shell | starship, zsh-autosuggestions, zsh-syntax-highlighting |
| Terminal | tmux |
| CLI | ast-grep, eza, gh, jq, ripgrep |

Everything else (pnpm, node, bun, cargo) managed outside Homebrew.

## Claude Code Framework

`home/.claude/` is the largest component -- a full AI workflow layer.

### Structure

```
.claude/
├── CLAUDE.md                    # global instructions
├── settings.json                # model, permissions, hooks
├── mcp.json.example             # MCP server template
├── hooks/                       # Go safety gates + statusline
├── commands/                    # slash command workflows
├── skills/                      # skill definitions (SKILL.md)
├── agents/                      # subagent definitions
└── rules/                       # always-loaded rules
```

### Hooks (Go, one module per hook)

Each hook is a standalone Go module built by `hooks/Makefile` into a `*-bin`
binary. The binaries are gitignored and built per-machine; only the built
binary is stowed into `~/.claude/hooks/`, never the source directory.

| Hook | Trigger | Purpose |
|------|---------|---------|
| `block-env-files` | PreToolUse (Read/Edit/Write/Bash/Grep) | Deny access to `.env` files |
| `block-aws-cli` | PreToolUse (Bash) | Ask before any `aws` CLI invocation |
| `block-dangerous-git` | PreToolUse (Bash) | Ask before push, merge, rebase, and other history- or worktree-destroying git commands |
| `block-dangerous-rm` | PreToolUse (Bash) | Ask before a recursive `rm` |
| `enforce-root` | PreToolUse (Bash) | Deny a top-level `cd`, which silently desyncs later commands |
| `statusline.sh` | statusLine | Render branch, tokens, project in status bar |

### Workflows (slash commands)

| Command | Purpose |
|---------|---------|
| `/lfg` | Full autonomous: plan -> deepen -> work -> review -> resolve -> test |
| `/slfg` | Simplified LFG |
| `/workflows:plan` | Feature planning with research + SpecFlow |
| `/workflows:work` | Implementation |
| `/workflows:review` | Multi-agent code review |
| `/workflows:brainstorm` | Exploratory brainstorming |

### Agents (organized by domain)

| Domain | Agents |
|--------|--------|
| `design/` | design-implementation-reviewer, design-iterator |
| `meta/` | agent-generator |
| `research/` | best-practices, framework-docs, git-history, learnings, repo-research |
| `review/` | architecture-strategist, code-simplicity, data-integrity, performance-oracle, security-sentinel, typescript-reviewer |
| `workflow/` | bug-reproduction-validator, pr-comment-resolver, spec-flow-analyzer |

## Key Patterns

1. **Stow over everything** -- no custom symlink logic, no templating
2. **XDG-compliant** -- all config under `~/.config/`, `.zshenv` bootstraps `ZDOTDIR`
3. **Single bootstrap** -- `dot` handles Homebrew, packages, stow, shell in one pass
4. **Hook-enforced quality** -- TS errors block agent completion; skill activation routes on every prompt
5. **Secrets excluded** -- `mcp.json`, `.env*`, `.continuity/` all gitignored
