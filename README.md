# dotfiles

GNU Stow-based dotfiles for macOS and Linux. One stow package, `home/`, mirrors `$HOME`.

## Quickstart

```bash
git clone --recursive https://github.com/sustinbebustin/dotfiles.git ~/.dotfiles
~/.dotfiles/dot init
```

`dot init` installs Homebrew packages, symlinks configs, and sets zsh as the default shell. Open a new terminal when it finishes.

## How it works

Everything under `home/` maps 1:1 to `$HOME` -- `home/.config/zsh/` becomes `~/.config/zsh/`. Stow is run with `--no-folding` so each file gets its own symlink rather than the directory being linked wholesale.

Agent config is split by who reads it. `home/.agents/` is the source of truth for what Claude Code and Factory Droid share; `home/.claude/` and `home/.factory/` keep only what is specific to one harness. `dot stow` publishes the shared tree to each:

| Source | Claude Code (`~/.claude`, every `~/.claude-<account>`) | Droid |
|---|---|---|
| `.agents/AGENTS.md` | `CLAUDE.md` (`.claude/CLAUDE.md` is a symlink to it) | read natively at `~/.agents/AGENTS.md` |
| `.agents/skills/` | one folder link per skill in `skills/` | one folder link per skill in `~/.agents/skills` |
| `.agents/commands/` | dir link `commands` | dir link `~/.factory/commands` |
| `.agents/agents/` | dir link `agents` | generated droids in `~/.factory/droids` |
| `.agents/hooks/` | `*-bin` stowed into `~/.agents/hooks`, registered in `.claude/settings.base.json` | same binaries, registered in `.factory/settings.base.json` |

Skills, commands and agents are skipped by stow and linked by `dot stow` itself. Skills may be grouped under a one-level category dir for clarity; that layer is flattened away when linking, since both harnesses read `<skills dir>/<name>/SKILL.md`.

A category dir holding `.claude-plugin/plugin.json` doubles as a Claude Code plugin, listed in the root `.claude-plugin/marketplace.json`. `dot` ignores the hidden manifest dir, so local linking is unchanged. The `meta` skills install without cloning this repo:

```sh
claude plugin marketplace add sustinbebustin/.dotfiles --sparse .claude-plugin
claude plugin install meta@sustinbebustin
```

Claude Code mods in `home/.claude/mods/` are published the same way, one folder symlink per mod into `~/.claude/mods`, because the plugin loader rejects per-file links. `settings.base.json` points `CLAUDE_CODE_PLUGIN_DIRS` at that folder, so every mod there loads in every session.

The Go hooks in `home/.agents/hooks/` (`agent-guard-bin` PreToolUse guards, `agent-quality-bin` PostToolUse format/lint) are stowed as built `*-bin` binaries only -- `dot stow` builds them first, since they are per-platform and gitignored -- and registered in each harness's `settings.base.json`, which `dot stow` merges into the gitignored `settings.json` beside it. The same module builds `agent-droids-bin`, which `dot stow` runs to turn the agents into Droid droids.

## Structure

```
~/.dotfiles/
├── dot                              # CLI (v4.0.0)
├── packages/
│   └── Brewfile                     # Homebrew packages
├── home/                            # the stow package -> $HOME
│   ├── .zshenv
│   ├── .agents/                     # config shared by Claude Code and Droid
│   │   ├── AGENTS.md                # global instructions (.claude/CLAUDE.md links here)
│   │   ├── skills/                  # agent skills (published by dot, not stow)
│   │   ├── commands/                # slash commands (dir-linked by dot)
│   │   ├── agents/                  # subagents (dir-linked; droids generated)
│   │   └── hooks/                   # Go guard hooks (sources ignored, *-bin stowed)
│   ├── .claude/                     # Claude Code only: CLAUDE.md, rules, settings, mods
│   ├── .factory/                    # Droid only: settings
│   └── .config/
│       ├── zsh/                     # shell (ZDOTDIR)
│       ├── starship.toml            # prompt
│       ├── git/                     # git config + aliases
│       ├── tmux/                    # tmux + TPM (submodule)
│       ├── ripgrep/                 # rg defaults
│       └── karabiner/               # key remapping (macOS)
└── docs/
```

## Commands

```
dot init              Full bootstrap (packages, stow, shell)
dot stow              Re-stow dotfiles
dot update            Pull, update packages, re-stow
dot doctor            Health check with remediation hints
dot package add       Install and track in Brewfile
dot package remove    Remove from package list
dot package list      Show packages with install status
dot claude add        Add a Claude account (~/.claude-<name>, run as <name>)
dot claude remove     Unlink a Claude account, keeping its login
dot claude list       Show Claude accounts
dot retry-failed      Retry failed package installations
dot link              Symlink dot into ~/.local/bin
dot edit              Open dotfiles in $EDITOR
dot gen-ssh-key       Generate ed25519 SSH key
dot completions       Generate zsh completions
```

## Documentation

- [Architecture](docs/architecture.md)
- [Tmux Keybindings](docs/tmux-keybindings.md)
