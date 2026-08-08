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

Agent skills are the exception. They map to no single `$HOME` path -- Claude Code reads `~/.claude/skills` and Codex reads `~/.agents/skills` -- so `dot stow` links the shared `skills/` tree into both. Skills may be grouped under a one-level category dir for clarity; that layer is flattened away when linking, since both CLIs only read `<root>/<name>/SKILL.md`.

## Structure

```
~/.dotfiles/
├── dot                              # CLI (v4.0.0)
├── packages/
│   └── Brewfile                     # Homebrew packages
├── skills/                          # agent skills (not a stow package)
├── home/                            # the stow package -> $HOME
│   ├── .zshenv
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
