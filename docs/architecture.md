# Architecture

## Overview

GNU Stow-based dotfiles system. `dot` bootstraps everything. `home/` mirrors `$HOME` exactly -- Stow symlinks it in place. No templating, no custom scripts.

## Components

```
.dotfiles/
├── dot                          # bootstrap entry point
├── packages/
│   └── Brewfile                 # all Homebrew deps
├── home/                        # stow target -> $HOME
│   ├── .zshenv                  # sets ZDOTDIR, XDG dirs
│   ├── .config/
│   │   ├── zsh/                 # shell config (ZDOTDIR)
│   │   ├── git/                 # git config + aliases
│   │   ├── ghostty/             # terminal emulator
│   │   ├── tmux/                # tmux + TPM (submodule)
│   │   ├── ripgrep/             # rg defaults
│   │   └── opencode/            # opencode.ai config
│   └── .claude/                 # Claude Code framework
└── docs/                        # documentation
```

## Bootstrap: `dot`

Single script, runs top-to-bottom:

1. Install Homebrew (Apple Silicon / Intel / Linux)
2. `brew bundle --file=packages/Brewfile`
3. `npm install -g @dmmulroy/overseer`
4. `stow --dir=. --target=$HOME home`
5. Set zsh as default shell

## Stow Strategy

`home/` is the sole stow package. Its directory tree maps 1:1 to `$HOME`:

| Source | Target |
|--------|--------|
| `home/.zshenv` | `~/.zshenv` |
| `home/.config/zsh/` | `~/.config/zsh/` |
| `home/.config/git/` | `~/.config/git/` |
| `home/.claude/` | `~/.claude/` |

No per-tool stow packages. One `stow home` command handles everything.

## Shell Config

XDG-compliant. `.zshenv` sets `ZDOTDIR=$HOME/.config/zsh`, redirecting all zsh config there.

| File | Scope | Purpose |
|------|-------|---------|
| `.zshenv` | All shells | `LANG`, `EDITOR`, XDG dirs, `ZDOTDIR` |
| `.zprofile` | Login | Homebrew init, base PATH |
| `.zshrc` | Interactive | Completions, PATH extensions, aliases, plugins, starship prompt |

Key tools loaded: nvm, bun, starship, eza, zsh-autosuggestions, zsh-syntax-highlighting.

## Packages

`packages/Brewfile` -- the single source of truth for system deps:

| Category | Packages |
|----------|----------|
| Core | git, stow, zsh |
| Shell | starship, zsh-autosuggestions, zsh-syntax-highlighting |
| Terminal | tmux |
| CLI | ast-grep, eza, gh, jq, ripgrep |

Everything else (nvm, bun, cargo) managed outside Homebrew.

## Claude Code Framework

`home/.claude/` is the largest component -- a full AI workflow layer.

### Structure

```
.claude/
├── CLAUDE.md                    # global instructions
├── settings.json                # model, permissions, hooks
├── mcp.json.example             # MCP server template
├── hooks/                       # TypeScript quality gates
├── commands/                    # slash command workflows
├── skills/                      # skill definitions (SKILL.md)
├── agents/                      # subagent definitions
└── rules/                       # always-loaded rules
```

### Hooks (TypeScript, esbuild -> ESM)

| Hook | Trigger | Purpose |
|------|---------|---------|
| `skill-activation-prompt` | UserPromptSubmit | Match keywords -> inject skill suggestions |
| `typescript-preflight` | PostToolUse (Edit/Write) | Non-blocking TS type-check warning |
| `typescript-stop-gate` | Stop | Blocking -- fails if TS errors remain |
| `statusline` | statusLine | Render branch, tokens, project in status bar |

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

## OpenCode (Parallel AI Config)

`home/.config/opencode/` mirrors the Claude framework: agents, commands, skills, custom tools. Configured with Dracula theme and Overseer MCP integration.

## Key Patterns

1. **Stow over everything** -- no custom symlink logic, no templating
2. **XDG-compliant** -- all config under `~/.config/`, `.zshenv` bootstraps `ZDOTDIR`
3. **Single bootstrap** -- `dot` handles Homebrew, packages, stow, shell in one pass
4. **Hook-enforced quality** -- TS errors block agent completion; skill activation routes on every prompt
5. **Dual AI support** -- Claude Code + OpenCode with mirrored skill/agent structures
6. **Secrets excluded** -- `mcp.json`, `.env*`, `.continuity/` all gitignored
