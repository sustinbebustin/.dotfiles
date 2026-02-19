# dotfiles

GNU Stow-based dotfiles for macOS/Linux. Single bootstrap script, XDG-compliant config.

## Quickstart

```bash
git clone --recursive https://github.com/sustinbebustin/dotfiles.git ~/.dotfiles
~/.dotfiles/dot
```

Open a new terminal to start using the config.

## What it does

`dot` runs top-to-bottom:

1. Installs Homebrew (if missing)
2. Installs packages from `packages/Brewfile`
3. Installs global npm packages (overseer)
4. Symlinks `home/` into `$HOME` via Stow
5. Sets zsh as default shell

## Structure

```
~/.dotfiles/
├── dot                          # bootstrap entry point
├── packages/
│   └── Brewfile                 # Homebrew dependencies
├── home/                        # stow target -> $HOME
│   ├── .zshenv                  # sets ZDOTDIR
│   ├── .config/
│   │   ├── zsh/                 # shell config
│   │   ├── git/                 # git config + aliases
│   │   ├── ghostty/             # terminal emulator
│   │   ├── tmux/                # tmux + TPM
│   │   ├── ripgrep/             # rg defaults
│   │   └── opencode/            # opencode.ai config
│   └── .claude/                 # Claude Code framework
└── docs/                        # documentation
```

## Packages

| Category | Packages |
|----------|----------|
| Core | git, stow, zsh |
| Shell | starship, zsh-autosuggestions, zsh-syntax-highlighting |
| Terminal | tmux |
| CLI | ast-grep, eza, gh, jq, ripgrep |

## Documentation

- [Architecture](docs/architecture.md)
- [Tmux Keybindings](docs/tmux-keybindings.md)
