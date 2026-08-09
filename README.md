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

Agent skills are the exception. Claude Code reads `~/.claude/skills` and Codex reads `~/.agents/skills`, and neither reads the other's, so the shared tree at `home/.agents/skills/` is skipped by stow and published to both roots by `dot stow` itself. Skills may be grouped under a one-level category dir for clarity; that layer is flattened away when linking, since both CLIs only read `<root>/<name>/SKILL.md`.

Guard hooks are shared too, but more simply: both CLIs take an absolute path in their hook config, so the binaries are stowed once to `~/.agents/hooks/` and both point at them. `dot stow` builds them first, since the binaries are per-platform and gitignored. Claude Code registers them in `home/.claude/settings.json` and Codex in `home/.codex/hooks.json`; the two harnesses share a hook contract but not every permission decision, so each binary takes a `-harness` flag that defaults to `claude`. See `home/.agents/hooks/internal/hookio` for what differs. Codex hash-pins hook trust, so a rebuilt or edited hook needs re-approving there via `/hooks`.

## Structure

```
~/.dotfiles/
├── dot                              # CLI (v4.0.0)
├── packages/
│   └── Brewfile                     # Homebrew packages
├── home/                            # the stow package -> $HOME
│   ├── .zshenv
│   ├── .agents/skills/              # shared agent skills (published by dot, not stow)
│   ├── .agents/hooks/               # shared Go guard hooks (sources ignored, *-bin stowed)
│   ├── .claude/                     # Claude Code config
│   ├── .codex/                      # Codex config
│   └── .config/
│       ├── zsh/                     # shell (ZDOTDIR)
│       ├── starship.toml            # prompt
│       ├── git/                     # git config + aliases
│       ├── ghostty/                 # terminal
│       ├── tmux/                    # tmux + TPM (submodule)
│       ├── ripgrep/                 # rg defaults
│       ├── karabiner/               # key remapping (macOS)
│       └── opencode/                # opencode.ai config
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
dot retry-failed      Retry failed package installations
dot link              Symlink dot into ~/.local/bin
dot edit              Open dotfiles in $EDITOR
dot gen-ssh-key       Generate ed25519 SSH key
dot completions       Generate zsh completions
```

## Documentation

- [Architecture](docs/architecture.md)
- [Tmux Keybindings](docs/tmux-keybindings.md)
