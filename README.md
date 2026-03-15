# dotfiles

GNU Stow-based dotfiles for Arch Linux and macOS. Single `dot` script handles bootstrap, package management, and symlinks. Configs are split into platform-specific stow packages so each machine only gets what it needs.

## Quickstart

```bash
git clone https://github.com/sustinbebustin/dotfiles.git ~/.dotfiles
~/.dotfiles/dot init
```

Open a new terminal to start using the config.

## What it does

`dot init` auto-detects the platform and runs:

**Arch Linux:**

1. Install packages from `packages/Pacfile` (pacman + AUR)
2. Back up conflicting files
3. Stow `home-common/` and `home-arch/` into `$HOME`
4. Set zsh as default shell

**macOS:**

1. Install Homebrew (if missing)
2. Install packages from `packages/Brewfile`
3. Install global npm packages
4. Back up conflicting files
5. Stow `home-common/` and `home-macos/` into `$HOME`
6. Set zsh as default shell

## Structure

```
~/.dotfiles/
├── dot                              # management CLI (v3.0.0)
├── packages/
│   ├── Brewfile                     # Homebrew packages (macOS)
│   └── Pacfile                      # pacman + AUR packages (Arch)
├── home-common/                     # stow package -> $HOME (all platforms)
│   ├── .zshenv
│   ├── .claude/                     # Claude Code config
│   └── .config/
│       ├── zsh/                     # shell config + completions
│       ├── starship.toml            # prompt
│       ├── git/                     # git config + aliases
│       ├── ghostty/                 # terminal emulator
│       ├── tmux/                    # tmux + TPM
│       ├── ripgrep/                 # rg defaults
│       ├── btop/                    # system monitor
│       └── opencode/                # opencode.ai config
├── home-arch/                       # stow package -> $HOME (Arch only)
│   └── .config/
│       ├── hypr/                    # Hyprland + Hyprlock + Hyprpaper
│       ├── waybar/                  # status bar + scripts
│       ├── swaync/                  # notification center
│       ├── walker/                  # app launcher
│       ├── matugen/                 # Material You color theming
│       ├── cava/                    # audio visualizer
│       ├── fastfetch/               # system info display
│       ├── kanata/                  # keyboard remapper
│       ├── xremap/                  # key remapper
│       └── systemd/                 # user services
├── home-macos/                      # stow package -> $HOME (macOS only)
│   └── .config/
│       └── karabiner/               # macOS key remapping
└── docs/
```

## Packages

### Arch Linux

| Category | Packages |
|----------|----------|
| Core | git, openssh, stow, zsh |
| Shell | fzf, lsd, starship, zsh-autosuggestions, zsh-syntax-highlighting |
| Terminal | ghostty, tmux |
| CLI | ast-grep, eza, gh, jq, ripgrep |
| Desktop | hyprland, hypridle, hyprlock, hyprpaper, swaync, waybar |
| Utilities | btop, cava, fastfetch, grim, matugen, playerctl, slurp, thunar, wl-clipboard |
| AUR | elephant, walker, wl-screenrec, yay |

### macOS

| Category | Packages |
|----------|----------|
| Core | git, stow, zsh |
| Shell | starship, zsh-autosuggestions, zsh-syntax-highlighting |
| Terminal | tmux |
| CLI | ast-grep, eza, gh, jq, ripgrep |

## Desktop (Arch)

Hyprland-based Wayland desktop with Material You theming via matugen.

| Component | Role |
|-----------|------|
| Hyprland | Tiling compositor (dwindle layout, 5120x1440 ultrawide) |
| Waybar | Status bar (workspaces, clock, system stats, media) |
| SwayNC | Notification center with power menu |
| Walker | Application launcher |
| Hyprlock | Lock screen (clock, quotes, uptime) |
| Hyprpaper | Wallpaper daemon |
| Matugen | Generates Material You colors across all components |

Matugen templates theme: ghostty, waybar, swaync, walker, hyprland, hyprlock, btop, cava, gtk, chromium, firefox.

### Key Bindings

| Bind | Action |
|------|--------|
| `SUPER+Return` | Terminal (ghostty) |
| `SUPER+Space` | App launcher (walker) |
| `SUPER+Q` | Kill window |
| `SUPER+F` | Fullscreen |
| `SUPER+V` | Toggle floating |
| `SUPER+L` | Lock screen |
| `SUPER+arrows` | Move focus |
| `SUPER+SHIFT+arrows` | Move window |
| `SUPER+CTRL+arrows` | Resize window |
| `SUPER+1-0` | Switch workspace |
| `SUPER+SHIFT+1-0` | Move to workspace |
| `SUPER+SHIFT+M` | Exit session |
| `Print` | Screenshot (region) |

## Commands

```
dot init              Full bootstrap (auto-detects platform)
dot stow              Re-stow dotfiles
dot update            Pull, update packages, re-stow
dot doctor            Health check with remediation hints
dot package add       Install and add to Brewfile/Pacfile
dot package remove    Remove from package list
dot package list      Show packages with install status
dot retry-failed      Retry failed package installations
dot link              Symlink dot into ~/.local/bin
dot edit              Open dotfiles in $EDITOR
dot gen-ssh-key       Generate ed25519 SSH key
dot completions       Install zsh completions
```

## Documentation

- [Architecture](docs/architecture.md)
- [Tmux Keybindings](docs/tmux-keybindings.md)
