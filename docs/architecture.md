# Architecture

## Overview

GNU Stow-based dotfiles system. `dot` bootstraps everything. Configs are split into platform-specific stow packages (`home-common/`, `home-arch/`, `home-macos/`) that mirror `$HOME`. No templating, no custom scripts.

## Components

```
.dotfiles/
├── dot                          # bootstrap entry point
├── packages/
│   ├── Brewfile                 # Homebrew deps (macOS)
│   └── Pacfile                  # pacman + AUR deps (Arch)
├── home-common/                 # stow package -> $HOME (all platforms)
│   ├── .zshenv                  # sets ZDOTDIR, XDG dirs
│   ├── .claude/                 # Claude Code framework
│   └── .config/
│       ├── zsh/                 # shell config (ZDOTDIR)
│       ├── git/                 # git config + aliases
│       ├── ghostty/             # terminal emulator
│       ├── tmux/                # tmux + TPM
│       ├── ripgrep/             # rg defaults
│       └── opencode/            # opencode.ai config
├── home-arch/                   # stow package -> $HOME (Arch only)
│   └── .config/
│       ├── hypr/                # Hyprland compositor
│       ├── waybar/              # status bar
│       ├── swaync/              # notifications
│       ├── walker/              # app launcher
│       ├── matugen/             # Material You theming
│       └── ...                  # cava, fastfetch, kanata, xremap, systemd
├── home-macos/                  # stow package -> $HOME (macOS only)
│   └── .config/
│       └── karabiner/           # key remapping
└── docs/                        # documentation
```

## Bootstrap: `dot`

Single script, auto-detects platform:

**Arch:** pacman/AUR packages -> stow `home-common` + `home-arch` -> set zsh default
**macOS:** Homebrew -> brew bundle -> npm globals -> stow `home-common` + `home-macos` -> set zsh default

## Stow Strategy

Configs are organized into per-platform stow packages. `dot stow` selects the right packages based on detected platform:

| Platform | Packages stowed |
|----------|----------------|
| Arch | `home-common`, `home-arch` |
| macOS | `home-common`, `home-macos` |
| Other Linux | `home-common` |

Each package's directory tree maps 1:1 to `$HOME`:

| Source | Target |
|--------|--------|
| `home-common/.zshenv` | `~/.zshenv` |
| `home-common/.config/zsh/` | `~/.config/zsh/` |
| `home-arch/.config/hypr/` | `~/.config/hypr/` |
| `home-macos/.config/karabiner/` | `~/.config/karabiner/` |

Stow is run with `--no-folding` to create per-file symlinks rather than directory symlinks, preventing conflicts between packages.

## Shell Config

XDG-compliant. `.zshenv` sets `ZDOTDIR=$HOME/.config/zsh`, redirecting all zsh config there.

| File | Scope | Purpose |
|------|-------|---------|
| `.zshenv` | All shells | `LANG`, `EDITOR`, XDG dirs, `ZDOTDIR` |
| `.zprofile` | Login | Homebrew init, base PATH |
| `.zshrc` | Interactive | Completions, PATH extensions, aliases, plugins, starship prompt |

Key tools loaded: nvm, bun, starship, eza, zsh-autosuggestions, zsh-syntax-highlighting.

## Packages

`packages/Brewfile` (macOS) and `packages/Pacfile` (Arch) are the source of truth for system deps:

### macOS

| Category | Packages |
|----------|----------|
| Core | git, stow, zsh |
| Shell | starship, zsh-autosuggestions, zsh-syntax-highlighting |
| Terminal | tmux |
| CLI | ast-grep, eza, gh, jq, ripgrep |

Everything else (nvm, bun, cargo) managed outside Homebrew.

## Claude Code Framework

`home-common/.claude/` is the largest component -- a full AI workflow layer.

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

## OpenCode (Parallel AI Config)

`home-common/.config/opencode/` mirrors the Claude framework: agents, commands, skills, custom tools. Configured with Dracula theme.

## Key Patterns

1. **Stow over everything** -- no custom symlink logic, no templating
2. **XDG-compliant** -- all config under `~/.config/`, `.zshenv` bootstraps `ZDOTDIR`
3. **Single bootstrap** -- `dot` handles Homebrew, packages, stow, shell in one pass
4. **Hook-enforced quality** -- TS errors block agent completion; skill activation routes on every prompt
5. **Dual AI support** -- Claude Code + OpenCode with mirrored skill/agent structures
6. **Secrets excluded** -- `mcp.json`, `.env*`, `.continuity/` all gitignored
