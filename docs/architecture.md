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
│   ├── .claude/                 # Claude Code framework
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

Skills are the exception. `home/.claude/skills/` is stow-ignored, since stow can express neither the category flattening nor the folder-level symlink each skill is published as. `dot stow` publishes it to `~/.claude/skills` instead. An optional one-level category dir is flattened away when linking, since Claude Code only reads `~/.claude/skills/<name>/SKILL.md`.

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

pnpm is the single source of truth, installed by `dot init` from the standalone `get.pnpm.io` installer -- deliberately not via corepack, which pnpm upstream advises against. `pnpm runtime set node <version> -g` then provides it, pinned by `NODE_VERSION` in `dot`.

Global packages are declared in `PNPM_GLOBALS` in `dot` and installed with `pnpm add -g`. `dot doctor` warns if node resolves outside `$PNPM_HOME`, which means a second manager (nvm, brew, corepack) is competing for PATH.

Registry config is split: `~/.npmrc` is tracked and holds settings, while auth tokens go to the untracked `~/.npmrc.local` via `NPM_CONFIG_USERCONFIG`. Non-auth pnpm settings live in `~/.config/pnpm/config.yaml`, since pnpm v11 reads only auth and registry config from `.npmrc`.

## Packages

`packages/Brewfile` is the source of truth for system deps:

| Category | Packages |
|----------|----------|
| Core | gawk, git, go, just, shellcheck, stow, zsh |
| Shell | starship, zsh-autosuggestions, zsh-syntax-highlighting |
| Terminal | tmux |
| CLI | ast-grep, direnv, eza, gh, jq, ripgrep |
| Personal | sustinbebustin/tap/mws |

Go is a build dependency, not a runtime one: `dot stow` compiles the hooks in `home/.claude/hooks` before linking them, and `build_hooks` warns and skips when go is absent -- so an undeclared go means a machine silently runs without the hook safety gates. `make` is not declared, since it ships with the Xcode command line tools that Homebrew already requires on macOS and with the base install on Linux.

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

`hooks/Makefile` builds it. The binary is gitignored and built per-machine; only
the built binary is stowed into `~/.claude/hooks/`, never the source tree.
`make list` prints the registered rules and the `matcher` settings.json needs.
`make check` is the gate before committing -- formatting, linters, tests -- and
needs `golangci-lint` (in the Brewfile); `.golangci.yml` configures both it and
the `gofumpt`/`gci` formatters `make fmt` applies.

| Rule | Applies to | Purpose |
|------|-----------|---------|
| `block-credential-files` | Read, Edit, Write, Bash, Grep | Deny access to credential files |
| `block-aws-cli` | Bash | Ask before any `aws` CLI invocation |
| `block-dangerous-git` | Bash | Ask before history- or worktree-destroying git commands; deny outward-facing gh operations (`pr close`, `repo delete`, writing `gh api`) |
| `block-dangerous-rm` | Bash | Ask before a recursive `rm` |
| `enforce-root` | Bash | Deny a top-level `cd`, which silently desyncs later commands |

`statusline.sh` is separate, registered as the `statusLine` command.

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
