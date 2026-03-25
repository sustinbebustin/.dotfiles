# dotfiles

GNU Stow-based dotfiles for Arch Linux and macOS. Configs are split into platform-specific stow packages so each machine only gets what it needs.

## Quickstart

```bash
git clone --recursive https://github.com/sustinbebustin/dotfiles.git ~/.dotfiles
~/.dotfiles/dot init
```

`dot init` auto-detects the platform, installs packages, symlinks configs, and sets zsh as the default shell. Open a new terminal when it finishes.

## How it works

Configs live in stow packages that mirror `$HOME`. `dot stow` selects packages based on the detected platform:

| Platform | Packages |
|----------|----------|
| Arch | `home-common` + `home-arch` |
| macOS | `home-common` + `home-macos` |
| Other Linux | `home-common` |

Stow creates per-file symlinks (`--no-folding`) so packages never conflict with each other.

## Structure

```
~/.dotfiles/
├── dot                              # CLI (v3.0.0)
├── packages/
│   ├── Brewfile                     # Homebrew packages (macOS)
│   └── Pacfile                      # pacman + AUR packages (Arch)
├── home-common/                     # all platforms
│   ├── .zshenv
│   ├── .claude/                     # Claude Code config
│   └── .config/
│       ├── zsh/                     # shell (ZDOTDIR)
│       ├── starship.toml            # prompt
│       ├── git/                     # git config + aliases
│       ├── ghostty/                 # terminal
│       ├── tmux/                    # tmux + TPM (submodule)
│       ├── ripgrep/                 # rg defaults
│       └── opencode/                # opencode.ai config
├── home-arch/                       # Arch only
│   └── .config/
│       ├── btop/                    # system monitor
│       ├── hypr/                    # Hyprland + Hyprlock + Hyprpaper
│       ├── waybar/                  # status bar
│       ├── swaync/                  # notification center
│       ├── walker/                  # app launcher
│       ├── matugen/                 # Material You theming
│       ├── kanata/                  # keyboard remapper
│       ├── xremap/                  # key remapper (macOS-style shortcuts)
│       ├── cava/                    # audio visualizer
│       ├── fastfetch/               # system info
│       └── systemd/                 # user services
├── home-macos/                      # macOS only
│   └── .config/
│       └── karabiner/               # key remapping
└── docs/
```

## Desktop (Arch)

Hyprland-based Wayland desktop with Material You theming via matugen.

| Component | Role |
|-----------|------|
| Hyprland | Tiling compositor (dwindle layout, 5120x1440 ultrawide) |
| Waybar | Status bar (workspaces, clock, system stats, media) |
| SwayNC | Notification center |
| Walker | Application launcher |
| Hyprlock | Lock screen |
| Hyprpaper | Wallpaper daemon |
| Matugen | Generates Material You colors from wallpaper |
| Kanata + Xremap | macOS-style key remapping |

Matugen templates theme: ghostty, waybar, swaync, walker, hyprland, hyprlock, btop, cava, gtk, chromium, firefox.

## Commands

```
dot init              Full bootstrap (auto-detects platform)
dot stow              Re-stow dotfiles
dot update            Pull, update packages, re-stow
dot doctor            Health check with remediation hints
dot package add       Install and track in Brewfile/Pacfile
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
- [Hyprland Keybindings](docs/hyprland-keybindings.md)
- [Tmux Keybindings](docs/tmux-keybindings.md)
