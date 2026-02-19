# .zshenv - Minimal environment variables for all shells (login, non-login, interactive, non-interactive)

# Load Cargo environment if it exists
[ -f "$HOME/.cargo/env" ] && . "$HOME/.cargo/env"

# Set essential environment variables
export LANG=en_US.UTF-8
export LC_ALL=en_US.UTF-8
export EDITOR='nano'

# Ensure XDG directories are set
export XDG_CONFIG_HOME="$HOME/.config"
export XDG_DATA_HOME="$HOME/.local/share"
export XDG_CACHE_HOME="$HOME/.cache"

# Load zsh config from XDG path
export ZDOTDIR="$HOME/.config/zsh"
